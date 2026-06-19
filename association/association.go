// SPDX-License-Identifier: Apache-2.0

package association

import (
	"bufio"
	"errors"
	"fmt"
	"net"
	"sync"

	"github.com/pravesh707/go-dicom/dicom"
	"github.com/pravesh707/go-dicom/dimse"
	"github.com/pravesh707/go-dicom/pdu"
)

const defaultMaxPDU = 16384

// Control-flow sentinels returned by readMessage.
var (
	errReleaseRequested = errors.New("association: A-RELEASE-RQ received")
	errReleaseConfirmed = errors.New("association: A-RELEASE-RP received")
	errAborted          = errors.New("association: A-ABORT received")
)

// Association is an established DICOM association over which DIMSE messages are
// exchanged. It is safe for a single goroutine to drive; concurrent sends are
// serialized by an internal mutex.
type Association struct {
	conn   net.Conn
	reader *bufio.Reader

	isRequestor   bool
	peerMaxLength uint32
	ourMaxLength  uint32

	acceptedByID     map[byte]AcceptedContext
	acceptedBySyntax map[string]AcceptedContext

	CallingAETitle string
	CalledAETitle  string

	// localAETitle is this AE's own title (the acceptor side), used as the
	// calling title for C-MOVE store sub-associations.
	localAETitle string
	// moveResolver maps a C-MOVE destination AE title to a "host:port" address.
	moveResolver func(aeTitle string) (string, bool)
	// moveStorageContexts are proposed to the C-MOVE destination.
	moveStorageContexts []RequestedContext

	mu        sync.Mutex
	nextMsgID uint16
	closed    bool
}

func newAssociation(conn net.Conn, isRequestor bool) *Association {
	return &Association{
		conn:             conn,
		reader:           bufio.NewReader(conn),
		isRequestor:      isRequestor,
		acceptedByID:     make(map[byte]AcceptedContext),
		acceptedBySyntax: make(map[string]AcceptedContext),
		nextMsgID:        1,
	}
}

// AcceptedContexts returns the negotiated presentation contexts.
func (a *Association) AcceptedContexts() []AcceptedContext {
	out := make([]AcceptedContext, 0, len(a.acceptedByID))
	for _, c := range a.acceptedByID {
		out = append(out, c)
	}
	return out
}

// contextForSyntax returns the accepted context for an abstract syntax UID.
func (a *Association) contextForSyntax(abstractSyntax string) (AcceptedContext, bool) {
	c, ok := a.acceptedBySyntax[abstractSyntax]
	return c, ok
}

func (a *Association) nextMessageID() uint16 {
	a.mu.Lock()
	defer a.mu.Unlock()
	id := a.nextMsgID
	a.nextMsgID++
	return id
}

// writePDU serializes and writes a PDU under the association write lock.
func (a *Association) writePDU(p pdu.PDU) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	return pdu.WritePDU(a.conn, p)
}

// Close closes the underlying connection.
func (a *Association) Close() error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.closed {
		return nil
	}
	a.closed = true
	return a.conn.Close()
}

// peerMaxData returns the maximum number of payload bytes to place in a single
// PDV, accounting for PDU and PDV framing overhead.
func (a *Association) peerMaxData() int {
	max := int(a.peerMaxLength)
	if max == 0 {
		max = defaultMaxPDU
	}
	// 6-byte PDU header + 4-byte PDV length + 1 context ID + 1 control header.
	chunk := max - 12
	if chunk < 256 {
		chunk = 256
	}
	return chunk
}

// sendMessage encodes and sends a DIMSE message (command set, then optional
// data set) as one or more P-DATA-TF PDUs on the given presentation context.
func (a *Association) sendMessage(ctx AcceptedContext, msg dimse.Message, data *dicom.DataSet) error {
	cmd, err := dimse.EncodeCommand(msg)
	if err != nil {
		return err
	}
	if err := a.sendPDVStream(ctx.ID, cmd, true); err != nil {
		return err
	}
	if msg.HasDataSet() && data != nil {
		raw, err := dicom.Encode(data, ctx.TransferSyntax)
		if err != nil {
			return err
		}
		if err := a.sendPDVStream(ctx.ID, raw, false); err != nil {
			return err
		}
	}
	return nil
}

// sendPDVStream fragments payload into PDVs (one per P-DATA-TF PDU) respecting
// the peer's maximum PDU length, marking the final fragment as last.
func (a *Association) sendPDVStream(ctxID byte, payload []byte, isCommand bool) error {
	chunk := a.peerMaxData()
	for off := 0; ; {
		end := off + chunk
		last := false
		if end >= len(payload) {
			end = len(payload)
			last = true
		}
		p := &pdu.PDataTF{PDVs: []pdu.PDV{{
			ContextID: ctxID,
			IsCommand: isCommand,
			IsLast:    last,
			Data:      payload[off:end],
		}}}
		if err := a.writePDU(p); err != nil {
			return err
		}
		off = end
		if last {
			return nil
		}
	}
}

// readMessage reads PDUs until one complete DIMSE message (command set plus any
// data set) has been assembled, returning the presentation context and decoded
// message. Control PDUs surface as the errRelease*/errAborted sentinels.
func (a *Association) readMessage() (AcceptedContext, dimse.Message, *dicom.DataSet, error) {
	var (
		cmdBuf  []byte
		dataBuf []byte
		ctxID   byte
		haveCmd bool
		cmdDone bool
		msg     dimse.Message
	)

	for {
		p, err := pdu.ReadPDU(a.reader)
		if err != nil {
			return AcceptedContext{}, nil, nil, err
		}
		switch pt := p.(type) {
		case *pdu.PDataTF:
			dataDone := false
			for _, v := range pt.PDVs {
				ctxID = v.ContextID
				if v.IsCommand {
					cmdBuf = append(cmdBuf, v.Data...)
					if v.IsLast {
						cmdDone = true
					}
				} else {
					dataBuf = append(dataBuf, v.Data...)
					if v.IsLast {
						dataDone = true
					}
				}
			}
			if cmdDone && !haveCmd {
				msg, err = dimse.DecodeCommand(cmdBuf)
				if err != nil {
					return AcceptedContext{}, nil, nil, err
				}
				haveCmd = true
				if !msg.HasDataSet() {
					ctx := a.acceptedByID[ctxID]
					return ctx, msg, nil, nil
				}
			}
			// If the command is complete and expects a data set, return once
			// the data stream has ended (signalled by the last data PDV).
			if haveCmd && msg.HasDataSet() && dataDone {
				ctx := a.acceptedByID[ctxID]
				ds, err := dicom.Decode(dataBuf, ctx.TransferSyntax)
				if err != nil {
					return ctx, msg, nil, fmt.Errorf("association: decoding data set: %w", err)
				}
				return ctx, msg, ds, nil
			}
		case *pdu.ReleaseRQ:
			return AcceptedContext{}, nil, nil, errReleaseRequested
		case *pdu.ReleaseRP:
			return AcceptedContext{}, nil, nil, errReleaseConfirmed
		case *pdu.Abort:
			return AcceptedContext{}, nil, nil, errAborted
		default:
			a.writePDU(&pdu.Abort{Source: pdu.AbortSourceServiceProvider, Reason: pdu.AbortReasonUnexpectedPDU})
			return AcceptedContext{}, nil, nil, fmt.Errorf("association: unexpected PDU %#x", p.PDUType())
		}
	}
}

// Abort sends an A-ABORT and closes the connection.
func (a *Association) Abort() error {
	_ = a.writePDU(&pdu.Abort{Source: pdu.AbortSourceServiceUser, Reason: pdu.AbortReasonNotSpecified})
	return a.Close()
}

// SPDX-License-Identifier: Apache-2.0

package association

import (
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/pravesh707/go-dicom/dicom"
	"github.com/pravesh707/go-dicom/dimse"
	"github.com/pravesh707/go-dicom/pdu"
)

// AcceptParams configures the acceptor (SCP) side of association negotiation.
type AcceptParams struct {
	AETitle                   string
	SupportedContexts         []SupportedContext
	MaximumLength             uint32
	ImplementationClassUID    string
	ImplementationVersionName string
	RequireCalledAET          bool

	// MoveResolver maps a C-MOVE destination AE title to a "host:port" address.
	// Required to serve C-MOVE.
	MoveResolver func(aeTitle string) (string, bool)
	// MoveStorageContexts are the presentation contexts proposed to the C-MOVE
	// destination when forwarding matched instances.
	MoveStorageContexts []RequestedContext
}

// Accept performs acceptor-side ACSE negotiation over conn. On success it
// returns an established Association ready for Serve. If the request is
// rejected, an A-ASSOCIATE-RJ is sent and an error returned.
func Accept(conn net.Conn, p AcceptParams) (*Association, error) {
	a := newAssociation(conn, false)
	a.ourMaxLength = p.MaximumLength
	a.localAETitle = p.AETitle
	a.moveResolver = p.MoveResolver
	a.moveStorageContexts = p.MoveStorageContexts

	req, err := pdu.ReadPDU(a.reader)
	if err != nil {
		conn.Close()
		return nil, err
	}
	rq, ok := req.(*pdu.AssociateRQ)
	if !ok {
		conn.Close()
		return nil, fmt.Errorf("association: expected A-ASSOCIATE-RQ, got PDU %#x", req.PDUType())
	}
	a.CallingAETitle = rq.CallingAETitle
	a.CalledAETitle = rq.CalledAETitle
	a.peerMaxLength = rq.UserInformation.MaximumLength

	if p.RequireCalledAET && rq.CalledAETitle != p.AETitle {
		rj := &pdu.AssociateRJ{
			Result: pdu.RJResultRejectedPermanent,
			Source: pdu.RJSourceServiceUser,
			Reason: pdu.RJReasonCalledAENotRecognized,
		}
		_ = pdu.WritePDU(conn, rj)
		conn.Close()
		return nil, fmt.Errorf("association: called AE title %q not recognized", rq.CalledAETitle)
	}

	acItems, accepted := negotiate(rq.PresentationContexts, p.SupportedContexts)
	a.acceptedByID = accepted
	for _, c := range accepted {
		a.acceptedBySyntax[c.AbstractSyntax] = c
	}

	// Confirm any proposed SCP/SCU Role Selection for accepted abstract
	// syntaxes, so the requestor may act as the SCP (e.g. receive instances
	// during a C-GET). PS3.7 §D.3.3.4.
	acceptedSyntax := make(map[string]bool, len(accepted))
	for _, c := range accepted {
		acceptedSyntax[c.AbstractSyntax] = true
	}
	var roles []pdu.RoleSelection
	for _, rs := range rq.UserInformation.RoleSelection {
		if acceptedSyntax[rs.SOPClassUID] {
			roles = append(roles, pdu.RoleSelection{SOPClassUID: rs.SOPClassUID, SCURole: rs.SCURole, SCPRole: rs.SCPRole})
		}
	}

	ac := &pdu.AssociateAC{
		CalledAETitle:        rq.CalledAETitle,
		CallingAETitle:       rq.CallingAETitle,
		ApplicationContext:   dicom.DICOMApplicationContextName,
		PresentationContexts: acItems,
		UserInformation: pdu.UserInformation{
			MaximumLength:             p.MaximumLength,
			ImplementationClassUID:    p.ImplementationClassUID,
			ImplementationVersionName: p.ImplementationVersionName,
			RoleSelection:             roles,
		},
	}
	if err := pdu.WritePDU(conn, ac); err != nil {
		conn.Close()
		return nil, err
	}
	return a, nil
}

// Serve runs the acceptor's DIMSE message loop until the association is
// released, aborted or the connection closes, dispatching requests to the bound
// handlers.
func (a *Association) Serve(bindings []HandlerBinding) error {
	ht := newHandlerTable(bindings)
	ht.emit(&Event{Type: EvtEstablished, Assoc: a})
	defer a.Close()

	for {
		ctx, msg, data, err := a.readMessage()
		if err != nil {
			switch {
			case errors.Is(err, errReleaseRequested):
				_ = a.writePDU(&pdu.ReleaseRP{})
				ht.emit(&Event{Type: EvtReleased, Assoc: a})
				return nil
			case errors.Is(err, errAborted):
				ht.emit(&Event{Type: EvtAborted, Assoc: a})
				return nil
			case errors.Is(err, io.EOF):
				return nil
			default:
				return err
			}
		}
		if err := a.dispatch(ht, ctx, msg, data); err != nil {
			return err
		}
	}
}

// dispatch routes one inbound DIMSE request to its handler and sends the
// response(s).
func (a *Association) dispatch(ht *handlerTable, ctx AcceptedContext, msg dimse.Message, data *dicom.DataSet) error {
	ht.emit(&Event{Type: EvtDIMSERecv, Assoc: a, Request: msg, DataSet: data})

	switch req := msg.(type) {
	case *dimse.CEchoRequest:
		ev := &Event{Type: EvtCEcho, Assoc: a, Context: ctx, Request: req}
		status, _ := ht.handle(ev) // default is Success for verification
		return a.sendMessage(ctx, &dimse.CEchoResponse{
			MessageIDBeingRespondedTo: req.MessageID,
			AffectedSOPClassUID:       req.AffectedSOPClassUID,
			Status:                    status,
		}, nil)

	case *dimse.CStoreRequest:
		ev := &Event{Type: EvtCStore, Assoc: a, Context: ctx, Request: req, DataSet: data}
		status, handled := ht.handle(ev)
		if !handled {
			// No store handler bound — the peer's instance cannot be processed.
			status = dimse.StatusRefusedSOPClassNotSupported
		}
		return a.sendMessage(ctx, &dimse.CStoreResponse{
			MessageIDBeingRespondedTo: req.MessageID,
			AffectedSOPClassUID:       req.AffectedSOPClassUID,
			AffectedSOPInstanceUID:    req.AffectedSOPInstanceUID,
			Status:                    status,
		}, nil)

	case *dimse.CFindRequest:
		ev := &Event{Type: EvtCFind, Assoc: a, Context: ctx, Request: req, DataSet: data}
		ev.yield = func(identifier *dicom.DataSet) error {
			return a.sendMessage(ctx, &dimse.CFindResponse{
				MessageIDBeingRespondedTo: req.MessageID,
				AffectedSOPClassUID:       req.AffectedSOPClassUID,
				Status:                    dimse.StatusPending,
				Identifier:                identifier,
			}, identifier)
		}
		status, _ := ht.handle(ev) // default Success => empty result set
		return a.sendMessage(ctx, &dimse.CFindResponse{
			MessageIDBeingRespondedTo: req.MessageID,
			AffectedSOPClassUID:       req.AffectedSOPClassUID,
			Status:                    status,
		}, nil)

	case *dimse.CGetRequest:
		return a.dispatchCGet(ht, ctx, req, data)

	case *dimse.CMoveRequest:
		return a.dispatchCMove(ht, ctx, req, data)

	case *dimse.NRequest:
		return a.dispatchN(ht, ctx, req, data)

	default:
		// No typed handler for this DIMSE service. Abort cleanly rather than
		// leaving the peer waiting.
		_ = a.writePDU(&pdu.Abort{
			Source: pdu.AbortSourceServiceProvider,
			Reason: pdu.AbortReasonUnexpectedPDU,
		})
		return fmt.Errorf("association: unhandled DIMSE service %s", msg.CommandField())
	}
}

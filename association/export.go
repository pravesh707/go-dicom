// SPDX-License-Identifier: Apache-2.0

package association

import (
	"net"

	"github.com/pravesh707/go-dicom/dicom"
	"github.com/pravesh707/go-dicom/dimse"
	"github.com/pravesh707/go-dicom/pdu"
)

// This file exposes a small set of package internals to the black-box tests in
// ./tests. They are low-level seams for white-box-style testing, not part of
// the ergonomic API — application code should prefer the high-level godicom
// package and the Association service methods (SendCEcho, Serve, …).

// DefaultMaxPDU is the maximum PDU length assumed when a peer advertises no
// limit (a Maximum Length of 0).
const DefaultMaxPDU = defaultMaxPDU

// Control-PDU sentinels returned by ReadMessage when the peer sends an
// A-RELEASE-RQ, A-RELEASE-RP or A-ABORT instead of a DIMSE message.
var (
	ErrReleaseRequested = errReleaseRequested
	ErrReleaseConfirmed = errReleaseConfirmed
	ErrAborted          = errAborted
)

// FirstCommonTransferSyntax returns the first element of prefer that also
// appears in avail, preserving the requestor's preference order.
func FirstCommonTransferSyntax(prefer, avail []string) string { return firstCommon(prefer, avail) }

// Negotiate runs acceptor-side presentation-context negotiation, returning the
// accept-side items and the accepted contexts keyed by presentation-context ID.
func Negotiate(requested []pdu.PresentationContextRQ, supported []SupportedContext) ([]pdu.PresentationContextAC, map[byte]AcceptedContext) {
	return negotiate(requested, supported)
}

// IsIntervention reports whether the event is an intervention event — one whose
// handler returns a DIMSE status that is sent to the peer (as opposed to a
// fire-and-forget notification event).
func (e EventType) IsIntervention() bool { return e.isIntervention() }

// HandlerTable is the event-dispatch table built from a set of HandlerBinding.
type HandlerTable = handlerTable

// NewHandlerTable builds a dispatch table from the given bindings.
func NewHandlerTable(bindings []HandlerBinding) *HandlerTable { return newHandlerTable(bindings) }

// Handle dispatches an intervention event to its handler, returning the status
// and whether a handler was bound.
func (t *handlerTable) Handle(ev *Event) (dimse.Status, bool) { return t.handle(ev) }

// Emit fans a notification event out to every bound handler.
func (t *handlerTable) Emit(ev *Event) { t.emit(ev) }

// NewAssociation builds an unestablished association over conn (requestor=true
// for the SCU side). For tests that drive the framing/DIMSE layers directly.
func NewAssociation(conn net.Conn, requestor bool) *Association {
	return newAssociation(conn, requestor)
}

// SetPeerMaxLength overrides the peer's advertised maximum PDU length.
func (a *Association) SetPeerMaxLength(n uint32) { a.peerMaxLength = n }

// PeerMaxData returns the usable PDV payload size (peer max PDU minus the
// P-DATA-TF framing overhead), with a small floor applied.
func (a *Association) PeerMaxData() int { return a.peerMaxData() }

// SetAcceptedContext records an accepted presentation context by ID, as
// negotiation would.
func (a *Association) SetAcceptedContext(id byte, ctx AcceptedContext) { a.acceptedByID[id] = ctx }

// ContextForSyntax returns the accepted presentation context for an abstract
// syntax, if one was negotiated.
func (a *Association) ContextForSyntax(abstractSyntax string) (AcceptedContext, bool) {
	return a.contextForSyntax(abstractSyntax)
}

// SendPDVStream fragments payload into one or more P-DATA-TF PDVs on the given
// presentation-context ID.
func (a *Association) SendPDVStream(ctxID byte, payload []byte, isCommand bool) error {
	return a.sendPDVStream(ctxID, payload, isCommand)
}

// SendMessage encodes and sends a DIMSE message and its optional data set on
// the given context.
func (a *Association) SendMessage(ctx AcceptedContext, msg dimse.Message, data *dicom.DataSet) error {
	return a.sendMessage(ctx, msg, data)
}

// ReadMessage reads the next DIMSE message (reassembling PDVs), surfacing
// control PDUs as the ErrRelease*/ErrAborted sentinels.
func (a *Association) ReadMessage() (AcceptedContext, dimse.Message, *dicom.DataSet, error) {
	return a.readMessage()
}

// ReadPDU reads one raw PDU from the association's connection — used by tests to
// drain a synchronous pipe so a peer's write does not block.
func (a *Association) ReadPDU() (pdu.PDU, error) { return pdu.ReadPDU(a.reader) }

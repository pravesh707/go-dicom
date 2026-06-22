// SPDX-License-Identifier: Apache-2.0

package association

import (
	"fmt"
	"net"

	"github.com/pravesh707/go-dicom/dicom"
	"github.com/pravesh707/go-dicom/dimse"
	"github.com/pravesh707/go-dicom/pdu"
)

// RequestParams configures the requestor (SCU) side of association negotiation.
type RequestParams struct {
	CallingAETitle            string
	CalledAETitle             string
	RequestedContexts         []RequestedContext
	MaximumLength             uint32
	ImplementationClassUID    string
	ImplementationVersionName string
}

// RejectedError reports an A-ASSOCIATE-RJ response.
type RejectedError struct {
	Result byte
	Source byte
	Reason byte
}

func (e *RejectedError) Error() string {
	return fmt.Sprintf("association rejected (result=%#x source=%#x reason=%#x)", e.Result, e.Source, e.Reason)
}

// Request performs requestor-side ACSE negotiation over conn, returning an
// established Association on success.
func Request(conn net.Conn, p RequestParams) (*Association, error) {
	a := newAssociation(conn, true)
	a.ourMaxLength = p.MaximumLength
	a.CallingAETitle = p.CallingAETitle
	a.CalledAETitle = p.CalledAETitle

	var pcs []pdu.PresentationContextRQ
	var roles []pdu.RoleSelection
	ctxByID := make(map[byte]RequestedContext)
	id := byte(1) // presentation context IDs are odd
	for _, rc := range p.RequestedContexts {
		pcs = append(pcs, pdu.PresentationContextRQ{
			ID:               id,
			AbstractSyntax:   rc.AbstractSyntax,
			TransferSyntaxes: rc.TransferSyntaxes,
		})
		if rc.ScuRole || rc.ScpRole {
			roles = append(roles, pdu.RoleSelection{
				SOPClassUID: rc.AbstractSyntax,
				SCURole:     rc.ScuRole,
				SCPRole:     rc.ScpRole,
			})
		}
		ctxByID[id] = rc
		id += 2
	}

	rq := &pdu.AssociateRQ{
		CalledAETitle:        p.CalledAETitle,
		CallingAETitle:       p.CallingAETitle,
		ApplicationContext:   dicom.DICOMApplicationContextName,
		PresentationContexts: pcs,
		UserInformation: pdu.UserInformation{
			MaximumLength:             p.MaximumLength,
			ImplementationClassUID:    p.ImplementationClassUID,
			ImplementationVersionName: p.ImplementationVersionName,
			RoleSelection:             roles,
		},
	}
	if err := pdu.WritePDU(conn, rq); err != nil {
		conn.Close()
		return nil, err
	}

	resp, err := pdu.ReadPDU(a.reader)
	if err != nil {
		conn.Close()
		return nil, err
	}
	switch r := resp.(type) {
	case *pdu.AssociateAC:
		a.peerMaxLength = r.UserInformation.MaximumLength
		for _, ac := range r.PresentationContexts {
			if ac.Result != pdu.PCAccepted {
				continue
			}
			rc := ctxByID[ac.ID]
			c := AcceptedContext{ID: ac.ID, AbstractSyntax: rc.AbstractSyntax, TransferSyntax: ac.TransferSyntax}
			a.acceptedByID[ac.ID] = c
			a.acceptedBySyntax[rc.AbstractSyntax] = c
		}
		return a, nil
	case *pdu.AssociateRJ:
		conn.Close()
		return nil, &RejectedError{Result: r.Result, Source: r.Source, Reason: r.Reason}
	case *pdu.Abort:
		conn.Close()
		return nil, errAborted
	default:
		conn.Close()
		return nil, fmt.Errorf("association: unexpected response PDU %#x", resp.PDUType())
	}
}

// SendCEcho issues a C-ECHO (verification) request on the Verification SOP
// Class context and returns the response status.
func (a *Association) SendCEcho() (dimse.Status, error) {
	return a.SendCEchoOn(dicom.VerificationSOPClass)
}

// SendCEchoOn issues a C-ECHO on the accepted context for the given abstract
// syntax (normally the Verification SOP Class).
func (a *Association) SendCEchoOn(abstractSyntax string) (dimse.Status, error) {
	ctx, ok := a.contextForSyntax(abstractSyntax)
	if !ok {
		return 0, fmt.Errorf("association: no accepted presentation context for %s", abstractSyntax)
	}
	rq := &dimse.CEchoRequest{MessageID: a.nextMessageID(), AffectedSOPClassUID: abstractSyntax}
	if err := a.sendMessage(ctx, rq, nil); err != nil {
		return 0, err
	}
	_, msg, _, err := a.readMessage()
	if err != nil {
		return 0, err
	}
	rsp, ok := msg.(*dimse.CEchoResponse)
	if !ok {
		return 0, fmt.Errorf("association: expected C-ECHO-RSP, got %s", msg.CommandField())
	}
	return rsp.Status, nil
}

// Release performs a graceful A-RELEASE handshake and closes the connection.
func (a *Association) Release() error {
	if err := a.writePDU(&pdu.ReleaseRQ{}); err != nil {
		a.Close()
		return err
	}
	resp, err := pdu.ReadPDU(a.reader)
	if err != nil {
		a.Close()
		return err
	}
	switch resp.(type) {
	case *pdu.ReleaseRP:
		return a.Close()
	case *pdu.Abort:
		a.Close()
		return errAborted
	default:
		return a.Close()
	}
}

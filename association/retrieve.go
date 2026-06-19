// SPDX-License-Identifier: Apache-2.0

package association

import (
	"fmt"
	"net"

	"github.com/pravesh707/go-dicom/dicom"
	"github.com/pravesh707/go-dicom/dimse"
)

// RetrieveResult summarizes a completed C-MOVE or C-GET operation.
type RetrieveResult struct {
	Status    dimse.Status
	Completed uint16
	Failed    uint16
	Warning   uint16
}

// StoreCallback handles an instance received during a C-GET sub-operation and
// returns the C-STORE status to report.
type StoreCallback func(*dicom.DataSet) dimse.Status

// statusMoveDestinationUnknown is the C-MOVE failure for an unresolvable
// destination AE (PS3.4 C.4.2.1.5).
const statusMoveDestinationUnknown = dimse.Status(0xA801)

// SendCMove issues a C-MOVE: the peer forwards matched instances to the named
// destination AE via C-STORE sub-operations. It returns the final result with
// sub-operation counts.
func (a *Association) SendCMove(sopClass, destination string, query *dicom.DataSet) (RetrieveResult, error) {
	ctx, ok := a.contextForSyntax(sopClass)
	if !ok {
		return RetrieveResult{}, fmt.Errorf("association: no accepted presentation context for %s", sopClass)
	}
	rq := &dimse.CMoveRequest{
		MessageID:           a.nextMessageID(),
		AffectedSOPClassUID: sopClass,
		Priority:            dimse.PriorityMedium,
		MoveDestination:     destination,
		Identifier:          query,
	}
	if err := a.sendMessage(ctx, rq, query); err != nil {
		return RetrieveResult{}, err
	}
	for {
		_, msg, _, err := a.readMessage()
		if err != nil {
			return RetrieveResult{}, err
		}
		rsp, ok := msg.(*dimse.RetrieveResponse)
		if !ok {
			return RetrieveResult{}, fmt.Errorf("association: expected C-MOVE-RSP, got %s", msg.CommandField())
		}
		if rsp.Status.IsPending() {
			continue
		}
		return result(rsp), nil
	}
}

// SendCGet issues a C-GET: the peer returns matched instances over this same
// association as C-STORE sub-operations, each delivered to store.
func (a *Association) SendCGet(sopClass string, query *dicom.DataSet, store StoreCallback) (RetrieveResult, error) {
	ctx, ok := a.contextForSyntax(sopClass)
	if !ok {
		return RetrieveResult{}, fmt.Errorf("association: no accepted presentation context for %s", sopClass)
	}
	rq := &dimse.CGetRequest{
		MessageID:           a.nextMessageID(),
		AffectedSOPClassUID: sopClass,
		Priority:            dimse.PriorityMedium,
		Identifier:          query,
	}
	if err := a.sendMessage(ctx, rq, query); err != nil {
		return RetrieveResult{}, err
	}
	for {
		rctx, msg, ds, err := a.readMessage()
		if err != nil {
			return RetrieveResult{}, err
		}
		switch m := msg.(type) {
		case *dimse.CStoreRequest:
			status := dimse.StatusSuccess
			if store != nil {
				status = store(ds)
			} else {
				status = dimse.StatusRefusedOutOfResources
			}
			if err := a.sendMessage(rctx, &dimse.CStoreResponse{
				MessageIDBeingRespondedTo: m.MessageID,
				AffectedSOPClassUID:       m.AffectedSOPClassUID,
				AffectedSOPInstanceUID:    m.AffectedSOPInstanceUID,
				Status:                    status,
			}, nil); err != nil {
				return RetrieveResult{}, err
			}
		case *dimse.RetrieveResponse:
			if m.Status.IsPending() {
				continue
			}
			return result(m), nil
		default:
			return RetrieveResult{}, fmt.Errorf("association: unexpected %s during C-GET", msg.CommandField())
		}
	}
}

func result(rsp *dimse.RetrieveResponse) RetrieveResult {
	return RetrieveResult{
		Status:    rsp.Status,
		Completed: rsp.SubOps.Completed,
		Failed:    rsp.SubOps.Failed,
		Warning:   rsp.SubOps.Warning,
	}
}

// dispatchCGet serves a C-GET request: the bound handler yields instances, each
// sent back to the requestor as a C-STORE sub-operation on this association.
func (a *Association) dispatchCGet(ht *handlerTable, ctx AcceptedContext, req *dimse.CGetRequest, data *dicom.DataSet) error {
	var ops dimse.SubOperations
	ev := &Event{Type: EvtCGet, Assoc: a, Context: ctx, Request: req, DataSet: data}
	ev.yield = func(instance *dicom.DataSet) error {
		sopClass, _ := instance.GetString(tagSOPClassUID)
		sopInst, _ := instance.GetString(tagSOPInstanceUID)
		storeCtx, ok := a.contextForSyntax(sopClass)
		if !ok {
			ops.Failed++
			return a.sendRetrievePending(ctx, dimse.CGetRSP, req.MessageID, req.AffectedSOPClassUID, ops)
		}
		storeRQ := &dimse.CStoreRequest{
			MessageID:              a.nextMessageID(),
			AffectedSOPClassUID:    sopClass,
			AffectedSOPInstanceUID: sopInst,
			Priority:               dimse.PriorityMedium,
		}
		if err := a.sendMessage(storeCtx, storeRQ, instance); err != nil {
			return err
		}
		_, msg, _, err := a.readMessage()
		if err != nil {
			return err
		}
		if rsp, ok := msg.(*dimse.CStoreResponse); ok && rsp.Status.IsSuccess() {
			ops.Completed++
		} else {
			ops.Failed++
		}
		return a.sendRetrievePending(ctx, dimse.CGetRSP, req.MessageID, req.AffectedSOPClassUID, ops)
	}
	status, _ := ht.handle(ev)
	return a.sendRetrieveFinal(ctx, dimse.CGetRSP, req.MessageID, req.AffectedSOPClassUID, status, ops)
}

// dispatchCMove serves a C-MOVE request: matched instances are forwarded to the
// resolved destination AE over a fresh store sub-association.
func (a *Association) dispatchCMove(ht *handlerTable, ctx AcceptedContext, req *dimse.CMoveRequest, data *dicom.DataSet) error {
	addr := ""
	if a.moveResolver != nil {
		if resolved, ok := a.moveResolver(req.MoveDestination); ok {
			addr = resolved
		}
	}
	if addr == "" {
		return a.sendRetrieveFinal(ctx, dimse.CMoveRSP, req.MessageID, req.AffectedSOPClassUID, statusMoveDestinationUnknown, dimse.SubOperations{})
	}

	var dest *Association
	var ops dimse.SubOperations
	ev := &Event{Type: EvtCMove, Assoc: a, Context: ctx, Request: req, DataSet: data}
	ev.yield = func(instance *dicom.DataSet) error {
		if dest == nil {
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				ops.Failed++
				return a.sendRetrievePending(ctx, dimse.CMoveRSP, req.MessageID, req.AffectedSOPClassUID, ops)
			}
			dest, err = Request(conn, RequestParams{
				CallingAETitle:            a.localAETitle,
				CalledAETitle:             req.MoveDestination,
				RequestedContexts:         a.moveStorageContexts,
				MaximumLength:             a.ourMaxLength,
				ImplementationClassUID:    dicom.GoDICOMImplementationClassUID,
				ImplementationVersionName: dicom.GoDICOMImplementationVersionName,
			})
			if err != nil {
				dest = nil
				ops.Failed++
				return a.sendRetrievePending(ctx, dimse.CMoveRSP, req.MessageID, req.AffectedSOPClassUID, ops)
			}
		}
		status, err := dest.SendCStore(instance)
		if err != nil || !status.IsSuccess() {
			ops.Failed++
		} else {
			ops.Completed++
		}
		return a.sendRetrievePending(ctx, dimse.CMoveRSP, req.MessageID, req.AffectedSOPClassUID, ops)
	}
	status, _ := ht.handle(ev)
	if dest != nil {
		dest.Release()
	}
	return a.sendRetrieveFinal(ctx, dimse.CMoveRSP, req.MessageID, req.AffectedSOPClassUID, status, ops)
}

func (a *Association) sendRetrievePending(ctx AcceptedContext, cf dimse.CommandField, msgID uint16, sopClass string, ops dimse.SubOperations) error {
	return a.sendMessage(ctx, &dimse.RetrieveResponse{
		Command:                   cf,
		MessageIDBeingRespondedTo: msgID,
		AffectedSOPClassUID:       sopClass,
		Status:                    dimse.StatusPending,
		SubOps:                    ops,
	}, nil)
}

func (a *Association) sendRetrieveFinal(ctx AcceptedContext, cf dimse.CommandField, msgID uint16, sopClass string, status dimse.Status, ops dimse.SubOperations) error {
	return a.sendMessage(ctx, &dimse.RetrieveResponse{
		Command:                   cf,
		MessageIDBeingRespondedTo: msgID,
		AffectedSOPClassUID:       sopClass,
		Status:                    status,
		SubOps:                    ops,
	}, nil)
}

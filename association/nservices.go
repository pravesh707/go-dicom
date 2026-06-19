// SPDX-License-Identifier: Apache-2.0

package association

import (
	"fmt"

	"github.com/pravesh707/go-dicom/dicom"
	"github.com/pravesh707/go-dicom/dimse"
)

// SendN issues a DIMSE-N request and returns the response. The message ID is
// assigned automatically. The SOP class must have an accepted presentation
// context.
func (a *Association) SendN(req *dimse.NRequest) (*dimse.NResponse, error) {
	ctx, ok := a.contextForSyntax(req.SOPClassUID)
	if !ok {
		return nil, fmt.Errorf("association: no accepted presentation context for %s", req.SOPClassUID)
	}
	req.MessageID = a.nextMessageID()
	if err := a.sendMessage(ctx, req, req.DataSet); err != nil {
		return nil, err
	}
	_, msg, ds, err := a.readMessage()
	if err != nil {
		return nil, err
	}
	rsp, ok := msg.(*dimse.NResponse)
	if !ok {
		return nil, fmt.Errorf("association: expected an N-service response, got %s", msg.CommandField())
	}
	rsp.DataSet = ds
	return rsp, nil
}

// SendNGet requests attributes of an SOP instance.
func (a *Association) SendNGet(sopClass, sopInstance string) (*dimse.NResponse, error) {
	return a.SendN(&dimse.NRequest{Command: dimse.NGetRQ, SOPClassUID: sopClass, SOPInstanceUID: sopInstance})
}

// SendNSet modifies attributes of an SOP instance.
func (a *Association) SendNSet(sopClass, sopInstance string, modifications *dicom.DataSet) (*dimse.NResponse, error) {
	return a.SendN(&dimse.NRequest{Command: dimse.NSetRQ, SOPClassUID: sopClass, SOPInstanceUID: sopInstance, DataSet: modifications})
}

// SendNAction invokes an action on an SOP instance.
func (a *Association) SendNAction(sopClass, sopInstance string, actionType uint16, info *dicom.DataSet) (*dimse.NResponse, error) {
	return a.SendN(&dimse.NRequest{Command: dimse.NActionRQ, SOPClassUID: sopClass, SOPInstanceUID: sopInstance, ActionTypeID: actionType, DataSet: info})
}

// SendNCreate creates a new managed SOP instance.
func (a *Association) SendNCreate(sopClass, sopInstance string, attributes *dicom.DataSet) (*dimse.NResponse, error) {
	return a.SendN(&dimse.NRequest{Command: dimse.NCreateRQ, SOPClassUID: sopClass, SOPInstanceUID: sopInstance, DataSet: attributes})
}

// SendNDelete deletes a managed SOP instance.
func (a *Association) SendNDelete(sopClass, sopInstance string) (*dimse.NResponse, error) {
	return a.SendN(&dimse.NRequest{Command: dimse.NDeleteRQ, SOPClassUID: sopClass, SOPInstanceUID: sopInstance})
}

// SendNEventReport reports an event for an SOP instance.
func (a *Association) SendNEventReport(sopClass, sopInstance string, eventType uint16, info *dicom.DataSet) (*dimse.NResponse, error) {
	return a.SendN(&dimse.NRequest{Command: dimse.NEventReportRQ, SOPClassUID: sopClass, SOPInstanceUID: sopInstance, EventTypeID: eventType, DataSet: info})
}

// dispatchN serves any DIMSE-N request: it invokes the bound handler (which may
// attach a response data set via Event.SetResponse) and returns the response.
func (a *Association) dispatchN(ht *handlerTable, ctx AcceptedContext, req *dimse.NRequest, data *dicom.DataSet) error {
	ev := &Event{Type: nEventType(req.Command), Assoc: a, Context: ctx, Request: req, DataSet: data}
	status, handled := ht.handle(ev)
	if !handled {
		status = dimse.StatusUnrecognizedOperation
	}
	rsp := &dimse.NResponse{
		Command:                   dimse.CommandField(uint16(req.Command) | 0x8000),
		MessageIDBeingRespondedTo: req.MessageID,
		SOPClassUID:               req.SOPClassUID,
		SOPInstanceUID:            req.SOPInstanceUID,
		ActionTypeID:              req.ActionTypeID,
		EventTypeID:               req.EventTypeID,
		Status:                    status,
		DataSet:                   ev.response,
	}
	return a.sendMessage(ctx, rsp, ev.response)
}

func nEventType(cf dimse.CommandField) EventType {
	switch cf {
	case dimse.NGetRQ:
		return EvtNGet
	case dimse.NSetRQ:
		return EvtNSet
	case dimse.NActionRQ:
		return EvtNAction
	case dimse.NCreateRQ:
		return EvtNCreate
	case dimse.NDeleteRQ:
		return EvtNDelete
	default:
		return EvtNEventReport
	}
}

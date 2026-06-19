// SPDX-License-Identifier: Apache-2.0

package dimse

import "github.com/pravesh707/go-dicom/dicom"

func init() {
	for _, cf := range []CommandField{NGetRQ, NSetRQ, NActionRQ, NCreateRQ, NDeleteRQ, NEventReportRQ} {
		cf := cf
		RegisterMessage(cf, func(ds *dicom.DataSet) Message { return parseNRequest(ds, cf) })
	}
	for _, cf := range []CommandField{NGetRSP, NSetRSP, NActionRSP, NCreateRSP, NDeleteRSP, NEventReportRSP} {
		cf := cf
		RegisterMessage(cf, func(ds *dicom.DataSet) Message { return parseNResponse(ds, cf) })
	}
}

// affectedClassCommand reports whether a command uses the Affected (rather than
// Requested) SOP Class/Instance UID tags. N-CREATE and N-EVENT-REPORT operate
// on an SOP instance the SCU "affects"; the others reference a "requested" one.
func affectedClassCommand(cf CommandField) bool {
	return cf == NCreateRQ || cf == NEventReportRQ
}

// NRequest is a generic DIMSE-N request primitive (N-GET/SET/ACTION/CREATE/
// DELETE/EVENT-REPORT), distinguished by Command.
type NRequest struct {
	Command        CommandField
	MessageID      uint16
	SOPClassUID    string
	SOPInstanceUID string
	ActionTypeID   uint16 // N-ACTION
	EventTypeID    uint16 // N-EVENT-REPORT
	DataSet        *dicom.DataSet
	hasData        bool
}

func (m *NRequest) CommandField() CommandField { return m.Command }
func (m *NRequest) GetMessageID() uint16       { return m.MessageID }
func (m *NRequest) HasDataSet() bool           { return m.DataSet != nil || m.hasData }

func (m *NRequest) CommandSet() *dicom.DataSet {
	classTag, instTag := dicom.TagRequestedSOPClassUID, dicom.TagRequestedSOPInstanceUID
	if affectedClassCommand(m.Command) {
		classTag, instTag = dicom.TagAffectedSOPClassUID, dicom.TagAffectedSOPInstanceUID
	}
	dst := dataSetTypeAbsent
	if m.DataSet != nil {
		dst = dataSetTypePresent
	}
	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUI(classTag, m.SOPClassUID))
	ds.Set(dicom.NewUS(dicom.TagCommandField, uint16(m.Command)))
	ds.Set(dicom.NewUS(dicom.TagMessageID, m.MessageID))
	ds.Set(dicom.NewUS(dicom.TagCommandDataSetType, uint16(dst)))
	if m.SOPInstanceUID != "" {
		ds.Set(dicom.NewUI(instTag, m.SOPInstanceUID))
	}
	switch m.Command {
	case NActionRQ:
		ds.Set(dicom.NewUS(dicom.TagActionTypeID, m.ActionTypeID))
	case NEventReportRQ:
		ds.Set(dicom.NewUS(dicom.TagEventTypeID, m.EventTypeID))
	}
	return ds
}

func parseNRequest(ds *dicom.DataSet, cf CommandField) *NRequest {
	m := &NRequest{Command: cf, hasData: hasDataSet(ds)}
	m.MessageID, _ = ds.GetUint16(dicom.TagMessageID)
	m.SOPClassUID = firstUID(ds, dicom.TagRequestedSOPClassUID, dicom.TagAffectedSOPClassUID)
	m.SOPInstanceUID = firstUID(ds, dicom.TagRequestedSOPInstanceUID, dicom.TagAffectedSOPInstanceUID)
	m.ActionTypeID, _ = ds.GetUint16(dicom.TagActionTypeID)
	m.EventTypeID, _ = ds.GetUint16(dicom.TagEventTypeID)
	return m
}

// NResponse is a generic DIMSE-N response primitive.
type NResponse struct {
	Command                   CommandField
	MessageIDBeingRespondedTo uint16
	SOPClassUID               string
	SOPInstanceUID            string
	ActionTypeID              uint16
	EventTypeID               uint16
	Status                    Status
	DataSet                   *dicom.DataSet
	hasData                   bool
}

func (m *NResponse) CommandField() CommandField { return m.Command }
func (m *NResponse) HasDataSet() bool           { return m.DataSet != nil || m.hasData }

func (m *NResponse) CommandSet() *dicom.DataSet {
	dst := dataSetTypeAbsent
	if m.DataSet != nil {
		dst = dataSetTypePresent
	}
	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUI(dicom.TagAffectedSOPClassUID, m.SOPClassUID))
	ds.Set(dicom.NewUS(dicom.TagCommandField, uint16(m.Command)))
	ds.Set(dicom.NewUS(dicom.TagMessageIDBeingRespondedTo, m.MessageIDBeingRespondedTo))
	ds.Set(dicom.NewUS(dicom.TagCommandDataSetType, uint16(dst)))
	ds.Set(dicom.NewUS(dicom.TagStatus, uint16(m.Status)))
	if m.SOPInstanceUID != "" {
		ds.Set(dicom.NewUI(dicom.TagAffectedSOPInstanceUID, m.SOPInstanceUID))
	}
	switch m.Command {
	case NActionRSP:
		ds.Set(dicom.NewUS(dicom.TagActionTypeID, m.ActionTypeID))
	case NEventReportRSP:
		ds.Set(dicom.NewUS(dicom.TagEventTypeID, m.EventTypeID))
	}
	return ds
}

func parseNResponse(ds *dicom.DataSet, cf CommandField) *NResponse {
	m := &NResponse{Command: cf, hasData: hasDataSet(ds)}
	m.MessageIDBeingRespondedTo, _ = ds.GetUint16(dicom.TagMessageIDBeingRespondedTo)
	m.SOPClassUID = firstUID(ds, dicom.TagAffectedSOPClassUID, dicom.TagRequestedSOPClassUID)
	m.SOPInstanceUID = firstUID(ds, dicom.TagAffectedSOPInstanceUID, dicom.TagRequestedSOPInstanceUID)
	m.ActionTypeID, _ = ds.GetUint16(dicom.TagActionTypeID)
	m.EventTypeID, _ = ds.GetUint16(dicom.TagEventTypeID)
	st, _ := ds.GetUint16(dicom.TagStatus)
	m.Status = Status(st)
	return m
}

func firstUID(ds *dicom.DataSet, tags ...dicom.Tag) string {
	for _, t := range tags {
		if v, ok := ds.GetString(t); ok && v != "" {
			return v
		}
	}
	return ""
}

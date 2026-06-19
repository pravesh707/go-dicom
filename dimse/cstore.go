// SPDX-License-Identifier: Apache-2.0

package dimse

import "github.com/pravesh707/go-dicom/dicom"

func init() {
	RegisterMessage(CStoreRQ, func(ds *dicom.DataSet) Message { return parseCStoreRequest(ds) })
	RegisterMessage(CStoreRSP, func(ds *dicom.DataSet) Message { return parseCStoreResponse(ds) })
}

// CStoreRequest is a C-STORE request primitive (PS3.7 §9.1.1). The instance to
// be stored travels as the associated data set; DataSet holds it on the SCU
// side so SendCStore can transmit it.
type CStoreRequest struct {
	MessageID               uint16
	AffectedSOPClassUID     string
	AffectedSOPInstanceUID  string
	Priority                Priority
	MoveOriginatorAETitle   string
	MoveOriginatorMessageID uint16
	DataSet                 *dicom.DataSet
}

func (m *CStoreRequest) CommandField() CommandField { return CStoreRQ }
func (m *CStoreRequest) HasDataSet() bool           { return true }
func (m *CStoreRequest) GetMessageID() uint16       { return m.MessageID }

func (m *CStoreRequest) CommandSet() *dicom.DataSet {
	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUI(dicom.TagAffectedSOPClassUID, m.AffectedSOPClassUID))
	ds.Set(dicom.NewUS(dicom.TagCommandField, uint16(CStoreRQ)))
	ds.Set(dicom.NewUS(dicom.TagMessageID, m.MessageID))
	ds.Set(dicom.NewUS(dicom.TagPriority, uint16(m.Priority)))
	ds.Set(dicom.NewUS(dicom.TagCommandDataSetType, dataSetTypePresent))
	ds.Set(dicom.NewUI(dicom.TagAffectedSOPInstanceUID, m.AffectedSOPInstanceUID))
	if m.MoveOriginatorAETitle != "" {
		ds.Set(dicom.NewString(dicom.TagMoveOriginatorAETitle, dicom.VRAE, m.MoveOriginatorAETitle))
		ds.Set(dicom.NewUS(dicom.TagMoveOriginatorMessageID, m.MoveOriginatorMessageID))
	}
	return ds
}

func parseCStoreRequest(ds *dicom.DataSet) *CStoreRequest {
	m := &CStoreRequest{}
	m.MessageID, _ = ds.GetUint16(dicom.TagMessageID)
	m.AffectedSOPClassUID, _ = ds.GetString(dicom.TagAffectedSOPClassUID)
	m.AffectedSOPInstanceUID, _ = ds.GetString(dicom.TagAffectedSOPInstanceUID)
	if p, ok := ds.GetUint16(dicom.TagPriority); ok {
		m.Priority = Priority(p)
	}
	m.MoveOriginatorAETitle, _ = ds.GetString(dicom.TagMoveOriginatorAETitle)
	m.MoveOriginatorMessageID, _ = ds.GetUint16(dicom.TagMoveOriginatorMessageID)
	return m
}

// CStoreResponse is a C-STORE response primitive.
type CStoreResponse struct {
	MessageIDBeingRespondedTo uint16
	AffectedSOPClassUID       string
	AffectedSOPInstanceUID    string
	Status                    Status
}

func (m *CStoreResponse) CommandField() CommandField { return CStoreRSP }
func (m *CStoreResponse) HasDataSet() bool           { return false }

func (m *CStoreResponse) CommandSet() *dicom.DataSet {
	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUI(dicom.TagAffectedSOPClassUID, m.AffectedSOPClassUID))
	ds.Set(dicom.NewUS(dicom.TagCommandField, uint16(CStoreRSP)))
	ds.Set(dicom.NewUS(dicom.TagMessageIDBeingRespondedTo, m.MessageIDBeingRespondedTo))
	ds.Set(dicom.NewUS(dicom.TagCommandDataSetType, dataSetTypeAbsent))
	ds.Set(dicom.NewUS(dicom.TagStatus, uint16(m.Status)))
	if m.AffectedSOPInstanceUID != "" {
		ds.Set(dicom.NewUI(dicom.TagAffectedSOPInstanceUID, m.AffectedSOPInstanceUID))
	}
	return ds
}

func parseCStoreResponse(ds *dicom.DataSet) *CStoreResponse {
	m := &CStoreResponse{}
	m.MessageIDBeingRespondedTo, _ = ds.GetUint16(dicom.TagMessageIDBeingRespondedTo)
	m.AffectedSOPClassUID, _ = ds.GetString(dicom.TagAffectedSOPClassUID)
	m.AffectedSOPInstanceUID, _ = ds.GetString(dicom.TagAffectedSOPInstanceUID)
	st, _ := ds.GetUint16(dicom.TagStatus)
	m.Status = Status(st)
	return m
}

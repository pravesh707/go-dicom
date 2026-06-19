// SPDX-License-Identifier: Apache-2.0

package dimse

import "github.com/pravesh707/go-dicom/dicom"

func init() {
	RegisterMessage(CFindRQ, func(ds *dicom.DataSet) Message { return parseCFindRequest(ds) })
	RegisterMessage(CFindRSP, func(ds *dicom.DataSet) Message { return parseCFindResponse(ds) })
}

// CFindRequest is a C-FIND request primitive (PS3.7 §9.1.2). The query keys
// travel as the Identifier data set.
type CFindRequest struct {
	MessageID           uint16
	AffectedSOPClassUID string
	Priority            Priority
	Identifier          *dicom.DataSet
}

func (m *CFindRequest) CommandField() CommandField { return CFindRQ }
func (m *CFindRequest) HasDataSet() bool           { return true }
func (m *CFindRequest) GetMessageID() uint16       { return m.MessageID }

func (m *CFindRequest) CommandSet() *dicom.DataSet {
	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUI(dicom.TagAffectedSOPClassUID, m.AffectedSOPClassUID))
	ds.Set(dicom.NewUS(dicom.TagCommandField, uint16(CFindRQ)))
	ds.Set(dicom.NewUS(dicom.TagMessageID, m.MessageID))
	ds.Set(dicom.NewUS(dicom.TagPriority, uint16(m.Priority)))
	ds.Set(dicom.NewUS(dicom.TagCommandDataSetType, dataSetTypePresent))
	return ds
}

func parseCFindRequest(ds *dicom.DataSet) *CFindRequest {
	m := &CFindRequest{}
	m.MessageID, _ = ds.GetUint16(dicom.TagMessageID)
	m.AffectedSOPClassUID, _ = ds.GetString(dicom.TagAffectedSOPClassUID)
	if p, ok := ds.GetUint16(dicom.TagPriority); ok {
		m.Priority = Priority(p)
	}
	return m
}

// CFindResponse is a C-FIND response primitive. A Pending response carries a
// matching Identifier; the final response carries only a status.
type CFindResponse struct {
	MessageIDBeingRespondedTo uint16
	AffectedSOPClassUID       string
	Status                    Status
	Identifier                *dicom.DataSet

	// hasData records the (0000,0800) flag on a decoded response, so a parsed
	// pending response (whose Identifier field is not populated until the data
	// set is read) still reports that a data set follows.
	hasData bool
}

func (m *CFindResponse) CommandField() CommandField { return CFindRSP }
func (m *CFindResponse) HasDataSet() bool           { return m.Identifier != nil || m.hasData }

func (m *CFindResponse) CommandSet() *dicom.DataSet {
	dst := dataSetTypeAbsent
	if m.Identifier != nil {
		dst = dataSetTypePresent
	}
	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUI(dicom.TagAffectedSOPClassUID, m.AffectedSOPClassUID))
	ds.Set(dicom.NewUS(dicom.TagCommandField, uint16(CFindRSP)))
	ds.Set(dicom.NewUS(dicom.TagMessageIDBeingRespondedTo, m.MessageIDBeingRespondedTo))
	ds.Set(dicom.NewUS(dicom.TagCommandDataSetType, uint16(dst)))
	ds.Set(dicom.NewUS(dicom.TagStatus, uint16(m.Status)))
	return ds
}

func parseCFindResponse(ds *dicom.DataSet) *CFindResponse {
	m := &CFindResponse{hasData: hasDataSet(ds)}
	m.MessageIDBeingRespondedTo, _ = ds.GetUint16(dicom.TagMessageIDBeingRespondedTo)
	m.AffectedSOPClassUID, _ = ds.GetString(dicom.TagAffectedSOPClassUID)
	st, _ := ds.GetUint16(dicom.TagStatus)
	m.Status = Status(st)
	return m
}

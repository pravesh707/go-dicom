// SPDX-License-Identifier: Apache-2.0

package dimse

import "github.com/pravesh707/go-dicom/dicom"

func init() {
	RegisterMessage(CMoveRQ, func(ds *dicom.DataSet) Message { return parseCMoveRequest(ds) })
	RegisterMessage(CMoveRSP, func(ds *dicom.DataSet) Message { return parseRetrieveResponse(ds, CMoveRSP) })
	RegisterMessage(CGetRQ, func(ds *dicom.DataSet) Message { return parseCGetRequest(ds) })
	RegisterMessage(CGetRSP, func(ds *dicom.DataSet) Message { return parseRetrieveResponse(ds, CGetRSP) })
}

// SubOperations holds the running C-MOVE/C-GET sub-operation counters reported
// in pending and final responses (PS3.7 §9.1.3 / §9.1.4).
type SubOperations struct {
	Remaining uint16
	Completed uint16
	Failed    uint16
	Warning   uint16
}

func (s SubOperations) set(ds *dicom.DataSet) {
	ds.Set(dicom.NewUS(dicom.TagNumberOfRemainingSubops, s.Remaining))
	ds.Set(dicom.NewUS(dicom.TagNumberOfCompletedSubops, s.Completed))
	ds.Set(dicom.NewUS(dicom.TagNumberOfFailedSubops, s.Failed))
	ds.Set(dicom.NewUS(dicom.TagNumberOfWarningSubops, s.Warning))
}

func subOperationsFrom(ds *dicom.DataSet) SubOperations {
	var s SubOperations
	s.Remaining, _ = ds.GetUint16(dicom.TagNumberOfRemainingSubops)
	s.Completed, _ = ds.GetUint16(dicom.TagNumberOfCompletedSubops)
	s.Failed, _ = ds.GetUint16(dicom.TagNumberOfFailedSubops)
	s.Warning, _ = ds.GetUint16(dicom.TagNumberOfWarningSubops)
	return s
}

// ---- C-MOVE ----

// CMoveRequest is a C-MOVE request primitive. The query keys travel as the
// Identifier; MoveDestination names the AE to receive the matched instances.
type CMoveRequest struct {
	MessageID           uint16
	AffectedSOPClassUID string
	Priority            Priority
	MoveDestination     string
	Identifier          *dicom.DataSet
}

func (m *CMoveRequest) CommandField() CommandField { return CMoveRQ }
func (m *CMoveRequest) HasDataSet() bool           { return true }
func (m *CMoveRequest) GetMessageID() uint16       { return m.MessageID }

func (m *CMoveRequest) CommandSet() *dicom.DataSet {
	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUI(dicom.TagAffectedSOPClassUID, m.AffectedSOPClassUID))
	ds.Set(dicom.NewUS(dicom.TagCommandField, uint16(CMoveRQ)))
	ds.Set(dicom.NewUS(dicom.TagMessageID, m.MessageID))
	ds.Set(dicom.NewUS(dicom.TagPriority, uint16(m.Priority)))
	ds.Set(dicom.NewString(dicom.TagMoveDestination, dicom.VRAE, m.MoveDestination))
	ds.Set(dicom.NewUS(dicom.TagCommandDataSetType, dataSetTypePresent))
	return ds
}

func parseCMoveRequest(ds *dicom.DataSet) *CMoveRequest {
	m := &CMoveRequest{}
	m.MessageID, _ = ds.GetUint16(dicom.TagMessageID)
	m.AffectedSOPClassUID, _ = ds.GetString(dicom.TagAffectedSOPClassUID)
	if p, ok := ds.GetUint16(dicom.TagPriority); ok {
		m.Priority = Priority(p)
	}
	m.MoveDestination, _ = ds.GetString(dicom.TagMoveDestination)
	return m
}

// ---- C-GET ----

// CGetRequest is a C-GET request primitive. Matched instances are returned over
// the same association as C-STORE sub-operations.
type CGetRequest struct {
	MessageID           uint16
	AffectedSOPClassUID string
	Priority            Priority
	Identifier          *dicom.DataSet
}

func (m *CGetRequest) CommandField() CommandField { return CGetRQ }
func (m *CGetRequest) HasDataSet() bool           { return true }
func (m *CGetRequest) GetMessageID() uint16       { return m.MessageID }

func (m *CGetRequest) CommandSet() *dicom.DataSet {
	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUI(dicom.TagAffectedSOPClassUID, m.AffectedSOPClassUID))
	ds.Set(dicom.NewUS(dicom.TagCommandField, uint16(CGetRQ)))
	ds.Set(dicom.NewUS(dicom.TagMessageID, m.MessageID))
	ds.Set(dicom.NewUS(dicom.TagPriority, uint16(m.Priority)))
	ds.Set(dicom.NewUS(dicom.TagCommandDataSetType, dataSetTypePresent))
	return ds
}

func parseCGetRequest(ds *dicom.DataSet) *CGetRequest {
	m := &CGetRequest{}
	m.MessageID, _ = ds.GetUint16(dicom.TagMessageID)
	m.AffectedSOPClassUID, _ = ds.GetString(dicom.TagAffectedSOPClassUID)
	if p, ok := ds.GetUint16(dicom.TagPriority); ok {
		m.Priority = Priority(p)
	}
	return m
}

// ---- Shared C-MOVE / C-GET response ----

// RetrieveResponse is a C-MOVE or C-GET response, distinguished by Command. It
// carries the sub-operation counters and an optional identifier (a Failed SOP
// Instance UID List on failure).
type RetrieveResponse struct {
	Command                   CommandField // CMoveRSP or CGetRSP
	MessageIDBeingRespondedTo uint16
	AffectedSOPClassUID       string
	Status                    Status
	SubOps                    SubOperations
	Identifier                *dicom.DataSet
	hasData                   bool
}

func (m *RetrieveResponse) CommandField() CommandField { return m.Command }
func (m *RetrieveResponse) HasDataSet() bool           { return m.Identifier != nil || m.hasData }

func (m *RetrieveResponse) CommandSet() *dicom.DataSet {
	dst := dataSetTypeAbsent
	if m.Identifier != nil {
		dst = dataSetTypePresent
	}
	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUI(dicom.TagAffectedSOPClassUID, m.AffectedSOPClassUID))
	ds.Set(dicom.NewUS(dicom.TagCommandField, uint16(m.Command)))
	ds.Set(dicom.NewUS(dicom.TagMessageIDBeingRespondedTo, m.MessageIDBeingRespondedTo))
	ds.Set(dicom.NewUS(dicom.TagCommandDataSetType, uint16(dst)))
	ds.Set(dicom.NewUS(dicom.TagStatus, uint16(m.Status)))
	m.SubOps.set(ds)
	return ds
}

func parseRetrieveResponse(ds *dicom.DataSet, cf CommandField) *RetrieveResponse {
	m := &RetrieveResponse{Command: cf, hasData: hasDataSet(ds)}
	m.MessageIDBeingRespondedTo, _ = ds.GetUint16(dicom.TagMessageIDBeingRespondedTo)
	m.AffectedSOPClassUID, _ = ds.GetString(dicom.TagAffectedSOPClassUID)
	st, _ := ds.GetUint16(dicom.TagStatus)
	m.Status = Status(st)
	m.SubOps = subOperationsFrom(ds)
	return m
}

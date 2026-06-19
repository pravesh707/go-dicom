// SPDX-License-Identifier: Apache-2.0

package dimse

import (
	"fmt"

	"github.com/pravesh707/go-dicom/dicom"
)

// Message is a DIMSE service primitive that can be encoded to / decoded from a
// command set.
type Message interface {
	// CommandField returns the command type.
	CommandField() CommandField
	// HasDataSet reports whether a data set follows the command.
	HasDataSet() bool
	// CommandSet builds the command data elements (group 0000), excluding the
	// auto-computed Command Group Length.
	CommandSet() *dicom.DataSet
}

// EncodeCommand encodes a message's command set in Implicit VR Little Endian,
// inserting the Command Group Length element.
func EncodeCommand(m Message) ([]byte, error) {
	return dicom.EncodeCommandSet(m.CommandSet())
}

// MessageParser builds a typed Message from a decoded command set. It is the
// Factory function registered per command field.
type MessageParser func(*dicom.DataSet) Message

// messageRegistry maps a command field to the parser that produces its typed
// primitive. New DIMSE services register themselves here (Open/Closed), so the
// dispatch below never needs editing.
var messageRegistry = map[CommandField]MessageParser{}

// RegisterMessage registers a parser for a command field. Intended for package
// init; not safe for concurrent use with DecodeCommand.
func RegisterMessage(cf CommandField, p MessageParser) { messageRegistry[cf] = p }

func init() {
	RegisterMessage(CEchoRQ, func(ds *dicom.DataSet) Message { return parseCEchoRequest(ds) })
	RegisterMessage(CEchoRSP, func(ds *dicom.DataSet) Message { return parseCEchoResponse(ds) })
}

// DecodeCommand parses a command set into a typed Message via the registry.
// Unregistered command fields yield a *RawMessage carrying the command set.
func DecodeCommand(b []byte) (Message, error) {
	ds, err := dicom.Decode(b, dicom.ImplicitVRLittleEndian)
	if err != nil {
		return nil, err
	}
	cfVal, ok := ds.GetUint16(dicom.TagCommandField)
	if !ok {
		return nil, fmt.Errorf("dimse: command set missing Command Field (0000,0100)")
	}
	cf := CommandField(cfVal)
	if parse, ok := messageRegistry[cf]; ok {
		return parse(ds), nil
	}
	return &RawMessage{Command: cf, Set: ds}, nil
}

func hasDataSet(ds *dicom.DataSet) bool {
	v, ok := ds.GetUint16(dicom.TagCommandDataSetType)
	if !ok {
		return false
	}
	return v != dataSetTypeAbsent
}

// ---- C-ECHO ----

// CEchoRequest is a C-ECHO request primitive (PS3.7 §9.1.5).
type CEchoRequest struct {
	MessageID           uint16
	AffectedSOPClassUID string // defaults to the Verification SOP Class
}

func (m *CEchoRequest) CommandField() CommandField { return CEchoRQ }
func (m *CEchoRequest) HasDataSet() bool           { return false }

func (m *CEchoRequest) CommandSet() *dicom.DataSet {
	sop := m.AffectedSOPClassUID
	if sop == "" {
		sop = dicom.VerificationSOPClass
	}
	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUI(dicom.TagAffectedSOPClassUID, sop))
	ds.Set(dicom.NewUS(dicom.TagCommandField, uint16(CEchoRQ)))
	ds.Set(dicom.NewUS(dicom.TagMessageID, m.MessageID))
	ds.Set(dicom.NewUS(dicom.TagCommandDataSetType, dataSetTypeAbsent))
	return ds
}

func parseCEchoRequest(ds *dicom.DataSet) *CEchoRequest {
	m := &CEchoRequest{}
	m.MessageID, _ = ds.GetUint16(dicom.TagMessageID)
	m.AffectedSOPClassUID, _ = ds.GetString(dicom.TagAffectedSOPClassUID)
	return m
}

// CEchoResponse is a C-ECHO response primitive.
type CEchoResponse struct {
	MessageIDBeingRespondedTo uint16
	AffectedSOPClassUID       string
	Status                    Status
}

func (m *CEchoResponse) CommandField() CommandField { return CEchoRSP }
func (m *CEchoResponse) HasDataSet() bool           { return false }

func (m *CEchoResponse) CommandSet() *dicom.DataSet {
	sop := m.AffectedSOPClassUID
	if sop == "" {
		sop = dicom.VerificationSOPClass
	}
	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUI(dicom.TagAffectedSOPClassUID, sop))
	ds.Set(dicom.NewUS(dicom.TagCommandField, uint16(CEchoRSP)))
	ds.Set(dicom.NewUS(dicom.TagMessageIDBeingRespondedTo, m.MessageIDBeingRespondedTo))
	ds.Set(dicom.NewUS(dicom.TagCommandDataSetType, dataSetTypeAbsent))
	ds.Set(dicom.NewUS(dicom.TagStatus, uint16(m.Status)))
	return ds
}

func parseCEchoResponse(ds *dicom.DataSet) *CEchoResponse {
	m := &CEchoResponse{}
	m.MessageIDBeingRespondedTo, _ = ds.GetUint16(dicom.TagMessageIDBeingRespondedTo)
	m.AffectedSOPClassUID, _ = ds.GetString(dicom.TagAffectedSOPClassUID)
	st, _ := ds.GetUint16(dicom.TagStatus)
	m.Status = Status(st)
	return m
}

// RawMessage wraps a command set for which no typed primitive is implemented
// yet. It lets associations route and inspect any DIMSE message.
type RawMessage struct {
	Command CommandField
	Set     *dicom.DataSet
}

func (m *RawMessage) CommandField() CommandField { return m.Command }
func (m *RawMessage) HasDataSet() bool           { return hasDataSet(m.Set) }
func (m *RawMessage) CommandSet() *dicom.DataSet { return m.Set }

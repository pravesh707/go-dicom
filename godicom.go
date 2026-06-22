// SPDX-License-Identifier: Apache-2.0

package godicom

import (
	"github.com/pravesh707/go-dicom/association"
	"github.com/pravesh707/go-dicom/dicom"
	"github.com/pravesh707/go-dicom/dimse"
)

// Re-exported core types, so callers usually import only this package.
type (
	// Association is an established DICOM association.
	Association = association.Association
	// Event is passed to handlers for association and DIMSE events.
	Event = association.Event
	// Handler handles an intervention event and returns a DIMSE status.
	Handler = association.Handler
	// HandlerBinding binds an event type to a handler.
	HandlerBinding = association.HandlerBinding
	// RequestedContext is an SCU-proposed presentation context.
	RequestedContext = association.RequestedContext
	// SupportedContext is an SCP-supported presentation context.
	SupportedContext = association.SupportedContext
	// EventType identifies an association/DIMSE event.
	EventType = association.EventType
	// Status is a DIMSE status code.
	Status = dimse.Status
	// DataSet is a DICOM data set.
	DataSet = dicom.DataSet
	// File is a DICOM Part-10 file (file meta + data set).
	File = dicom.File
	// RetrieveResult summarizes a completed C-MOVE/C-GET.
	RetrieveResult = association.RetrieveResult
	// StoreCallback handles an instance received during a C-GET.
	StoreCallback = association.StoreCallback
)

// Part-10 file helpers, re-exported for convenience.
var (
	// ReadFile parses a DICOM Part-10 file from disk.
	ReadFile = dicom.ReadFile
	// NewFile builds a Part-10 file from a data set and transfer syntax.
	NewFile = dicom.NewFile
)

// Event type constants (mirror pynetdicom's evt.EVT_*).
const (
	EvtCEcho        = association.EvtCEcho
	EvtCStore       = association.EvtCStore
	EvtCFind        = association.EvtCFind
	EvtCGet         = association.EvtCGet
	EvtCMove        = association.EvtCMove
	EvtNEventReport = association.EvtNEventReport
	EvtNGet         = association.EvtNGet
	EvtNSet         = association.EvtNSet
	EvtNAction      = association.EvtNAction
	EvtNCreate      = association.EvtNCreate
	EvtNDelete      = association.EvtNDelete
	EvtEstablished  = association.EvtEstablished
	EvtReleased     = association.EvtReleased
	EvtAborted      = association.EvtAborted
	EvtDIMSERecv    = association.EvtDIMSERecv
)

// Commonly used UID constants.
const (
	ImplicitVRLittleEndian = dicom.ImplicitVRLittleEndian
	ExplicitVRLittleEndian = dicom.ExplicitVRLittleEndian
	VerificationSOPClass   = dicom.VerificationSOPClass

	// Query/Retrieve information model SOP Classes (PS3.4 Annex C).
	PatientRootQueryRetrieveFind = "1.2.840.10008.5.1.4.1.2.1.1"
	PatientRootQueryRetrieveMove = "1.2.840.10008.5.1.4.1.2.1.2"
	PatientRootQueryRetrieveGet  = "1.2.840.10008.5.1.4.1.2.1.3"
	StudyRootQueryRetrieveFind   = "1.2.840.10008.5.1.4.1.2.2.1"
	StudyRootQueryRetrieveMove   = "1.2.840.10008.5.1.4.1.2.2.2"
	StudyRootQueryRetrieveGet    = "1.2.840.10008.5.1.4.1.2.2.3"
)

// StatusSuccess is the DIMSE success status (0x0000), the usual handler return.
const StatusSuccess = dimse.StatusSuccess

// DefaultTransferSyntaxes is the default set proposed/accepted when a context is
// added without explicit transfer syntaxes (Explicit then Implicit VR Little
// Endian), matching common SCU behaviour.
var DefaultTransferSyntaxes = []string{
	dicom.ExplicitVRLittleEndian,
	dicom.ImplicitVRLittleEndian,
}

// AE is a DICOM Application Entity: a factory for associations (as SCU) and
// servers (as SCP). The zero value is not usable; construct with NewAE.
type AE struct {
	AETitle                   string
	MaximumLength             uint32
	ImplementationClassUID    string
	ImplementationVersionName string
	// RequireCalledAET, when true, rejects requests whose Called AE Title does
	// not match AETitle.
	RequireCalledAET bool

	transport        Transport
	requested        []RequestedContext
	supported        []SupportedContext
	moveDestinations map[string]string // C-MOVE destination AE title -> host:port
}

// AddMoveDestination registers a C-MOVE destination: instances retrieved with a
// C-MOVE naming this AE title are forwarded to addr ("host:port").
func (ae *AE) AddMoveDestination(aeTitle, addr string) {
	if ae.moveDestinations == nil {
		ae.moveDestinations = make(map[string]string)
	}
	ae.moveDestinations[aeTitle] = addr
}

func (ae *AE) moveResolver() func(string) (string, bool) {
	if len(ae.moveDestinations) == 0 {
		return nil
	}
	m := ae.moveDestinations
	return func(aet string) (string, bool) { a, ok := m[aet]; return a, ok }
}

// moveStorageContexts converts the AE's supported contexts into the contexts
// proposed to a C-MOVE destination when forwarding instances.
func (ae *AE) moveStorageContexts() []RequestedContext {
	out := make([]RequestedContext, 0, len(ae.supported))
	for _, s := range ae.supported {
		out = append(out, RequestedContext{AbstractSyntax: s.AbstractSyntax, TransferSyntaxes: s.TransferSyntaxes})
	}
	return out
}

// NewAE returns an AE with sensible defaults and the given AE title. Behaviour
// is customised with functional options (see With…), the idiomatic Go
// configuration pattern.
func NewAE(aeTitle string, opts ...Option) *AE {
	ae := &AE{
		AETitle:                   aeTitle,
		MaximumLength:             16384,
		ImplementationClassUID:    dicom.GoDICOMImplementationClassUID,
		ImplementationVersionName: dicom.GoDICOMImplementationVersionName,
		transport:                 TCPTransport{},
	}
	for _, opt := range opts {
		opt(ae)
	}
	return ae
}

// AddRequestedContext adds a presentation context to propose when acting as an
// SCU. With no transfer syntaxes, DefaultTransferSyntaxes is used.
func (ae *AE) AddRequestedContext(abstractSyntax string, transferSyntaxes ...string) {
	if len(transferSyntaxes) == 0 {
		transferSyntaxes = DefaultTransferSyntaxes
	}
	ae.requested = append(ae.requested, RequestedContext{
		AbstractSyntax:   abstractSyntax,
		TransferSyntaxes: transferSyntaxes,
	})
}

// AddRequestedContextWithRole adds an SCU presentation context that also
// proposes SCP/SCU Role Selection (PS3.7 §D.3.3.4). Use it for the storage
// contexts of a C-GET — set scpRole true so the peer may return matched
// instances over the same association. With no transfer syntaxes,
// DefaultTransferSyntaxes is used.
func (ae *AE) AddRequestedContextWithRole(abstractSyntax string, scuRole, scpRole bool, transferSyntaxes ...string) {
	if len(transferSyntaxes) == 0 {
		transferSyntaxes = DefaultTransferSyntaxes
	}
	ae.requested = append(ae.requested, RequestedContext{
		AbstractSyntax:   abstractSyntax,
		TransferSyntaxes: transferSyntaxes,
		ScuRole:          scuRole,
		ScpRole:          scpRole,
	})
}

// AddSupportedContext adds a presentation context to accept when acting as an
// SCP. With no transfer syntaxes, DefaultTransferSyntaxes is used.
func (ae *AE) AddSupportedContext(abstractSyntax string, transferSyntaxes ...string) {
	if len(transferSyntaxes) == 0 {
		transferSyntaxes = DefaultTransferSyntaxes
	}
	ae.supported = append(ae.supported, SupportedContext{
		AbstractSyntax:   abstractSyntax,
		TransferSyntaxes: transferSyntaxes,
	})
}

// RequestedContexts returns the configured SCU contexts.
func (ae *AE) RequestedContexts() []RequestedContext { return ae.requested }

// SupportedContexts returns the configured SCP contexts.
func (ae *AE) SupportedContexts() []SupportedContext { return ae.supported }

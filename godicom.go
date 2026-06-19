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
)

// Event type constants.
const (
	EvtCEcho       = association.EvtCEcho
	EvtEstablished = association.EvtEstablished
	EvtReleased    = association.EvtReleased
	EvtAborted     = association.EvtAborted
	EvtDIMSERecv   = association.EvtDIMSERecv
)

// Commonly used UID constants.
const (
	ImplicitVRLittleEndian = dicom.ImplicitVRLittleEndian
	ExplicitVRLittleEndian = dicom.ExplicitVRLittleEndian
	VerificationSOPClass   = dicom.VerificationSOPClass
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

	transport Transport
	requested []RequestedContext
	supported []SupportedContext
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

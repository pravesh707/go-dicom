// SPDX-License-Identifier: Apache-2.0

// Package dimse implements the DICOM Message Service Element (PS3.7): the
// DIMSE-C and DIMSE-N service primitives, their command-set encoding, and the
// status code system. Command sets are always encoded in Implicit VR Little
// Endian.
package dimse

// Status is a 16-bit DIMSE status code (PS3.7 Annex C).
type Status uint16

// Category classifies a status code into one of the five DIMSE service-response
// categories.
type Category int

const (
	CategorySuccess Category = iota
	CategoryPending
	CategoryCancel
	CategoryWarning
	CategoryFailure
)

// Common status codes (PS3.7 Annex C). Service classes define additional
// service-specific failure codes in the 0xAxxx/0xCxxx ranges.
const (
	StatusSuccess Status = 0x0000

	StatusCancel Status = 0xFE00

	StatusPending        Status = 0xFF00
	StatusPendingWarning Status = 0xFF01

	// Warnings.
	StatusCoercionOfDataElements   Status = 0xB000
	StatusElementsDiscarded        Status = 0xB006
	StatusDataSetDoesNotMatchSOP   Status = 0xB007
	StatusAttributeListError       Status = 0x0107
	StatusAttributeValueOutOfRange Status = 0x0116

	// Failures.
	StatusRefusedOutOfResources       Status = 0xA700
	StatusRefusedSOPClassNotSupported Status = 0x0122
	StatusErrorCannotUnderstand       Status = 0xC000
	StatusErrorDataSetDoesNotMatch    Status = 0xA900
	StatusInvalidArgumentValue        Status = 0x0115
	StatusInvalidAttributeValue       Status = 0x0106
	StatusInvalidObjectInstance       Status = 0x0117
	StatusMissingAttribute            Status = 0x0120
	StatusMissingAttributeValue       Status = 0x0121
	StatusNoSuchSOPClass              Status = 0x0118
	StatusClassInstanceConflict       Status = 0x0119
	StatusProcessingFailure           Status = 0x0110
	StatusUnrecognizedOperation       Status = 0x0211
	StatusNotAuthorized               Status = 0x0124
)

// Category returns the response category for a status code.
func (s Status) Category() Category {
	switch {
	case s == StatusSuccess:
		return CategorySuccess
	case s == StatusPending || s == StatusPendingWarning:
		return CategoryPending
	case s == StatusCancel:
		return CategoryCancel
	case s == 0x0001 || (s&0xF000) == 0xB000:
		return CategoryWarning
	default:
		return CategoryFailure
	}
}

// IsSuccess reports whether the status indicates success.
func (s Status) IsSuccess() bool { return s.Category() == CategorySuccess }

// IsPending reports whether the status indicates a pending (intermediate)
// response, as used by C-FIND/C-GET/C-MOVE.
func (s Status) IsPending() bool { return s.Category() == CategoryPending }

func (c Category) String() string {
	switch c {
	case CategorySuccess:
		return "Success"
	case CategoryPending:
		return "Pending"
	case CategoryCancel:
		return "Cancel"
	case CategoryWarning:
		return "Warning"
	default:
		return "Failure"
	}
}

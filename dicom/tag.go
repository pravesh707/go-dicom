// SPDX-License-Identifier: Apache-2.0

// Package dicom is a from-scratch implementation of the DICOM data layer:
// data elements, data sets, the value-representation (VR) system, a data
// dictionary, and encoders/decoders for the Implicit and Explicit VR Little
// Endian transfer syntaxes.
//
// It is the rough equivalent of pydicom, on top of which the networking
// packages (pdu, dimse, association) are built
package dicom

import "fmt"

// Tag identifies a DICOM data element by its (group, element) number.
type Tag struct {
	Group   uint16
	Element uint16
}

// String returns the canonical "(gggg,eeee)" representation.
func (t Tag) String() string {
	return fmt.Sprintf("(%04X,%04X)", t.Group, t.Element)
}

// IsCommand reports whether the tag belongs to the command group (0000).
func (t Tag) IsCommand() bool { return t.Group == 0x0000 }

// IsPrivate reports whether the tag is private (odd group number).
func (t Tag) IsPrivate() bool { return t.Group%2 == 1 && t.Group != 0x0001 && t.Group != 0x0003 }

// IsGroupLength reports whether the tag is a group-length element (gggg,0000).
func (t Tag) IsGroupLength() bool { return t.Element == 0x0000 }

// Less orders tags by group then element, which is the order data elements
// must appear in within an encoded data set.
func (t Tag) Less(other Tag) bool {
	if t.Group != other.Group {
		return t.Group < other.Group
	}
	return t.Element < other.Element
}

// Item-delimitation tags used in sequence (SQ) and undefined-length encoding.
var (
	TagItem                 = Tag{0xFFFE, 0xE000}
	TagItemDelimitationItem = Tag{0xFFFE, 0xE00D}
	TagSequenceDelimitation = Tag{0xFFFE, 0xE0DD}
	TagPixelData            = Tag{0x7FE0, 0x0010}
)

// Command-set (group 0000) tags — PS3.7 §9.3 / §10.3.
var (
	TagCommandGroupLength        = Tag{0x0000, 0x0000} // UL
	TagAffectedSOPClassUID       = Tag{0x0000, 0x0002} // UI
	TagRequestedSOPClassUID      = Tag{0x0000, 0x0003} // UI
	TagCommandField              = Tag{0x0000, 0x0100} // US
	TagMessageID                 = Tag{0x0000, 0x0110} // US
	TagMessageIDBeingRespondedTo = Tag{0x0000, 0x0120} // US
	TagMoveDestination           = Tag{0x0000, 0x0600} // AE
	TagPriority                  = Tag{0x0000, 0x0700} // US
	TagCommandDataSetType        = Tag{0x0000, 0x0800} // US
	TagStatus                    = Tag{0x0000, 0x0900} // US
	TagOffendingElement          = Tag{0x0000, 0x0901} // AT
	TagErrorComment              = Tag{0x0000, 0x0902} // LO
	TagErrorID                   = Tag{0x0000, 0x0903} // US
	TagAffectedSOPInstanceUID    = Tag{0x0000, 0x1000} // UI
	TagRequestedSOPInstanceUID   = Tag{0x0000, 0x1001} // UI
	TagEventTypeID               = Tag{0x0000, 0x1002} // US
	TagAttributeIdentifierList   = Tag{0x0000, 0x1005} // AT
	TagActionTypeID              = Tag{0x0000, 0x1008} // US
	TagNumberOfRemainingSubops   = Tag{0x0000, 0x1020} // US
	TagNumberOfCompletedSubops   = Tag{0x0000, 0x1021} // US
	TagNumberOfFailedSubops      = Tag{0x0000, 0x1022} // US
	TagNumberOfWarningSubops     = Tag{0x0000, 0x1023} // US
	TagMoveOriginatorAETitle     = Tag{0x0000, 0x1030} // AE
	TagMoveOriginatorMessageID   = Tag{0x0000, 0x1031} // US
)

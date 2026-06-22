// SPDX-License-Identifier: Apache-2.0

package dicom

// VR is a DICOM Value Representation (PS3.5 §6.2). It is encoded explicitly in
// the Explicit VR transfer syntaxes and looked up from the data dictionary in
// the Implicit VR transfer syntax.
type VR string

const (
	VRAE VR = "AE" // Application Entity
	VRAS VR = "AS" // Age String
	VRAT VR = "AT" // Attribute Tag
	VRCS VR = "CS" // Code String
	VRDA VR = "DA" // Date
	VRDS VR = "DS" // Decimal String
	VRDT VR = "DT" // Date Time
	VRFL VR = "FL" // Floating Point Single
	VRFD VR = "FD" // Floating Point Double
	VRIS VR = "IS" // Integer String
	VRLO VR = "LO" // Long String
	VRLT VR = "LT" // Long Text
	VROB VR = "OB" // Other Byte
	VROD VR = "OD" // Other Double
	VROF VR = "OF" // Other Float
	VROL VR = "OL" // Other Long
	VROV VR = "OV" // Other Very Long
	VROW VR = "OW" // Other Word
	VRPN VR = "PN" // Person Name
	VRSH VR = "SH" // Short String
	VRSL VR = "SL" // Signed Long
	VRSQ VR = "SQ" // Sequence of Items
	VRSS VR = "SS" // Signed Short
	VRST VR = "ST" // Short Text
	VRSV VR = "SV" // Signed Very Long
	VRTM VR = "TM" // Time
	VRUC VR = "UC" // Unlimited Characters
	VRUI VR = "UI" // Unique Identifier (UID)
	VRUL VR = "UL" // Unsigned Long
	VRUN VR = "UN" // Unknown
	VRUR VR = "UR" // URI/URL
	VRUS VR = "US" // Unsigned Short
	VRUT VR = "UT" // Unlimited Text
	VRUV VR = "UV" // Unsigned Very Long
)

// UsesLongLength reports whether, in the Explicit VR encoding, this VR is
// written with a 2-byte reserved field followed by a 4-byte length (true), or
// a plain 2-byte length (false). PS3.5 §7.1.2.
func (vr VR) UsesLongLength() bool {
	switch vr {
	case VROB, VROD, VROF, VROL, VROV, VROW, VRSQ, VRUC, VRUN, VRUR, VRUT, VRSV, VRUV:
		return true
	}
	return false
}

// PadByte is the byte used to pad odd-length values to even length. Text-like
// VRs pad with a space; UI pads with NUL; binary/unknown pad with NUL.
func (vr VR) PadByte() byte {
	switch vr {
	case VRUI:
		return 0x00
	case VRAE, VRAS, VRCS, VRDA, VRDS, VRDT, VRIS, VRLO, VRLT, VRPN, VRSH, VRST,
		VRTM, VRUC, VRUR, VRUT:
		return 0x20 // space
	default:
		return 0x00
	}
}

// IsString reports whether the VR carries a (possibly multi-valued, backslash
// separated) text value rather than raw binary.
func (vr VR) IsString() bool {
	switch vr {
	case VRAE, VRAS, VRCS, VRDA, VRDS, VRDT, VRIS, VRLO, VRLT, VRPN, VRSH, VRST,
		VRTM, VRUC, VRUI, VRUR, VRUT:
		return true
	}
	return false
}

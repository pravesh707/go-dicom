// SPDX-License-Identifier: Apache-2.0

package dicom

import (
	"encoding/binary"
	"strings"
)

// Element is a single DICOM data element. The value is stored as raw bytes in
// little-endian form (the representation used by both Implicit and Explicit VR
// Little Endian). Typed accessors decode it on demand.
type Element struct {
	Tag Tag
	VR  VR
	Raw []byte
	// undefined marks a value that was encoded with undefined length
	// (0xFFFFFFFF) — sequences and encapsulated pixel data. Raw then contains
	// the full item stream including the closing Sequence Delimitation Item.
	undefined bool
}

// NewElement builds an element from raw little-endian bytes, padding to even
// length using the VR's pad byte.
func NewElement(tag Tag, vr VR, raw []byte) *Element {
	return &Element{Tag: tag, VR: vr, Raw: padEven(raw, vr.padByte())}
}

// NewUS builds an Unsigned Short (US) element from one or more uint16 values.
func NewUS(tag Tag, values ...uint16) *Element {
	b := make([]byte, 2*len(values))
	for i, v := range values {
		binary.LittleEndian.PutUint16(b[2*i:], v)
	}
	return &Element{Tag: tag, VR: VRUS, Raw: b}
}

// NewUL builds an Unsigned Long (UL) element from one or more uint32 values.
func NewUL(tag Tag, values ...uint32) *Element {
	b := make([]byte, 4*len(values))
	for i, v := range values {
		binary.LittleEndian.PutUint32(b[4*i:], v)
	}
	return &Element{Tag: tag, VR: VRUL, Raw: b}
}

// NewUI builds a UID (UI) element, NUL-padded to even length.
func NewUI(tag Tag, uid string) *Element {
	return &Element{Tag: tag, VR: VRUI, Raw: padEven([]byte(uid), 0x00)}
}

// NewString builds a text element with the given VR, padded to even length
// using that VR's pad byte. Multiple values are joined with backslash.
func NewString(tag Tag, vr VR, values ...string) *Element {
	s := strings.Join(values, `\`)
	return &Element{Tag: tag, VR: vr, Raw: padEven([]byte(s), vr.padByte())}
}

// Uint16 returns the first value decoded as a uint16.
func (e *Element) Uint16() uint16 {
	if len(e.Raw) < 2 {
		return 0
	}
	return binary.LittleEndian.Uint16(e.Raw)
}

// Uint16s returns all values decoded as uint16.
func (e *Element) Uint16s() []uint16 {
	out := make([]uint16, len(e.Raw)/2)
	for i := range out {
		out[i] = binary.LittleEndian.Uint16(e.Raw[2*i:])
	}
	return out
}

// Uint32 returns the first value decoded as a uint32.
func (e *Element) Uint32() uint32 {
	if len(e.Raw) < 4 {
		return 0
	}
	return binary.LittleEndian.Uint32(e.Raw)
}

// String returns the value as text with trailing padding (spaces and NULs)
// removed.
func (e *Element) String() string {
	return strings.TrimRight(string(e.Raw), " \x00")
}

// Strings returns the backslash-separated multi-valued text.
func (e *Element) Strings() []string {
	s := strings.TrimRight(string(e.Raw), " \x00")
	if s == "" {
		return nil
	}
	return strings.Split(s, `\`)
}

// Len returns the encoded value length in bytes (always even).
func (e *Element) Len() int { return len(e.Raw) }

func padEven(b []byte, pad byte) []byte {
	if len(b)%2 == 0 {
		return b
	}
	return append(b, pad)
}

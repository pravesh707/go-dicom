// SPDX-License-Identifier: Apache-2.0

package dicom

import (
	"bytes"
	"encoding/binary"
	"fmt"
)

const undefinedLength = 0xFFFFFFFF

// Codec encodes and decodes data sets for one transfer syntax. It is the
// Strategy abstraction over the wire encodings: callers select a concrete
// codec by transfer syntax UID and depend only on this interface, so new
// transfer syntaxes (Deflated, Big Endian, …) can be added without touching
// existing code (Open/Closed).
type Codec interface {
	// TransferSyntaxUID is the UID this codec implements.
	TransferSyntaxUID() string
	// Implicit reports whether VRs are implicit (looked up from the dictionary)
	// rather than encoded in the stream.
	Implicit() bool
	// Encode serializes a data set in ascending tag order.
	Encode(ds *DataSet) ([]byte, error)
	// Decode parses a data set from bytes.
	Decode(data []byte) (*DataSet, error)
}

// codecRegistry maps transfer syntax UID to its Codec.
var codecRegistry = map[string]Codec{}

// RegisterCodec adds or replaces the codec for a transfer syntax. Intended for
// package init; not safe for concurrent use with lookups.
func RegisterCodec(c Codec) { codecRegistry[c.TransferSyntaxUID()] = c }

// CodecFor returns the registered codec for a transfer syntax UID.
func CodecFor(transferSyntax string) (Codec, bool) {
	c, ok := codecRegistry[transferSyntax]
	return c, ok
}

func init() {
	RegisterCodec(littleEndianCodec{uid: ImplicitVRLittleEndian, implicit: true})
	RegisterCodec(littleEndianCodec{uid: ExplicitVRLittleEndian, implicit: false})
}

// littleEndianCodec implements Codec for both Implicit and Explicit VR Little
// Endian; the only difference is whether VRs are written, so a single type
// (parameterised by the implicit flag) serves both — keeping the byte-level
// logic DRY.
type littleEndianCodec struct {
	uid      string
	implicit bool
}

func (c littleEndianCodec) TransferSyntaxUID() string { return c.uid }
func (c littleEndianCodec) Implicit() bool            { return c.implicit }

func (c littleEndianCodec) Encode(ds *DataSet) ([]byte, error) {
	return encodeElements(ds.Elements(), c.implicit)
}

func (c littleEndianCodec) Decode(data []byte) (*DataSet, error) {
	cur := &cursor{data: data}
	ds := NewDataSet()
	if err := parseDataSet(cur, c.implicit, ds, false); err != nil {
		return nil, err
	}
	return ds, nil
}

// Encode serializes a data set using the codec registered for the given
// transfer syntax UID. It is a thin convenience over CodecFor.
func Encode(ds *DataSet, transferSyntax string) ([]byte, error) {
	c, ok := CodecFor(transferSyntax)
	if !ok {
		return nil, fmt.Errorf("dicom: no codec registered for transfer syntax %s", transferSyntax)
	}
	return c.Encode(ds)
}

// Decode parses a data set using the codec registered for the transfer syntax.
func Decode(data []byte, transferSyntax string) (*DataSet, error) {
	c, ok := CodecFor(transferSyntax)
	if !ok {
		return nil, fmt.Errorf("dicom: no codec registered for transfer syntax %s", transferSyntax)
	}
	return c.Decode(data)
}

// EncodeCommandSet encodes a DIMSE command set. Command sets are always encoded
// in Implicit VR Little Endian, and the (0000,0000) Command Group Length is
// (re)computed automatically.
func EncodeCommandSet(ds *DataSet) ([]byte, error) {
	ds.Remove(TagCommandGroupLength)
	body, err := encodeElements(ds.Elements(), true)
	if err != nil {
		return nil, err
	}
	ds.Set(NewUL(TagCommandGroupLength, uint32(len(body))))
	return encodeElements(ds.Elements(), true)
}

// ---- encoding internals (shared by all little-endian codecs) ----

func encodeElements(elements []*Element, implicit bool) ([]byte, error) {
	var buf bytes.Buffer
	for _, e := range elements {
		if err := encodeElement(&buf, e, implicit); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

func encodeElement(buf *bytes.Buffer, e *Element, implicit bool) error {
	var hdr [4]byte
	binary.LittleEndian.PutUint16(hdr[0:], e.Tag.Group)
	binary.LittleEndian.PutUint16(hdr[2:], e.Tag.Element)
	buf.Write(hdr[:])

	length := uint32(len(e.Raw))
	if e.undefined {
		length = undefinedLength
	}
	if implicit {
		var l [4]byte
		binary.LittleEndian.PutUint32(l[:], length)
		buf.Write(l[:])
		buf.Write(e.Raw)
		return nil
	}

	// Explicit VR.
	vr := e.VR
	if len(vr) != 2 {
		vr = VRUN
	}
	buf.WriteString(string(vr))
	if vr.usesLongLength() {
		buf.Write([]byte{0x00, 0x00}) // reserved
		var l [4]byte
		binary.LittleEndian.PutUint32(l[:], length)
		buf.Write(l[:])
	} else {
		if length > 0xFFFF {
			return fmt.Errorf("dicom: value too long for explicit VR %s short length: %d", vr, length)
		}
		var l [2]byte
		binary.LittleEndian.PutUint16(l[:], uint16(length))
		buf.Write(l[:])
	}
	buf.Write(e.Raw)
	return nil
}

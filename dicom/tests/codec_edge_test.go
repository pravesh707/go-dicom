// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"bytes"
	"encoding/binary"
	"testing"

	"github.com/pravesh707/go-dicom/dicom"
)

func le16(v uint16) []byte { b := make([]byte, 2); binary.LittleEndian.PutUint16(b, v); return b }
func le32(v uint32) []byte { b := make([]byte, 4); binary.LittleEndian.PutUint32(b, v); return b }

func TestEmptyDataSetRoundTrip(t *testing.T) {
	ds := dicom.NewDataSet()
	for _, ts := range []string{dicom.ImplicitVRLittleEndian, dicom.ExplicitVRLittleEndian} {
		b, err := dicom.Encode(ds, ts)
		if err != nil {
			t.Fatalf("encode empty: %v", err)
		}
		if len(b) != 0 {
			t.Errorf("empty dataset encoded to %d bytes", len(b))
		}
		got, err := dicom.Decode(b, ts)
		if err != nil || got.Len() != 0 {
			t.Errorf("decode empty: len=%d err=%v", got.Len(), err)
		}
	}
}

func TestExplicitLongLengthVRRoundTrip(t *testing.T) {
	ds := dicom.NewDataSet()
	ds.Set(dicom.NewElement(dicom.NewTag(0x7FE0, 0x0010), dicom.VROW, []byte{0x01, 0x02, 0x03, 0x04}))
	ds.Set(dicom.NewElement(dicom.NewTag(0x0009, 0x0001), dicom.VRUN, []byte{0xAA, 0xBB}))

	b, err := dicom.Encode(ds, dicom.ExplicitVRLittleEndian)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	got, err := dicom.Decode(b, dicom.ExplicitVRLittleEndian)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	e, ok := got.Get(dicom.NewTag(0x7FE0, 0x0010))
	if !ok || e.VR != dicom.VROW || !bytes.Equal(e.Raw, []byte{1, 2, 3, 4}) {
		t.Errorf("OW element = %+v", e)
	}
}

func TestExplicitShortLengthOverflowErrors(t *testing.T) {
	ds := dicom.NewDataSet()
	// PN uses a 2-byte length in Explicit VR; a value > 0xFFFF cannot be encoded.
	ds.Set(dicom.NewElement(dicom.NewTag(0x0010, 0x0010), dicom.VRPN, make([]byte, 0x10000)))
	if _, err := dicom.Encode(ds, dicom.ExplicitVRLittleEndian); err == nil {
		t.Error("expected overflow error for explicit short-length VR")
	}
	// Implicit VR uses a 4-byte length, so the same value is fine.
	if _, err := dicom.Encode(ds, dicom.ImplicitVRLittleEndian); err != nil {
		t.Errorf("implicit encode should succeed: %v", err)
	}
}

func TestUndefinedLengthSequenceRoundTrip(t *testing.T) {
	var s []byte
	s = append(s, le16(0x0008)...) // (0008,1140) sequence
	s = append(s, le16(0x1140)...)
	s = append(s, le32(0xFFFFFFFF)...) // undefined length
	s = append(s, le16(0xFFFE)...)     // Item
	s = append(s, le16(0xE000)...)
	s = append(s, le32(4)...)
	s = append(s, 0x01, 0x02, 0x03, 0x04) // opaque item content
	s = append(s, le16(0xFFFE)...)        // Sequence Delimitation Item
	s = append(s, le16(0xE0DD)...)
	s = append(s, le32(0)...)

	ds, err := dicom.Decode(s, dicom.ImplicitVRLittleEndian)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	e, ok := ds.Get(dicom.NewTag(0x0008, 0x1140))
	if !ok {
		t.Fatal("sequence element missing")
	}
	if !e.UndefinedLength() {
		t.Error("element should be marked undefined-length")
	}
	reenc, err := dicom.Encode(ds, dicom.ImplicitVRLittleEndian)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	if !bytes.Equal(reenc, s) {
		t.Errorf("undefined-length round trip mismatch:\n got %x\nwant %x", reenc, s)
	}
}

func TestUndefinedLengthNestedItemRoundTrip(t *testing.T) {
	var s []byte
	s = append(s, le16(0x0008)...) // sequence, undefined length
	s = append(s, le16(0x1140)...)
	s = append(s, le32(0xFFFFFFFF)...)
	s = append(s, le16(0xFFFE)...) // Item, undefined length
	s = append(s, le16(0xE000)...)
	s = append(s, le32(0xFFFFFFFF)...)
	s = append(s, le16(0x0010)...) // sub-element (0010,0020) LO "AB"
	s = append(s, le16(0x0020)...)
	s = append(s, le32(2)...)
	s = append(s, 0x41, 0x42)
	s = append(s, le16(0xFFFE)...) // Item Delimitation Item
	s = append(s, le16(0xE00D)...)
	s = append(s, le32(0)...)
	s = append(s, le16(0xFFFE)...) // Sequence Delimitation Item
	s = append(s, le16(0xE0DD)...)
	s = append(s, le32(0)...)

	ds, err := dicom.Decode(s, dicom.ImplicitVRLittleEndian)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	reenc, err := dicom.Encode(ds, dicom.ImplicitVRLittleEndian)
	if err != nil {
		t.Fatalf("re-encode: %v", err)
	}
	if !bytes.Equal(reenc, s) {
		t.Errorf("nested undefined-length round trip mismatch")
	}
}

func TestDecodeTruncatedErrors(t *testing.T) {
	cases := [][]byte{
		{0x08, 0x00, 0x18, 0x00, 0x02, 0x00},                                           // tag ok, length truncated
		{0x08, 0x00, 0x18, 0x00, 0x0A, 0x00, 0x00, 0x00},                               // length 10, no value
		append(append(le16(0x0008), le16(0x0018)...), append(le32(10), 0x41, 0x42)...), // value short
	}
	for i, c := range cases {
		if _, err := dicom.Decode(c, dicom.ImplicitVRLittleEndian); err == nil {
			t.Errorf("case %d: expected truncation error", i)
		}
	}
}

func TestUnknownTransferSyntaxErrors(t *testing.T) {
	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUS(dicom.NewTag(0x0028, 0x0010), 1))
	if _, err := dicom.Encode(ds, "1.2.840.10008.1.2.99.bogus"); err == nil {
		t.Error("encode with unknown TS should error")
	}
	if _, err := dicom.Decode([]byte{0x01}, "1.2.840.10008.1.2.99.bogus"); err == nil {
		t.Error("decode with unknown TS should error")
	}
}

// fakeCodec is a no-op Codec used to test the registry.
type fakeCodec struct{ uid string }

func (f fakeCodec) TransferSyntaxUID() string             { return f.uid }
func (f fakeCodec) Implicit() bool                        { return false }
func (f fakeCodec) Encode(*dicom.DataSet) ([]byte, error) { return []byte{0xEE}, nil }
func (f fakeCodec) Decode([]byte) (*dicom.DataSet, error) { return dicom.NewDataSet(), nil }

func TestCodecRegistry(t *testing.T) {
	c, ok := dicom.CodecFor(dicom.ImplicitVRLittleEndian)
	if !ok || !c.Implicit() {
		t.Error("implicit codec lookup failed")
	}
	c, ok = dicom.CodecFor(dicom.ExplicitVRLittleEndian)
	if !ok || c.Implicit() {
		t.Error("explicit codec lookup failed")
	}
	if _, ok := dicom.CodecFor("totally-unregistered"); ok {
		t.Error("unregistered TS should not resolve")
	}

	dicom.RegisterCodec(fakeCodec{uid: "1.2.3.fake"})
	out, err := dicom.Encode(dicom.NewDataSet(), "1.2.3.fake")
	if err != nil || !bytes.Equal(out, []byte{0xEE}) {
		t.Errorf("fake codec not used: out=%x err=%v", out, err)
	}
}

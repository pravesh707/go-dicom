// SPDX-License-Identifier: Apache-2.0

package dicom

import (
	"bytes"
	"testing"
)

func TestCommandSetRoundTrip(t *testing.T) {
	ds := NewDataSet()
	ds.Set(NewUI(TagAffectedSOPClassUID, VerificationSOPClass))
	ds.Set(NewUS(TagCommandField, 0x0030)) // C-ECHO-RQ
	ds.Set(NewUS(TagMessageID, 7))
	ds.Set(NewUS(TagCommandDataSetType, 0x0101))

	encoded, err := EncodeCommandSet(ds)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	// Command Group Length must now be present and first.
	gl, ok := ds.GetUint32(TagCommandGroupLength)
	if !ok {
		t.Fatal("command group length not set")
	}
	// The group length counts every byte after the group-length element.
	if int(gl) != len(encoded)-12 { // (0000,0000) UL header(8)+value(4) = 12 bytes
		t.Errorf("group length = %d, want %d", gl, len(encoded)-12)
	}

	got, err := Decode(encoded, ImplicitVRLittleEndian)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if cf, _ := got.GetUint16(TagCommandField); cf != 0x0030 {
		t.Errorf("command field = %#x, want 0x0030", cf)
	}
	if mid, _ := got.GetUint16(TagMessageID); mid != 7 {
		t.Errorf("message id = %d, want 7", mid)
	}
	if sop, _ := got.GetString(TagAffectedSOPClassUID); sop != VerificationSOPClass {
		t.Errorf("sop class = %q, want %q", sop, VerificationSOPClass)
	}
}

func TestExplicitVRRoundTrip(t *testing.T) {
	ds := NewDataSet()
	ds.Set(NewString(Tag{0x0010, 0x0010}, VRPN, "Doe^Jane"))
	ds.Set(NewUI(Tag{0x0008, 0x0018}, "1.2.3.4.5"))
	ds.Set(NewUS(Tag{0x0028, 0x0010}, 512))

	for _, ts := range []string{ImplicitVRLittleEndian, ExplicitVRLittleEndian} {
		encoded, err := Encode(ds, ts)
		if err != nil {
			t.Fatalf("encode %s: %v", ts, err)
		}
		got, err := Decode(encoded, ts)
		if err != nil {
			t.Fatalf("decode %s: %v", ts, err)
		}
		if name, _ := got.GetString(Tag{0x0010, 0x0010}); name != "Doe^Jane" {
			t.Errorf("%s: patient name = %q", ts, name)
		}
		if uid, _ := got.GetString(Tag{0x0008, 0x0018}); uid != "1.2.3.4.5" {
			t.Errorf("%s: uid = %q", ts, uid)
		}
		if rows, _ := got.GetUint16(Tag{0x0028, 0x0010}); rows != 512 {
			t.Errorf("%s: rows = %d", ts, rows)
		}
	}
}

func TestUIDPaddingEven(t *testing.T) {
	e := NewUI(Tag{0x0008, 0x0018}, "1.2.3") // odd length 5 -> padded to 6 with NUL
	if len(e.Raw)%2 != 0 {
		t.Fatalf("UI not padded to even length: %d", len(e.Raw))
	}
	if e.Raw[len(e.Raw)-1] != 0x00 {
		t.Errorf("UI pad byte = %#x, want 0x00", e.Raw[len(e.Raw)-1])
	}
	if !bytes.Equal([]byte(e.String()), []byte("1.2.3")) {
		t.Errorf("String() = %q, want trimmed", e.String())
	}
}

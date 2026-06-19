// SPDX-License-Identifier: Apache-2.0

package dicom

import (
	"path/filepath"
	"testing"
)

func sampleDataSet() *DataSet {
	ds := NewDataSet()
	ds.Set(NewUI(Tag{0x0008, 0x0016}, "1.2.840.10008.5.1.4.1.1.7")) // SC Image Storage
	ds.Set(NewUI(Tag{0x0008, 0x0018}, "1.2.3.4.5.6.7.8.9"))         // SOP Instance UID
	ds.Set(NewString(Tag{0x0010, 0x0010}, VRPN, "Doe^Jane"))
	ds.Set(NewString(Tag{0x0010, 0x0020}, VRLO, "PID-123"))
	ds.Set(NewUS(Tag{0x0028, 0x0010}, 4)) // Rows
	ds.Set(NewUS(Tag{0x0028, 0x0011}, 4)) // Columns
	ds.Set(NewElement(Tag{0x7FE0, 0x0010}, VROW, []byte{1, 2, 3, 4, 5, 6, 7, 8}))
	return ds
}

func TestFileRoundTrip(t *testing.T) {
	for _, ts := range []string{ImplicitVRLittleEndian, ExplicitVRLittleEndian} {
		f := NewFile(sampleDataSet(), ts)
		path := filepath.Join(t.TempDir(), "test.dcm")
		if err := f.WriteFile(path); err != nil {
			t.Fatalf("%s write: %v", ts, err)
		}

		got, err := ReadFile(path)
		if err != nil {
			t.Fatalf("%s read: %v", ts, err)
		}
		if got.TransferSyntax() != ts {
			t.Errorf("transfer syntax = %q, want %q", got.TransferSyntax(), ts)
		}
		if got.SOPClassUID() != "1.2.840.10008.5.1.4.1.1.7" {
			t.Errorf("SOP class = %q", got.SOPClassUID())
		}
		if got.SOPInstanceUID() != "1.2.3.4.5.6.7.8.9" {
			t.Errorf("SOP instance = %q", got.SOPInstanceUID())
		}
		if name, _ := got.DataSet.GetString(Tag{0x0010, 0x0010}); name != "Doe^Jane" {
			t.Errorf("patient name = %q", name)
		}
		if rows, _ := got.DataSet.GetUint16(Tag{0x0028, 0x0010}); rows != 4 {
			t.Errorf("rows = %d", rows)
		}
		if px, ok := got.DataSet.Get(Tag{0x7FE0, 0x0010}); !ok || len(px.Raw) != 8 {
			t.Errorf("pixel data = %+v", px)
		}
	}
}

func TestParseRejectsNonDICOM(t *testing.T) {
	if _, err := Parse([]byte("not a dicom file at all, definitely")); err == nil {
		t.Error("expected error for non-DICOM input")
	}
	if _, err := Parse(make([]byte, 10)); err == nil {
		t.Error("expected error for short input")
	}
}

func TestFileMetaGroupLength(t *testing.T) {
	f := NewFile(sampleDataSet(), ExplicitVRLittleEndian)
	if _, err := encodeFileMeta(f.Meta); err != nil {
		t.Fatal(err)
	}
	gl, ok := f.Meta.GetUint32(TagFileMetaInformationGroupLength)
	if !ok || gl == 0 {
		t.Errorf("group length = %d, ok=%v", gl, ok)
	}
}

// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"path/filepath"
	"testing"

	"github.com/pravesh707/go-dicom/dicom"
)

func sampleDataSet() *dicom.DataSet {
	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUI(dicom.NewTag(0x0008, 0x0016), "1.2.840.10008.5.1.4.1.1.7")) // SC Image Storage
	ds.Set(dicom.NewUI(dicom.NewTag(0x0008, 0x0018), "1.2.3.4.5.6.7.8.9"))         // SOP Instance UID
	ds.Set(dicom.NewString(dicom.NewTag(0x0010, 0x0010), dicom.VRPN, "Doe^Jane"))
	ds.Set(dicom.NewString(dicom.NewTag(0x0010, 0x0020), dicom.VRLO, "PID-123"))
	ds.Set(dicom.NewUS(dicom.NewTag(0x0028, 0x0010), 4)) // Rows
	ds.Set(dicom.NewUS(dicom.NewTag(0x0028, 0x0011), 4)) // Columns
	ds.Set(dicom.NewElement(dicom.NewTag(0x7FE0, 0x0010), dicom.VROW, []byte{1, 2, 3, 4, 5, 6, 7, 8}))
	return ds
}

func TestFileRoundTrip(t *testing.T) {
	for _, ts := range []string{dicom.ImplicitVRLittleEndian, dicom.ExplicitVRLittleEndian} {
		f := dicom.NewFile(sampleDataSet(), ts)
		path := filepath.Join(t.TempDir(), "test.dcm")
		if err := f.WriteFile(path); err != nil {
			t.Fatalf("%s write: %v", ts, err)
		}

		got, err := dicom.ReadFile(path)
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
		if name, _ := got.DataSet.GetString(dicom.NewTag(0x0010, 0x0010)); name != "Doe^Jane" {
			t.Errorf("patient name = %q", name)
		}
		if rows, _ := got.DataSet.GetUint16(dicom.NewTag(0x0028, 0x0010)); rows != 4 {
			t.Errorf("rows = %d", rows)
		}
		if px, ok := got.DataSet.Get(dicom.NewTag(0x7FE0, 0x0010)); !ok || len(px.Raw) != 8 {
			t.Errorf("pixel data = %+v", px)
		}
	}
}

func TestParseRejectsNonDICOM(t *testing.T) {
	if _, err := dicom.Parse([]byte("not a dicom file at all, definitely")); err == nil {
		t.Error("expected error for non-DICOM input")
	}
	if _, err := dicom.Parse(make([]byte, 10)); err == nil {
		t.Error("expected error for short input")
	}
}

// TestFileMetaGroupLength confirms the File Meta Information Group Length
// (0002,0000) is computed on write and present on read-back — exercised through
// the public WriteFile/ReadFile round trip.
func TestFileMetaGroupLength(t *testing.T) {
	f := dicom.NewFile(sampleDataSet(), dicom.ExplicitVRLittleEndian)
	path := filepath.Join(t.TempDir(), "meta.dcm")
	if err := f.WriteFile(path); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := dicom.ReadFile(path)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	gl, ok := got.Meta.GetUint32(dicom.TagFileMetaInformationGroupLength)
	if !ok || gl == 0 {
		t.Errorf("group length = %d, ok=%v", gl, ok)
	}
}

// SPDX-License-Identifier: Apache-2.0

package dicom

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"io"
	"os"
)

// File-meta (group 0002) tags, encoded in Explicit VR Little Endian per PS3.10.
var (
	TagFileMetaInformationGroupLength = Tag{0x0002, 0x0000}
	TagFileMetaInformationVersion     = Tag{0x0002, 0x0001}
	TagMediaStorageSOPClassUID        = Tag{0x0002, 0x0002}
	TagMediaStorageSOPInstanceUID     = Tag{0x0002, 0x0003}
	TagTransferSyntaxUID              = Tag{0x0002, 0x0010}
	TagImplementationClassUID         = Tag{0x0002, 0x0012}
	TagImplementationVersionName      = Tag{0x0002, 0x0013}
)

// dicmMagic marks a Part-10 file at offset 128.
var dicmMagic = []byte("DICM")

// File is a DICOM Part-10 file: a File Meta Information group plus the main
// data set encoded in the transfer syntax named by the meta.
type File struct {
	Meta    *DataSet
	DataSet *DataSet
}

// NewFile builds a File from a data set, deriving the File Meta Information from
// the data set's SOP Class/Instance UIDs and the given transfer syntax.
func NewFile(ds *DataSet, transferSyntax string) *File {
	meta := NewDataSet()
	meta.Set(NewElement(TagFileMetaInformationVersion, VROB, []byte{0x00, 0x01}))
	if sop, ok := ds.GetString(Tag{0x0008, 0x0016}); ok {
		meta.Set(NewUI(TagMediaStorageSOPClassUID, sop))
	}
	if inst, ok := ds.GetString(Tag{0x0008, 0x0018}); ok {
		meta.Set(NewUI(TagMediaStorageSOPInstanceUID, inst))
	}
	meta.Set(NewUI(TagTransferSyntaxUID, transferSyntax))
	meta.Set(NewUI(TagImplementationClassUID, GoDICOMImplementationClassUID))
	meta.Set(NewString(TagImplementationVersionName, VRSH, GoDICOMImplementationVersionName))
	return &File{Meta: meta, DataSet: ds}
}

// TransferSyntax returns the transfer syntax UID from the file meta, defaulting
// to Explicit VR Little Endian if absent.
func (f *File) TransferSyntax() string {
	if ts, ok := f.Meta.GetString(TagTransferSyntaxUID); ok && ts != "" {
		return ts
	}
	return ExplicitVRLittleEndian
}

// SOPClassUID returns the SOP Class UID, preferring the data set's (0008,0016).
func (f *File) SOPClassUID() string {
	if v, ok := f.DataSet.GetString(Tag{0x0008, 0x0016}); ok {
		return v
	}
	v, _ := f.Meta.GetString(TagMediaStorageSOPClassUID)
	return v
}

// SOPInstanceUID returns the SOP Instance UID, preferring the data set's
// (0008,0018).
func (f *File) SOPInstanceUID() string {
	if v, ok := f.DataSet.GetString(Tag{0x0008, 0x0018}); ok {
		return v
	}
	v, _ := f.Meta.GetString(TagMediaStorageSOPInstanceUID)
	return v
}

// WriteTo encodes the file (preamble, DICM, meta group, data set) to w.
func (f *File) WriteTo(w io.Writer) (int64, error) {
	var buf bytes.Buffer
	buf.Write(make([]byte, 128)) // preamble
	buf.Write(dicmMagic)

	metaBytes, err := encodeFileMeta(f.Meta)
	if err != nil {
		return 0, err
	}
	buf.Write(metaBytes)

	body, err := Encode(f.DataSet, f.TransferSyntax())
	if err != nil {
		return 0, err
	}
	buf.Write(body)

	n, err := w.Write(buf.Bytes())
	return int64(n), err
}

// WriteFile writes the file to path.
func (f *File) WriteFile(path string) error {
	out, err := os.Create(path)
	if err != nil {
		return err
	}
	defer out.Close()
	if _, err := f.WriteTo(out); err != nil {
		return err
	}
	return out.Close()
}

// encodeFileMeta encodes the group-0002 meta in Explicit VR Little Endian with a
// correct (0002,0000) group length.
func encodeFileMeta(meta *DataSet) ([]byte, error) {
	meta.Remove(TagFileMetaInformationGroupLength)
	body, err := Encode(meta, ExplicitVRLittleEndian)
	if err != nil {
		return nil, err
	}
	meta.Set(NewUL(TagFileMetaInformationGroupLength, uint32(len(body))))
	return Encode(meta, ExplicitVRLittleEndian)
}

// ReadFile reads and parses a Part-10 file from path.
func ReadFile(path string) (*File, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}

// Parse parses an in-memory Part-10 file (preamble, DICM, meta, data set).
func Parse(data []byte) (*File, error) {
	if len(data) < 132 || !bytes.Equal(data[128:132], dicmMagic) {
		return nil, fmt.Errorf("dicom: not a Part-10 file (missing DICM magic)")
	}
	pos := 132

	// The first meta element is (0002,0000) UL Group Length in Explicit VR LE:
	// tag(4) + "UL"(2) + length(2) + value(4) = 12 bytes.
	if len(data) < pos+12 {
		return nil, fmt.Errorf("dicom: truncated file meta group length")
	}
	if !(data[pos] == 0x02 && data[pos+1] == 0x00 && data[pos+2] == 0x00 && data[pos+3] == 0x00) {
		return nil, fmt.Errorf("dicom: file does not start with (0002,0000) group length")
	}
	groupLen := int(binary.LittleEndian.Uint32(data[pos+8 : pos+12]))
	metaEnd := pos + 12 + groupLen
	if metaEnd > len(data) {
		return nil, fmt.Errorf("dicom: file meta group length %d exceeds file", groupLen)
	}

	meta, err := Decode(data[pos:metaEnd], ExplicitVRLittleEndian)
	if err != nil {
		return nil, fmt.Errorf("dicom: parsing file meta: %w", err)
	}

	ts, _ := meta.GetString(TagTransferSyntaxUID)
	if _, ok := CodecFor(ts); !ok {
		return nil, fmt.Errorf("dicom: unsupported transfer syntax %q", ts)
	}
	ds, err := Decode(data[metaEnd:], ts)
	if err != nil {
		return nil, fmt.Errorf("dicom: parsing data set: %w", err)
	}
	return &File{Meta: meta, DataSet: ds}, nil
}

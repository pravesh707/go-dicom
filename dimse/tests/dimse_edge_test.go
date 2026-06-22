// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"testing"

	"github.com/pravesh707/go-dicom/dicom"
	"github.com/pravesh707/go-dicom/dimse"
)

func TestCommandFieldString(t *testing.T) {
	cases := map[dimse.CommandField]string{
		dimse.CEchoRQ:    "C-ECHO-RQ",
		dimse.CEchoRSP:   "C-ECHO-RSP",
		dimse.CStoreRQ:   "C-STORE-RQ",
		dimse.CFindRQ:    "C-FIND-RQ",
		dimse.CMoveRQ:    "C-MOVE-RQ",
		dimse.CGetRQ:     "C-GET-RQ",
		dimse.CCancelRQ:  "C-CANCEL-RQ",
		dimse.NCreateRQ:  "N-CREATE-RQ",
		dimse.NDeleteRSP: "N-DELETE-RSP",
		0xABCD:           "UNKNOWN",
	}
	for cf, want := range cases {
		if got := cf.String(); got != want {
			t.Errorf("%#x.String() = %q, want %q", uint16(cf), got, want)
		}
	}
}

func TestResponseCodeIsRequestOr0x8000(t *testing.T) {
	pairs := [][2]dimse.CommandField{
		{dimse.CEchoRQ, dimse.CEchoRSP}, {dimse.CStoreRQ, dimse.CStoreRSP}, {dimse.CFindRQ, dimse.CFindRSP},
		{dimse.CGetRQ, dimse.CGetRSP}, {dimse.CMoveRQ, dimse.CMoveRSP}, {dimse.NGetRQ, dimse.NGetRSP},
	}
	for _, p := range pairs {
		if p[0]|0x8000 != p[1] {
			t.Errorf("%s response code mismatch: %#x|0x8000 != %#x", p[0], uint16(p[0]), uint16(p[1]))
		}
	}
}

func TestPriorityValues(t *testing.T) {
	if dimse.PriorityMedium != 0x0000 || dimse.PriorityHigh != 0x0001 || dimse.PriorityLow != 0x0002 {
		t.Errorf("priority values wrong: %d %d %d", dimse.PriorityMedium, dimse.PriorityHigh, dimse.PriorityLow)
	}
}

func TestStatusCategoryBoundaries(t *testing.T) {
	cases := map[dimse.Status]dimse.Category{
		0x0000: dimse.CategorySuccess,
		0xFF00: dimse.CategoryPending,
		0xFF01: dimse.CategoryPending,
		0xFE00: dimse.CategoryCancel,
		0x0001: dimse.CategoryWarning,
		0xB000: dimse.CategoryWarning,
		0xB006: dimse.CategoryWarning,
		0xB007: dimse.CategoryWarning,
		0xA700: dimse.CategoryFailure,
		0x0122: dimse.CategoryFailure,
		0xC000: dimse.CategoryFailure,
		0x0110: dimse.CategoryFailure,
	}
	for s, want := range cases {
		if got := s.Category(); got != want {
			t.Errorf("status %#04x category = %v, want %v", uint16(s), got, want)
		}
	}
}

func TestStatusHelpers(t *testing.T) {
	if !dimse.StatusSuccess.IsSuccess() || dimse.StatusSuccess.IsPending() {
		t.Error("success helper wrong")
	}
	if !dimse.StatusPending.IsPending() || dimse.StatusPending.IsSuccess() {
		t.Error("pending helper wrong")
	}
	if dimse.StatusProcessingFailure.IsSuccess() {
		t.Error("failure should not be success")
	}
}

func TestCategoryString(t *testing.T) {
	cases := map[dimse.Category]string{
		dimse.CategorySuccess: "Success", dimse.CategoryPending: "Pending", dimse.CategoryCancel: "Cancel",
		dimse.CategoryWarning: "Warning", dimse.CategoryFailure: "Failure",
	}
	for c, want := range cases {
		if c.String() != want {
			t.Errorf("Category %d = %q, want %q", c, c.String(), want)
		}
	}
}

func TestCEchoDefaultSOPClass(t *testing.T) {
	rq := &dimse.CEchoRequest{MessageID: 1} // no AffectedSOPClassUID
	b, _ := dimse.EncodeCommand(rq)
	msg, _ := dimse.DecodeCommand(b)
	got := msg.(*dimse.CEchoRequest)
	if got.AffectedSOPClassUID != dicom.VerificationSOPClass {
		t.Errorf("default SOP class = %q", got.AffectedSOPClassUID)
	}
}

func TestCEchoResponseCustomSOPClass(t *testing.T) {
	rsp := &dimse.CEchoResponse{MessageIDBeingRespondedTo: 9, AffectedSOPClassUID: "1.2.3", Status: 0xC000}
	b, _ := dimse.EncodeCommand(rsp)
	msg, _ := dimse.DecodeCommand(b)
	got := msg.(*dimse.CEchoResponse)
	if got.AffectedSOPClassUID != "1.2.3" || got.Status != 0xC000 {
		t.Errorf("rsp = %+v", got)
	}
	if got.Status.Category() != dimse.CategoryFailure {
		t.Error("0xC000 should be failure")
	}
}

func TestDecodeUnknownCommandYieldsRawMessage(t *testing.T) {
	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUI(dicom.TagAffectedSOPClassUID, "1.2.3"))
	ds.Set(dicom.NewUS(dicom.TagCommandField, uint16(dimse.CCancelRQ))) // not registered
	ds.Set(dicom.NewUS(dicom.TagCommandDataSetType, 0x0000))            // data set present
	b, _ := dicom.EncodeCommandSet(ds)

	msg, err := dimse.DecodeCommand(b)
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := msg.(*dimse.RawMessage)
	if !ok {
		t.Fatalf("expected RawMessage, got %T", msg)
	}
	if raw.CommandField() != dimse.CCancelRQ {
		t.Errorf("raw command = %s", raw.CommandField())
	}
	if !raw.HasDataSet() {
		t.Error("raw message should report data set present")
	}
}

func TestDecodeMissingCommandFieldErrors(t *testing.T) {
	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUS(dicom.TagMessageID, 1)) // no (0000,0100)
	b, _ := dicom.EncodeCommandSet(ds)
	if _, err := dimse.DecodeCommand(b); err == nil {
		t.Error("missing Command Field should error")
	}
}

// TestHasDataSetLogic exercises the Command Data Set Type → HasDataSet mapping
// through the public RawMessage.HasDataSet(): an unregistered command field
// decodes to a *RawMessage whose HasDataSet reflects (0000,0800).
func TestHasDataSetLogic(t *testing.T) {
	decode := func(setType bool, dst uint16) dimse.Message {
		ds := dicom.NewDataSet()
		ds.Set(dicom.NewUI(dicom.TagAffectedSOPClassUID, "1.2.3"))
		ds.Set(dicom.NewUS(dicom.TagCommandField, uint16(dimse.CCancelRQ))) // unregistered → RawMessage
		if setType {
			ds.Set(dicom.NewUS(dicom.TagCommandDataSetType, dst))
		}
		b, _ := dicom.EncodeCommandSet(ds)
		msg, err := dimse.DecodeCommand(b)
		if err != nil {
			t.Fatal(err)
		}
		return msg
	}
	if decode(true, 0x0101).HasDataSet() {
		t.Error("0x0101 means no data set")
	}
	if !decode(true, 0x0000).HasDataSet() {
		t.Error("0x0000 means data set present")
	}
	if decode(false, 0).HasDataSet() {
		t.Error("missing element means no data set")
	}
}

// TestMessageRegistry confirms a built-in command field resolves to its typed
// message and that a custom RegisterMessage parser is consulted by
// DecodeCommand — both observable through the public API.
func TestMessageRegistry(t *testing.T) {
	rqBytes, _ := dimse.EncodeCommand(&dimse.CEchoRequest{MessageID: 1})
	if msg, err := dimse.DecodeCommand(rqBytes); err != nil {
		t.Fatal(err)
	} else if _, ok := msg.(*dimse.CEchoRequest); !ok {
		t.Errorf("C-ECHO-RQ decoded to %T, want *dimse.CEchoRequest (registered)", msg)
	}

	const custom dimse.CommandField = 0x7777
	dimse.RegisterMessage(custom, func(ds *dicom.DataSet) dimse.Message {
		return &dimse.CEchoResponse{MessageIDBeingRespondedTo: 0xABCD}
	})
	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUI(dicom.TagAffectedSOPClassUID, "1.2.3"))
	ds.Set(dicom.NewUS(dicom.TagCommandField, uint16(custom)))
	ds.Set(dicom.NewUS(dicom.TagCommandDataSetType, 0x0101))
	b, _ := dicom.EncodeCommandSet(ds)
	msg, err := dimse.DecodeCommand(b)
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := msg.(*dimse.CEchoResponse); !ok || got.MessageIDBeingRespondedTo != 0xABCD {
		t.Errorf("custom parser not consulted: got %T %+v", msg, msg)
	}
}

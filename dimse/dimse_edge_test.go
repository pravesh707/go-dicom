// SPDX-License-Identifier: Apache-2.0

package dimse

import (
	"testing"

	"github.com/pravesh707/go-dicom/dicom"
)

func TestCommandFieldString(t *testing.T) {
	cases := map[CommandField]string{
		CEchoRQ:    "C-ECHO-RQ",
		CEchoRSP:   "C-ECHO-RSP",
		CStoreRQ:   "C-STORE-RQ",
		CFindRQ:    "C-FIND-RQ",
		CMoveRQ:    "C-MOVE-RQ",
		CGetRQ:     "C-GET-RQ",
		CCancelRQ:  "C-CANCEL-RQ",
		NCreateRQ:  "N-CREATE-RQ",
		NDeleteRSP: "N-DELETE-RSP",
		0xABCD:     "UNKNOWN",
	}
	for cf, want := range cases {
		if got := cf.String(); got != want {
			t.Errorf("%#x.String() = %q, want %q", uint16(cf), got, want)
		}
	}
}

func TestResponseCodeIsRequestOr0x8000(t *testing.T) {
	pairs := [][2]CommandField{
		{CEchoRQ, CEchoRSP}, {CStoreRQ, CStoreRSP}, {CFindRQ, CFindRSP},
		{CGetRQ, CGetRSP}, {CMoveRQ, CMoveRSP}, {NGetRQ, NGetRSP},
	}
	for _, p := range pairs {
		if p[0]|0x8000 != p[1] {
			t.Errorf("%s response code mismatch: %#x|0x8000 != %#x", p[0], uint16(p[0]), uint16(p[1]))
		}
	}
}

func TestPriorityValues(t *testing.T) {
	if PriorityMedium != 0x0000 || PriorityHigh != 0x0001 || PriorityLow != 0x0002 {
		t.Errorf("priority values wrong: %d %d %d", PriorityMedium, PriorityHigh, PriorityLow)
	}
}

func TestStatusCategoryBoundaries(t *testing.T) {
	cases := map[Status]Category{
		0x0000: CategorySuccess,
		0xFF00: CategoryPending,
		0xFF01: CategoryPending,
		0xFE00: CategoryCancel,
		0x0001: CategoryWarning,
		0xB000: CategoryWarning,
		0xB006: CategoryWarning,
		0xB007: CategoryWarning,
		0xA700: CategoryFailure,
		0x0122: CategoryFailure,
		0xC000: CategoryFailure,
		0x0110: CategoryFailure,
	}
	for s, want := range cases {
		if got := s.Category(); got != want {
			t.Errorf("status %#04x category = %v, want %v", uint16(s), got, want)
		}
	}
}

func TestStatusHelpers(t *testing.T) {
	if !StatusSuccess.IsSuccess() || StatusSuccess.IsPending() {
		t.Error("success helper wrong")
	}
	if !StatusPending.IsPending() || StatusPending.IsSuccess() {
		t.Error("pending helper wrong")
	}
	if StatusProcessingFailure.IsSuccess() {
		t.Error("failure should not be success")
	}
}

func TestCategoryString(t *testing.T) {
	cases := map[Category]string{
		CategorySuccess: "Success", CategoryPending: "Pending", CategoryCancel: "Cancel",
		CategoryWarning: "Warning", CategoryFailure: "Failure",
	}
	for c, want := range cases {
		if c.String() != want {
			t.Errorf("Category %d = %q, want %q", c, c.String(), want)
		}
	}
}

func TestCEchoDefaultSOPClass(t *testing.T) {
	rq := &CEchoRequest{MessageID: 1} // no AffectedSOPClassUID
	b, _ := EncodeCommand(rq)
	msg, _ := DecodeCommand(b)
	got := msg.(*CEchoRequest)
	if got.AffectedSOPClassUID != dicom.VerificationSOPClass {
		t.Errorf("default SOP class = %q", got.AffectedSOPClassUID)
	}
}

func TestCEchoResponseCustomSOPClass(t *testing.T) {
	rsp := &CEchoResponse{MessageIDBeingRespondedTo: 9, AffectedSOPClassUID: "1.2.3", Status: 0xC000}
	b, _ := EncodeCommand(rsp)
	msg, _ := DecodeCommand(b)
	got := msg.(*CEchoResponse)
	if got.AffectedSOPClassUID != "1.2.3" || got.Status != 0xC000 {
		t.Errorf("rsp = %+v", got)
	}
	if got.Status.Category() != CategoryFailure {
		t.Error("0xC000 should be failure")
	}
}

func TestDecodeUnknownCommandYieldsRawMessage(t *testing.T) {
	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUI(dicom.TagAffectedSOPClassUID, "1.2.3"))
	ds.Set(dicom.NewUS(dicom.TagCommandField, uint16(CMoveRQ))) // not registered
	ds.Set(dicom.NewUS(dicom.TagCommandDataSetType, 0x0000))    // data set present
	b, _ := dicom.EncodeCommandSet(ds)

	msg, err := DecodeCommand(b)
	if err != nil {
		t.Fatal(err)
	}
	raw, ok := msg.(*RawMessage)
	if !ok {
		t.Fatalf("expected RawMessage, got %T", msg)
	}
	if raw.CommandField() != CMoveRQ {
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
	if _, err := DecodeCommand(b); err == nil {
		t.Error("missing Command Field should error")
	}
}

func TestHasDataSetLogic(t *testing.T) {
	absent := dicom.NewDataSet()
	absent.Set(dicom.NewUS(dicom.TagCommandDataSetType, dataSetTypeAbsent))
	if hasDataSet(absent) {
		t.Error("0x0101 means no data set")
	}
	present := dicom.NewDataSet()
	present.Set(dicom.NewUS(dicom.TagCommandDataSetType, dataSetTypePresent))
	if !hasDataSet(present) {
		t.Error("0x0000 means data set present")
	}
	missing := dicom.NewDataSet()
	if hasDataSet(missing) {
		t.Error("missing element means no data set")
	}
}

func TestMessageRegistry(t *testing.T) {
	if _, ok := messageRegistry[CEchoRQ]; !ok {
		t.Error("C-ECHO-RQ not registered")
	}
	if _, ok := messageRegistry[CEchoRSP]; !ok {
		t.Error("C-ECHO-RSP not registered")
	}
	// Register a custom parser and confirm it is used.
	RegisterMessage(0x7777, func(ds *dicom.DataSet) Message { return &RawMessage{Command: 0x7777, Set: ds} })
	if _, ok := messageRegistry[0x7777]; !ok {
		t.Error("custom registration failed")
	}
}

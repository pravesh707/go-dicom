// SPDX-License-Identifier: Apache-2.0

package dimse

import "testing"

func TestCEchoRoundTrip(t *testing.T) {
	rq := &CEchoRequest{MessageID: 42}
	b, err := EncodeCommand(rq)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	msg, err := DecodeCommand(b)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	got, ok := msg.(*CEchoRequest)
	if !ok {
		t.Fatalf("type = %T", msg)
	}
	if got.MessageID != 42 {
		t.Errorf("message id = %d", got.MessageID)
	}
	if got.AffectedSOPClassUID != "1.2.840.10008.1.1" {
		t.Errorf("sop class = %q", got.AffectedSOPClassUID)
	}
	if got.CommandField() != CEchoRQ || got.HasDataSet() {
		t.Errorf("command field/dataset wrong")
	}
}

func TestCEchoResponseStatus(t *testing.T) {
	rsp := &CEchoResponse{MessageIDBeingRespondedTo: 42, Status: StatusSuccess}
	b, _ := EncodeCommand(rsp)
	msg, err := DecodeCommand(b)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	got := msg.(*CEchoResponse)
	if got.MessageIDBeingRespondedTo != 42 {
		t.Errorf("responded-to id = %d", got.MessageIDBeingRespondedTo)
	}
	if !got.Status.IsSuccess() {
		t.Errorf("status not success: %#x", uint16(got.Status))
	}
}

func TestStatusCategories(t *testing.T) {
	cases := map[Status]Category{
		StatusSuccess:                CategorySuccess,
		StatusPending:                CategoryPending,
		StatusCancel:                 CategoryCancel,
		StatusCoercionOfDataElements: CategoryWarning,
		StatusProcessingFailure:      CategoryFailure,
		StatusRefusedOutOfResources:  CategoryFailure,
	}
	for s, want := range cases {
		if got := s.Category(); got != want {
			t.Errorf("status %#x: category = %v, want %v", uint16(s), got, want)
		}
	}
}

// SPDX-License-Identifier: Apache-2.0

// Package tests holds the dimse package's tests. They exercise the package
// through its exported API (black box), so they live in this subdirectory
// rather than alongside the code.
package tests

import (
	"testing"

	"github.com/pravesh707/go-dicom/dimse"
)

func TestCEchoRoundTrip(t *testing.T) {
	rq := &dimse.CEchoRequest{MessageID: 42}
	b, err := dimse.EncodeCommand(rq)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	msg, err := dimse.DecodeCommand(b)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	got, ok := msg.(*dimse.CEchoRequest)
	if !ok {
		t.Fatalf("type = %T", msg)
	}
	if got.MessageID != 42 {
		t.Errorf("message id = %d", got.MessageID)
	}
	if got.AffectedSOPClassUID != "1.2.840.10008.1.1" {
		t.Errorf("sop class = %q", got.AffectedSOPClassUID)
	}
	if got.CommandField() != dimse.CEchoRQ || got.HasDataSet() {
		t.Errorf("command field/dataset wrong")
	}
}

func TestCEchoResponseStatus(t *testing.T) {
	rsp := &dimse.CEchoResponse{MessageIDBeingRespondedTo: 42, Status: dimse.StatusSuccess}
	b, _ := dimse.EncodeCommand(rsp)
	msg, err := dimse.DecodeCommand(b)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	got := msg.(*dimse.CEchoResponse)
	if got.MessageIDBeingRespondedTo != 42 {
		t.Errorf("responded-to id = %d", got.MessageIDBeingRespondedTo)
	}
	if !got.Status.IsSuccess() {
		t.Errorf("status not success: %#x", uint16(got.Status))
	}
}

func TestStatusCategories(t *testing.T) {
	cases := map[dimse.Status]dimse.Category{
		dimse.StatusSuccess:                dimse.CategorySuccess,
		dimse.StatusPending:                dimse.CategoryPending,
		dimse.StatusCancel:                 dimse.CategoryCancel,
		dimse.StatusCoercionOfDataElements: dimse.CategoryWarning,
		dimse.StatusProcessingFailure:      dimse.CategoryFailure,
		dimse.StatusRefusedOutOfResources:  dimse.CategoryFailure,
	}
	for s, want := range cases {
		if got := s.Category(); got != want {
			t.Errorf("status %#x: category = %v, want %v", uint16(s), got, want)
		}
	}
}

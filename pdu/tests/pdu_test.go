// SPDX-License-Identifier: Apache-2.0

// Package tests holds the pdu package's tests. They exercise the package
// through its exported API (black box), so they live in this subdirectory
// rather than alongside the code.
package tests

import (
	"bytes"
	"testing"

	"github.com/pravesh707/go-dicom/pdu"
)

func TestAssociateRQRoundTrip(t *testing.T) {
	rq := &pdu.AssociateRQ{
		CalledAETitle:      "STORE_SCP",
		CallingAETitle:     "ECHO_SCU",
		ApplicationContext: "1.2.840.10008.3.1.1.1",
		PresentationContexts: []pdu.PresentationContextRQ{
			{ID: 1, AbstractSyntax: "1.2.840.10008.1.1", TransferSyntaxes: []string{
				"1.2.840.10008.1.2", "1.2.840.10008.1.2.1",
			}},
		},
		UserInformation: pdu.UserInformation{
			MaximumLength:             16384,
			ImplementationClassUID:    "1.2.826.0.1.3680043.10.1337.1",
			ImplementationVersionName: "GODICOM_0_1",
		},
	}
	rq.UserInformation.RoleSelection = []pdu.RoleSelection{
		{SOPClassUID: "1.2.840.10008.5.1.4.1.1.2", SCURole: true, SCPRole: true},
	}

	encoded, err := rq.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	if encoded[0] != pdu.TypeAssociateRQ {
		t.Fatalf("pdu type = %#x", encoded[0])
	}

	decoded, err := pdu.ReadPDU(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got, ok := decoded.(*pdu.AssociateRQ)
	if !ok {
		t.Fatalf("decoded type = %T", decoded)
	}
	if got.CalledAETitle != "STORE_SCP" || got.CallingAETitle != "ECHO_SCU" {
		t.Errorf("AE titles = %q / %q", got.CalledAETitle, got.CallingAETitle)
	}
	if len(got.PresentationContexts) != 1 || got.PresentationContexts[0].AbstractSyntax != "1.2.840.10008.1.1" {
		t.Fatalf("presentation contexts = %+v", got.PresentationContexts)
	}
	if len(got.PresentationContexts[0].TransferSyntaxes) != 2 {
		t.Errorf("transfer syntaxes = %v", got.PresentationContexts[0].TransferSyntaxes)
	}
	if got.UserInformation.MaximumLength != 16384 {
		t.Errorf("max length = %d", got.UserInformation.MaximumLength)
	}
	if len(got.UserInformation.RoleSelection) != 1 || !got.UserInformation.RoleSelection[0].SCPRole {
		t.Errorf("role selection = %+v", got.UserInformation.RoleSelection)
	}
}

func TestPDataTFRoundTrip(t *testing.T) {
	cmd := []byte{0x01, 0x02, 0x03, 0x04}
	p := &pdu.PDataTF{PDVs: []pdu.PDV{
		{ContextID: 1, IsCommand: true, IsLast: true, Data: cmd},
	}}
	encoded, err := p.Encode()
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	decoded, err := pdu.ReadPDU(bytes.NewReader(encoded))
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	got := decoded.(*pdu.PDataTF)
	if len(got.PDVs) != 1 {
		t.Fatalf("pdv count = %d", len(got.PDVs))
	}
	v := got.PDVs[0]
	if v.ContextID != 1 || !v.IsCommand || !v.IsLast || !bytes.Equal(v.Data, cmd) {
		t.Errorf("pdv = %+v", v)
	}
}

func TestControlPDUs(t *testing.T) {
	for _, p := range []pdu.PDU{&pdu.ReleaseRQ{}, &pdu.ReleaseRP{}, &pdu.Abort{Source: pdu.AbortSourceServiceUser, Reason: pdu.AbortReasonNotSpecified}} {
		encoded, err := p.Encode()
		if err != nil {
			t.Fatalf("encode %T: %v", p, err)
		}
		if _, err := pdu.ReadPDU(bytes.NewReader(encoded)); err != nil {
			t.Fatalf("read %T: %v", p, err)
		}
	}
}

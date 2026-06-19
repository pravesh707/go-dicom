// SPDX-License-Identifier: Apache-2.0

package pdu

import (
	"bytes"
	"encoding/binary"
	"testing"
)

func TestPDUHeaderLength(t *testing.T) {
	rq := &AssociateRQ{
		CalledAETitle:      "SCP",
		CallingAETitle:     "SCU",
		ApplicationContext: "1.2.840.10008.3.1.1.1",
		PresentationContexts: []PresentationContextRQ{
			{ID: 1, AbstractSyntax: "1.2.840.10008.1.1", TransferSyntaxes: []string{"1.2.840.10008.1.2"}},
		},
		UserInformation: UserInformation{MaximumLength: 16384},
	}
	b, err := rq.Encode()
	if err != nil {
		t.Fatal(err)
	}
	wantLen := uint32(len(b) - 6)
	if got := binary.BigEndian.Uint32(b[2:]); got != wantLen {
		t.Errorf("PDU length header = %d, want %d", got, wantLen)
	}
}

func TestAETitlePaddingAndTruncation(t *testing.T) {
	rq := &AssociateRQ{
		CalledAETitle:      "THIS_IS_A_VERY_LONG_AETITLE", // > 16, must truncate
		CallingAETitle:     "AB",                          // < 16, must pad/trim
		ApplicationContext: "1.2.840.10008.3.1.1.1",
	}
	b, _ := rq.Encode()
	decoded, err := ReadPDU(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	got := decoded.(*AssociateRQ)
	if got.CalledAETitle != "THIS_IS_A_VERY_L" {
		t.Errorf("called AE title = %q (len %d)", got.CalledAETitle, len(got.CalledAETitle))
	}
	if got.CallingAETitle != "AB" {
		t.Errorf("calling AE title = %q", got.CallingAETitle)
	}
}

func TestAssociateACRoundTrip(t *testing.T) {
	ac := &AssociateAC{
		CalledAETitle:      "SCP",
		CallingAETitle:     "SCU",
		ApplicationContext: "1.2.840.10008.3.1.1.1",
		PresentationContexts: []PresentationContextAC{
			{ID: 1, Result: PCAccepted, TransferSyntax: "1.2.840.10008.1.2"},
			{ID: 3, Result: PCAbstractSyntaxNotSup}, // rejected, no TS
		},
		UserInformation: UserInformation{MaximumLength: 0}, // 0 = unlimited
	}
	b, _ := ac.Encode()
	decoded, err := ReadPDU(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	got := decoded.(*AssociateAC)
	if len(got.PresentationContexts) != 2 {
		t.Fatalf("contexts = %d", len(got.PresentationContexts))
	}
	if got.PresentationContexts[0].Result != PCAccepted || got.PresentationContexts[0].TransferSyntax != "1.2.840.10008.1.2" {
		t.Errorf("accepted context = %+v", got.PresentationContexts[0])
	}
	if got.PresentationContexts[1].Result != PCAbstractSyntaxNotSup {
		t.Errorf("rejected context result = %#x", got.PresentationContexts[1].Result)
	}
	if got.UserInformation.MaximumLength != 0 {
		t.Errorf("max length = %d", got.UserInformation.MaximumLength)
	}
}

func TestAssociateRJRoundTrip(t *testing.T) {
	rj := &AssociateRJ{Result: RJResultRejectedPermanent, Source: RJSourceServiceUser, Reason: RJReasonCalledAENotRecognized}
	b, _ := rj.Encode()
	decoded, err := ReadPDU(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	got := decoded.(*AssociateRJ)
	if got.Result != RJResultRejectedPermanent || got.Source != RJSourceServiceUser || got.Reason != RJReasonCalledAENotRecognized {
		t.Errorf("RJ fields = %+v", got)
	}
}

func TestAbortFields(t *testing.T) {
	ab := &Abort{Source: AbortSourceServiceProvider, Reason: AbortReasonUnexpectedPDU}
	b, _ := ab.Encode()
	decoded, err := ReadPDU(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	got := decoded.(*Abort)
	if got.Source != AbortSourceServiceProvider || got.Reason != AbortReasonUnexpectedPDU {
		t.Errorf("abort fields = %+v", got)
	}
}

func TestUserInformationFull(t *testing.T) {
	ui := UserInformation{
		MaximumLength:             16384,
		ImplementationClassUID:    "1.2.3.4",
		ImplementationVersionName: "GODICOM_0_1",
		RoleSelection: []RoleSelection{
			{SOPClassUID: "1.2.840.10008.5.1.4.1.1.2", SCURole: true, SCPRole: false},
		},
		Extra: [][2]any{{int(itemExtendedNegotiation), []byte{0x01, 0x02, 0x03, 0x04}}},
	}
	ui.SetAsyncOps(5, 7)

	rq := &AssociateRQ{ApplicationContext: "1.2.840.10008.3.1.1.1", UserInformation: ui}
	b, _ := rq.Encode()
	decoded, err := ReadPDU(bytes.NewReader(b))
	if err != nil {
		t.Fatal(err)
	}
	g := decoded.(*AssociateRQ).UserInformation
	if g.ImplementationClassUID != "1.2.3.4" || g.ImplementationVersionName != "GODICOM_0_1" {
		t.Errorf("impl identity = %+v", g)
	}
	if g.MaxOpsInvoked != 5 || g.MaxOpsPerformed != 7 {
		t.Errorf("async ops = %d/%d", g.MaxOpsInvoked, g.MaxOpsPerformed)
	}
	if len(g.RoleSelection) != 1 || !g.RoleSelection[0].SCURole || g.RoleSelection[0].SCPRole {
		t.Errorf("role selection = %+v", g.RoleSelection)
	}
	// Unknown sub-item preserved verbatim.
	if len(g.Extra) != 1 || g.Extra[0][0].(int) != int(itemExtendedNegotiation) ||
		!bytes.Equal(g.Extra[0][1].([]byte), []byte{1, 2, 3, 4}) {
		t.Errorf("extra sub-item not preserved: %+v", g.Extra)
	}
}

func TestMultiplePresentationContexts(t *testing.T) {
	rq := &AssociateRQ{
		ApplicationContext: "1.2.840.10008.3.1.1.1",
		PresentationContexts: []PresentationContextRQ{
			{ID: 1, AbstractSyntax: "1.2.840.10008.1.1", TransferSyntaxes: []string{"1.2.840.10008.1.2", "1.2.840.10008.1.2.1"}},
			{ID: 3, AbstractSyntax: "1.2.840.10008.5.1.4.1.1.2", TransferSyntaxes: []string{"1.2.840.10008.1.2"}},
		},
	}
	b, _ := rq.Encode()
	decoded, _ := ReadPDU(bytes.NewReader(b))
	got := decoded.(*AssociateRQ)
	if len(got.PresentationContexts) != 2 {
		t.Fatalf("contexts = %d", len(got.PresentationContexts))
	}
	if len(got.PresentationContexts[0].TransferSyntaxes) != 2 {
		t.Errorf("first context TS = %v", got.PresentationContexts[0].TransferSyntaxes)
	}
	if got.PresentationContexts[1].ID != 3 {
		t.Errorf("second context ID = %d", got.PresentationContexts[1].ID)
	}
}

func TestPDVControlHeaderCombinations(t *testing.T) {
	cases := []PDV{
		{ContextID: 1, IsCommand: true, IsLast: false, Data: []byte{1}},
		{ContextID: 1, IsCommand: true, IsLast: true, Data: []byte{2}},
		{ContextID: 5, IsCommand: false, IsLast: false, Data: []byte{3}},
		{ContextID: 5, IsCommand: false, IsLast: true, Data: []byte{4}},
	}
	p := &PDataTF{PDVs: cases}
	b, _ := p.Encode()
	decoded, _ := ReadPDU(bytes.NewReader(b))
	got := decoded.(*PDataTF)
	if len(got.PDVs) != 4 {
		t.Fatalf("pdv count = %d", len(got.PDVs))
	}
	for i, want := range cases {
		g := got.PDVs[i]
		if g.ContextID != want.ContextID || g.IsCommand != want.IsCommand || g.IsLast != want.IsLast || !bytes.Equal(g.Data, want.Data) {
			t.Errorf("pdv[%d] = %+v, want %+v", i, g, want)
		}
	}
}

func TestReadPDUSequential(t *testing.T) {
	var buf bytes.Buffer
	WritePDU(&buf, &ReleaseRQ{})
	WritePDU(&buf, &ReleaseRP{})
	WritePDU(&buf, &Abort{})

	if _, err := ReadPDU(&buf); err != nil {
		t.Fatalf("first PDU: %v", err)
	}
	if p, err := ReadPDU(&buf); err != nil || p.PDUType() != TypeReleaseRP {
		t.Fatalf("second PDU: %v type=%#x", err, p.PDUType())
	}
	if p, err := ReadPDU(&buf); err != nil || p.PDUType() != TypeAbort {
		t.Fatalf("third PDU: %v", err)
	}
}

func TestDecodeUnknownPDUType(t *testing.T) {
	if _, err := Decode(0x99, []byte{}); err == nil {
		t.Error("unknown PDU type should error")
	}
}

func TestReadPDUTruncatedHeader(t *testing.T) {
	if _, err := ReadPDU(bytes.NewReader([]byte{0x01, 0x00})); err == nil {
		t.Error("truncated header should error")
	}
}

func TestDecodeShortBodiesError(t *testing.T) {
	if _, err := Decode(TypeAssociateRQ, make([]byte, 10)); err == nil {
		t.Error("short A-ASSOCIATE-RQ should error")
	}
	if _, err := Decode(TypeAssociateAC, make([]byte, 10)); err == nil {
		t.Error("short A-ASSOCIATE-AC should error")
	}
	if _, err := Decode(TypeAssociateRJ, []byte{0x00, 0x01}); err == nil {
		t.Error("short A-ASSOCIATE-RJ should error")
	}
	if _, err := Decode(TypeAbort, []byte{0x00, 0x00}); err == nil {
		t.Error("short A-ABORT should error")
	}
}

func TestDecodeInvalidPDV(t *testing.T) {
	// PDV length field of 1 is invalid (< 2 = ctxID + control header).
	body := []byte{0x00, 0x00, 0x00, 0x01, 0xAA}
	if _, err := Decode(TypePDataTF, body); err == nil {
		t.Error("invalid PDV length should error")
	}
}

func TestRegisterPDU(t *testing.T) {
	if _, ok := decoderRegistry[TypeAssociateRQ]; !ok {
		t.Error("A-ASSOCIATE-RQ not registered")
	}
	if len(decoderRegistry) != 7 {
		t.Errorf("expected 7 registered PDU decoders, got %d", len(decoderRegistry))
	}
}

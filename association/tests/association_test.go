// SPDX-License-Identifier: Apache-2.0

// Package tests holds the association package's tests. They exercise the
// package through its exported API (black box) — including the low-level seams
// in export.go — so they live in this subdirectory rather than alongside the
// code.
package tests

import (
	"bytes"
	"errors"
	"net"
	"testing"

	"github.com/pravesh707/go-dicom/association"
	"github.com/pravesh707/go-dicom/dicom"
	"github.com/pravesh707/go-dicom/dimse"
	"github.com/pravesh707/go-dicom/pdu"
)

const verification = "1.2.840.10008.1.1"
const ctStorage = "1.2.840.10008.5.1.4.1.1.2"

func TestFirstCommon(t *testing.T) {
	if got := association.FirstCommonTransferSyntax([]string{"a", "b"}, []string{"b", "a"}); got != "a" {
		t.Errorf("preference order: got %q, want requestor's first match 'a'", got)
	}
	if got := association.FirstCommonTransferSyntax([]string{"x"}, []string{"y"}); got != "" {
		t.Errorf("no common: got %q, want empty", got)
	}
	if got := association.FirstCommonTransferSyntax(nil, []string{"a"}); got != "" {
		t.Errorf("empty prefer: got %q", got)
	}
}

func TestNegotiateOutcomes(t *testing.T) {
	supported := []association.SupportedContext{
		{AbstractSyntax: verification, TransferSyntaxes: []string{dicom.ImplicitVRLittleEndian, dicom.ExplicitVRLittleEndian}},
		{AbstractSyntax: ctStorage, TransferSyntaxes: []string{dicom.ExplicitVRLittleEndian}},
	}
	requested := []pdu.PresentationContextRQ{
		{ID: 1, AbstractSyntax: verification, TransferSyntaxes: []string{dicom.ExplicitVRLittleEndian, dicom.ImplicitVRLittleEndian}},
		{ID: 3, AbstractSyntax: "1.2.999", TransferSyntaxes: []string{dicom.ImplicitVRLittleEndian}}, // abstract not supported
		{ID: 5, AbstractSyntax: ctStorage, TransferSyntaxes: []string{dicom.ImplicitVRLittleEndian}}, // no common TS
	}

	items, accepted := association.Negotiate(requested, supported)
	byID := map[byte]pdu.PresentationContextAC{}
	for _, it := range items {
		byID[it.ID] = it
	}

	if byID[1].Result != pdu.PCAccepted {
		t.Errorf("ctx 1 result = %#x, want accepted", byID[1].Result)
	}
	// Requestor preference is honoured: Explicit comes first in the request.
	if byID[1].TransferSyntax != dicom.ExplicitVRLittleEndian {
		t.Errorf("ctx 1 TS = %q, want explicit (requestor preference)", byID[1].TransferSyntax)
	}
	if byID[3].Result != pdu.PCAbstractSyntaxNotSup {
		t.Errorf("ctx 3 result = %#x, want abstract-syntax-not-supported", byID[3].Result)
	}
	if byID[5].Result != pdu.PCTransferSyntaxNotSup {
		t.Errorf("ctx 5 result = %#x, want transfer-syntax-not-supported", byID[5].Result)
	}
	if _, ok := accepted[1]; !ok {
		t.Error("ctx 1 should be in accepted map")
	}
	if _, ok := accepted[3]; ok {
		t.Error("ctx 3 should not be accepted")
	}
}

func TestEventTypeStringAndIntervention(t *testing.T) {
	if association.EvtCEcho.String() != "EVT_C_ECHO" || association.EvtEstablished.String() != "EVT_ESTABLISHED" {
		t.Error("event names wrong")
	}
	if association.EventType(999).String() != "EVT_UNKNOWN" {
		t.Error("unknown event name")
	}
	if !association.EvtCEcho.IsIntervention() {
		t.Error("C-ECHO is an intervention event")
	}
	if association.EvtEstablished.IsIntervention() {
		t.Error("ESTABLISHED is a notification event")
	}
}

func TestHandlerTable(t *testing.T) {
	var notified int
	ht := association.NewHandlerTable([]association.HandlerBinding{
		{Event: association.EvtCEcho, Handle: func(*association.Event) dimse.Status { return dimse.StatusProcessingFailure }},
		{Event: association.EvtEstablished, Handle: func(*association.Event) dimse.Status { notified++; return dimse.StatusSuccess }},
		{Event: association.EvtEstablished, Handle: func(*association.Event) dimse.Status { notified++; return dimse.StatusSuccess }},
	})

	status, handled := ht.Handle(&association.Event{Type: association.EvtCEcho})
	if !handled || status != dimse.StatusProcessingFailure {
		t.Errorf("intervention handler: status=%#x handled=%v", uint16(status), handled)
	}

	// No handler registered for an event -> default success, not handled.
	if status, handled := ht.Handle(&association.Event{Type: association.EvtAborted}); handled || !status.IsSuccess() {
		t.Errorf("default: status=%#x handled=%v", uint16(status), handled)
	}

	ht.Emit(&association.Event{Type: association.EvtEstablished})
	if notified != 2 {
		t.Errorf("notification fan-out = %d, want 2", notified)
	}
}

func TestEventMessageID(t *testing.T) {
	ev := &association.Event{Request: &dimse.CEchoRequest{MessageID: 77}}
	if ev.MessageID() != 77 {
		t.Errorf("MessageID = %d", ev.MessageID())
	}
	if (&association.Event{}).MessageID() != 0 {
		t.Error("nil request should give 0")
	}
}

func TestPeerMaxData(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	a := association.NewAssociation(client, true)
	a.SetPeerMaxLength(0)
	if a.PeerMaxData() != association.DefaultMaxPDU-12 {
		t.Errorf("zero max -> %d", a.PeerMaxData())
	}
	a.SetPeerMaxLength(100) // below the 256 floor
	if a.PeerMaxData() != 256 {
		t.Errorf("small max -> %d, want floor 256", a.PeerMaxData())
	}
	a.SetPeerMaxLength(16384)
	if a.PeerMaxData() != 16372 {
		t.Errorf("normal max -> %d", a.PeerMaxData())
	}
}

func TestPDVFragmentationReassembly(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	a := association.NewAssociation(client, true)
	a.SetPeerMaxLength(268) // chunk = 256

	payload := bytes.Repeat([]byte{0xAB}, 1000) // forces 4 fragments
	go func() { _ = a.SendPDVStream(1, payload, true) }()

	var got []byte
	fragments := 0
	for {
		p, err := pdu.ReadPDU(server)
		if err != nil {
			t.Fatalf("read: %v", err)
		}
		tf := p.(*pdu.PDataTF)
		for _, v := range tf.PDVs {
			fragments++
			if v.ContextID != 1 || !v.IsCommand {
				t.Errorf("pdv ctx/command wrong: %+v", v)
			}
			got = append(got, v.Data...)
			if v.IsLast {
				if !bytes.Equal(got, payload) {
					t.Errorf("reassembled %d bytes, want %d", len(got), len(payload))
				}
				if fragments != 4 {
					t.Errorf("fragments = %d, want 4", fragments)
				}
				return
			}
		}
	}
}

func TestSendReadCEchoOverPipe(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	ctx := association.AcceptedContext{ID: 1, AbstractSyntax: verification, TransferSyntax: dicom.ImplicitVRLittleEndian}
	a := association.NewAssociation(client, true)
	b := association.NewAssociation(server, false)
	b.SetAcceptedContext(1, ctx)

	go func() { _ = a.SendMessage(ctx, &dimse.CEchoRequest{MessageID: 5}, nil) }()

	gotCtx, msg, data, err := b.ReadMessage()
	if err != nil {
		t.Fatalf("readMessage: %v", err)
	}
	if data != nil {
		t.Error("C-ECHO carries no data set")
	}
	if gotCtx.ID != 1 {
		t.Errorf("context = %+v", gotCtx)
	}
	rq, ok := msg.(*dimse.CEchoRequest)
	if !ok || rq.MessageID != 5 {
		t.Errorf("decoded message = %#v", msg)
	}
}

func TestReadMessageControlPDUs(t *testing.T) {
	cases := []struct {
		name  string
		write pdu.PDU
		want  error
	}{
		{"release-rq", &pdu.ReleaseRQ{}, association.ErrReleaseRequested},
		{"release-rp", &pdu.ReleaseRP{}, association.ErrReleaseConfirmed},
		{"abort", &pdu.Abort{}, association.ErrAborted},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client, server := net.Pipe()
			defer client.Close()
			defer server.Close()
			b := association.NewAssociation(server, false)
			go func() { _ = pdu.WritePDU(client, c.write) }()
			_, _, _, err := b.ReadMessage()
			if !errors.Is(err, c.want) {
				t.Errorf("got %v, want %v", err, c.want)
			}
		})
	}
}

func TestRequestAcceptNegotiation(t *testing.T) {
	client, server := net.Pipe()

	type result struct {
		a   *association.Association
		err error
	}
	ch := make(chan result, 1)
	go func() {
		a, err := association.Request(client, association.RequestParams{
			CallingAETitle:    "SCU",
			CalledAETitle:     "SCP",
			RequestedContexts: []association.RequestedContext{{AbstractSyntax: verification, TransferSyntaxes: []string{dicom.ImplicitVRLittleEndian}}},
			MaximumLength:     16384,
		})
		ch <- result{a, err}
	}()

	b, err := association.Accept(server, association.AcceptParams{
		AETitle:           "SCP",
		SupportedContexts: []association.SupportedContext{{AbstractSyntax: verification, TransferSyntaxes: []string{dicom.ImplicitVRLittleEndian}}},
		MaximumLength:     16384,
	})
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	r := <-ch
	if r.err != nil {
		t.Fatalf("request: %v", r.err)
	}
	if _, ok := r.a.ContextForSyntax(verification); !ok {
		t.Error("requestor did not record accepted Verification context")
	}
	if _, ok := b.ContextForSyntax(verification); !ok {
		t.Error("acceptor did not record accepted Verification context")
	}
	if b.CallingAETitle != "SCU" || b.CalledAETitle != "SCP" {
		t.Errorf("acceptor AE titles = %q/%q", b.CallingAETitle, b.CalledAETitle)
	}
}

func TestRequestAcceptRejection(t *testing.T) {
	client, server := net.Pipe()

	type result struct {
		a   *association.Association
		err error
	}
	ch := make(chan result, 1)
	go func() {
		a, err := association.Request(client, association.RequestParams{
			CallingAETitle:    "SCU",
			CalledAETitle:     "WRONG",
			RequestedContexts: []association.RequestedContext{{AbstractSyntax: verification, TransferSyntaxes: []string{dicom.ImplicitVRLittleEndian}}},
		})
		ch <- result{a, err}
	}()

	_, err := association.Accept(server, association.AcceptParams{
		AETitle:           "REAL",
		RequireCalledAET:  true, // RQ's called AE "WRONG" != "REAL" -> reject
		SupportedContexts: []association.SupportedContext{{AbstractSyntax: verification, TransferSyntaxes: []string{dicom.ImplicitVRLittleEndian}}},
	})
	if err == nil {
		t.Error("acceptor should reject mismatched called AE title")
	}
	r := <-ch
	var rejErr *association.RejectedError
	if !errors.As(r.err, &rejErr) {
		t.Fatalf("requestor error = %v, want *RejectedError", r.err)
	}
	if rejErr.Reason != pdu.RJReasonCalledAENotRecognized {
		t.Errorf("reject reason = %#x", rejErr.Reason)
	}
}

func TestRejectedErrorMessage(t *testing.T) {
	e := &association.RejectedError{Result: 1, Source: 1, Reason: 7}
	if e.Error() == "" {
		t.Error("RejectedError.Error() should be non-empty")
	}
}

// TestRoleSelectionRequestorEmits is the regression guard for the C-GET SCU
// role-selection bug: a context with ScpRole set must produce an SCP/SCU Role
// Selection sub-item in the A-ASSOCIATE-RQ (and a context without one must not),
// otherwise a standards-strict C-GET SCP refuses to return instances.
func TestRoleSelectionRequestorEmits(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	type result struct {
		a   *association.Association
		err error
	}
	ch := make(chan result, 1)
	go func() {
		a, err := association.Request(client, association.RequestParams{
			CallingAETitle: "GET_SCU",
			CalledAETitle:  "GET_SCP",
			RequestedContexts: []association.RequestedContext{
				{AbstractSyntax: verification, TransferSyntaxes: []string{dicom.ImplicitVRLittleEndian}},
				{AbstractSyntax: ctStorage, TransferSyntaxes: []string{dicom.ImplicitVRLittleEndian}, ScpRole: true},
			},
			MaximumLength: 16384,
		})
		ch <- result{a, err}
	}()

	p, err := pdu.ReadPDU(server)
	if err != nil {
		t.Fatalf("read RQ: %v", err)
	}
	rq, ok := p.(*pdu.AssociateRQ)
	if !ok {
		t.Fatalf("expected A-ASSOCIATE-RQ, got %T", p)
	}
	// Exactly one role item — for the storage context, SCP role only.
	if len(rq.UserInformation.RoleSelection) != 1 {
		t.Fatalf("role selection items = %d, want 1 (only the context that set a role)", len(rq.UserInformation.RoleSelection))
	}
	rs := rq.UserInformation.RoleSelection[0]
	if rs.SOPClassUID != ctStorage || !rs.SCPRole || rs.SCURole {
		t.Errorf("role selection = %+v, want {%s SCPRole=true SCURole=false}", rs, ctStorage)
	}

	// Reply with an AC accepting both contexts so Request returns cleanly.
	ac := &pdu.AssociateAC{
		CalledAETitle:      rq.CalledAETitle,
		CallingAETitle:     rq.CallingAETitle,
		ApplicationContext: dicom.DICOMApplicationContextName,
		PresentationContexts: []pdu.PresentationContextAC{
			{ID: 1, Result: pdu.PCAccepted, TransferSyntax: dicom.ImplicitVRLittleEndian},
			{ID: 3, Result: pdu.PCAccepted, TransferSyntax: dicom.ImplicitVRLittleEndian},
		},
		UserInformation: pdu.UserInformation{
			MaximumLength: 16384,
			RoleSelection: []pdu.RoleSelection{{SOPClassUID: ctStorage, SCPRole: true}},
		},
	}
	if err := pdu.WritePDU(server, ac); err != nil {
		t.Fatalf("write AC: %v", err)
	}

	r := <-ch
	if r.err != nil {
		t.Fatalf("request: %v", r.err)
	}
	if _, ok := r.a.ContextForSyntax(ctStorage); !ok {
		t.Error("requestor did not record accepted storage context")
	}
}

// TestRoleSelectionAcceptorEchoes verifies the acceptor confirms a proposed SCP
// role for an accepted abstract syntax in the A-ASSOCIATE-AC (PS3.7 §D.3.3.4) —
// the SCP-side half that lets our C-GET SCP send instances back to a peer.
func TestRoleSelectionAcceptorEchoes(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	go func() {
		rq := &pdu.AssociateRQ{
			CalledAETitle:      "SCP",
			CallingAETitle:     "SCU",
			ApplicationContext: dicom.DICOMApplicationContextName,
			PresentationContexts: []pdu.PresentationContextRQ{
				{ID: 1, AbstractSyntax: ctStorage, TransferSyntaxes: []string{dicom.ImplicitVRLittleEndian}},
			},
			UserInformation: pdu.UserInformation{
				MaximumLength: 16384,
				RoleSelection: []pdu.RoleSelection{{SOPClassUID: ctStorage, SCPRole: true}},
			},
		}
		_ = pdu.WritePDU(client, rq)
	}()

	errc := make(chan error, 1)
	go func() {
		_, err := association.Accept(server, association.AcceptParams{
			AETitle:           "SCP",
			SupportedContexts: []association.SupportedContext{{AbstractSyntax: ctStorage, TransferSyntaxes: []string{dicom.ImplicitVRLittleEndian}}},
			MaximumLength:     16384,
		})
		errc <- err
	}()

	p, err := pdu.ReadPDU(client)
	if err != nil {
		t.Fatalf("read AC: %v", err)
	}
	ac, ok := p.(*pdu.AssociateAC)
	if !ok {
		t.Fatalf("expected A-ASSOCIATE-AC, got %T", p)
	}
	if len(ac.UserInformation.RoleSelection) != 1 {
		t.Fatalf("echoed role items = %d, want 1", len(ac.UserInformation.RoleSelection))
	}
	if rs := ac.UserInformation.RoleSelection[0]; rs.SOPClassUID != ctStorage || !rs.SCPRole {
		t.Errorf("echoed role = %+v, want {%s SCPRole=true}", rs, ctStorage)
	}
	if err := <-errc; err != nil {
		t.Fatalf("accept: %v", err)
	}
}

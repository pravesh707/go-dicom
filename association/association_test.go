// SPDX-License-Identifier: Apache-2.0

package association

import (
	"bytes"
	"errors"
	"net"
	"testing"

	"github.com/pravesh707/go-dicom/dicom"
	"github.com/pravesh707/go-dicom/dimse"
	"github.com/pravesh707/go-dicom/pdu"
)

const verification = "1.2.840.10008.1.1"

func TestFirstCommon(t *testing.T) {
	if got := firstCommon([]string{"a", "b"}, []string{"b", "a"}); got != "a" {
		t.Errorf("preference order: got %q, want requestor's first match 'a'", got)
	}
	if got := firstCommon([]string{"x"}, []string{"y"}); got != "" {
		t.Errorf("no common: got %q, want empty", got)
	}
	if got := firstCommon(nil, []string{"a"}); got != "" {
		t.Errorf("empty prefer: got %q", got)
	}
}

func TestNegotiateOutcomes(t *testing.T) {
	supported := []SupportedContext{
		{AbstractSyntax: verification, TransferSyntaxes: []string{dicom.ImplicitVRLittleEndian, dicom.ExplicitVRLittleEndian}},
		{AbstractSyntax: "1.2.840.10008.5.1.4.1.1.2", TransferSyntaxes: []string{dicom.ExplicitVRLittleEndian}},
	}
	requested := []pdu.PresentationContextRQ{
		{ID: 1, AbstractSyntax: verification, TransferSyntaxes: []string{dicom.ExplicitVRLittleEndian, dicom.ImplicitVRLittleEndian}},
		{ID: 3, AbstractSyntax: "1.2.999", TransferSyntaxes: []string{dicom.ImplicitVRLittleEndian}},                   // abstract not supported
		{ID: 5, AbstractSyntax: "1.2.840.10008.5.1.4.1.1.2", TransferSyntaxes: []string{dicom.ImplicitVRLittleEndian}}, // no common TS
	}

	items, accepted := negotiate(requested, supported)
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
	if EvtCEcho.String() != "EVT_C_ECHO" || EvtEstablished.String() != "EVT_ESTABLISHED" {
		t.Error("event names wrong")
	}
	if EventType(999).String() != "EVT_UNKNOWN" {
		t.Error("unknown event name")
	}
	if !EvtCEcho.isIntervention() {
		t.Error("C-ECHO is an intervention event")
	}
	if EvtEstablished.isIntervention() {
		t.Error("ESTABLISHED is a notification event")
	}
}

func TestHandlerTable(t *testing.T) {
	var notified int
	ht := newHandlerTable([]HandlerBinding{
		{Event: EvtCEcho, Handle: func(*Event) dimse.Status { return dimse.StatusProcessingFailure }},
		{Event: EvtEstablished, Handle: func(*Event) dimse.Status { notified++; return dimse.StatusSuccess }},
		{Event: EvtEstablished, Handle: func(*Event) dimse.Status { notified++; return dimse.StatusSuccess }},
	})

	status, handled := ht.handle(&Event{Type: EvtCEcho})
	if !handled || status != dimse.StatusProcessingFailure {
		t.Errorf("intervention handler: status=%#x handled=%v", uint16(status), handled)
	}

	// No handler registered for an event -> default success, not handled.
	if status, handled := ht.handle(&Event{Type: EvtAborted}); handled || !status.IsSuccess() {
		t.Errorf("default: status=%#x handled=%v", uint16(status), handled)
	}

	ht.emit(&Event{Type: EvtEstablished})
	if notified != 2 {
		t.Errorf("notification fan-out = %d, want 2", notified)
	}
}

func TestEventMessageID(t *testing.T) {
	ev := &Event{Request: &dimse.CEchoRequest{MessageID: 77}}
	if ev.MessageID() != 77 {
		t.Errorf("MessageID = %d", ev.MessageID())
	}
	if (&Event{}).MessageID() != 0 {
		t.Error("nil request should give 0")
	}
}

func TestPeerMaxData(t *testing.T) {
	a := &Association{peerMaxLength: 0}
	if a.peerMaxData() != defaultMaxPDU-12 {
		t.Errorf("zero max -> %d", a.peerMaxData())
	}
	a.peerMaxLength = 100 // below the 256 floor
	if a.peerMaxData() != 256 {
		t.Errorf("small max -> %d, want floor 256", a.peerMaxData())
	}
	a.peerMaxLength = 16384
	if a.peerMaxData() != 16372 {
		t.Errorf("normal max -> %d", a.peerMaxData())
	}
}

func TestPDVFragmentationReassembly(t *testing.T) {
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()

	a := newAssociation(client, true)
	a.peerMaxLength = 268 // chunk = 256

	payload := bytes.Repeat([]byte{0xAB}, 1000) // forces 4 fragments
	go func() { _ = a.sendPDVStream(1, payload, true) }()

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

	ctx := AcceptedContext{ID: 1, AbstractSyntax: verification, TransferSyntax: dicom.ImplicitVRLittleEndian}
	a := newAssociation(client, true)
	b := newAssociation(server, false)
	b.acceptedByID[1] = ctx

	go func() { _ = a.sendMessage(ctx, &dimse.CEchoRequest{MessageID: 5}, nil) }()

	gotCtx, msg, data, err := b.readMessage()
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
		{"release-rq", &pdu.ReleaseRQ{}, errReleaseRequested},
		{"release-rp", &pdu.ReleaseRP{}, errReleaseConfirmed},
		{"abort", &pdu.Abort{}, errAborted},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			client, server := net.Pipe()
			defer client.Close()
			defer server.Close()
			b := newAssociation(server, false)
			go func() { _ = pdu.WritePDU(client, c.write) }()
			_, _, _, err := b.readMessage()
			if !errors.Is(err, c.want) {
				t.Errorf("got %v, want %v", err, c.want)
			}
		})
	}
}

func TestRequestAcceptNegotiation(t *testing.T) {
	client, server := net.Pipe()

	type result struct {
		a   *Association
		err error
	}
	ch := make(chan result, 1)
	go func() {
		a, err := Request(client, RequestParams{
			CallingAETitle:    "SCU",
			CalledAETitle:     "SCP",
			RequestedContexts: []RequestedContext{{AbstractSyntax: verification, TransferSyntaxes: []string{dicom.ImplicitVRLittleEndian}}},
			MaximumLength:     16384,
		})
		ch <- result{a, err}
	}()

	b, err := Accept(server, AcceptParams{
		AETitle:           "SCP",
		SupportedContexts: []SupportedContext{{AbstractSyntax: verification, TransferSyntaxes: []string{dicom.ImplicitVRLittleEndian}}},
		MaximumLength:     16384,
	})
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	r := <-ch
	if r.err != nil {
		t.Fatalf("request: %v", r.err)
	}
	if _, ok := r.a.contextForSyntax(verification); !ok {
		t.Error("requestor did not record accepted Verification context")
	}
	if _, ok := b.contextForSyntax(verification); !ok {
		t.Error("acceptor did not record accepted Verification context")
	}
	if b.CallingAETitle != "SCU" || b.CalledAETitle != "SCP" {
		t.Errorf("acceptor AE titles = %q/%q", b.CallingAETitle, b.CalledAETitle)
	}
}

func TestRequestAcceptRejection(t *testing.T) {
	client, server := net.Pipe()

	type result struct {
		a   *Association
		err error
	}
	ch := make(chan result, 1)
	go func() {
		a, err := Request(client, RequestParams{
			CallingAETitle:    "SCU",
			CalledAETitle:     "WRONG",
			RequestedContexts: []RequestedContext{{AbstractSyntax: verification, TransferSyntaxes: []string{dicom.ImplicitVRLittleEndian}}},
		})
		ch <- result{a, err}
	}()

	_, err := Accept(server, AcceptParams{
		AETitle:           "REAL",
		RequireCalledAET:  true, // RQ's called AE "WRONG" != "REAL" -> reject
		SupportedContexts: []SupportedContext{{AbstractSyntax: verification, TransferSyntaxes: []string{dicom.ImplicitVRLittleEndian}}},
	})
	if err == nil {
		t.Error("acceptor should reject mismatched called AE title")
	}
	r := <-ch
	var rejErr *RejectedError
	if !errors.As(r.err, &rejErr) {
		t.Fatalf("requestor error = %v, want *RejectedError", r.err)
	}
	if rejErr.Reason != pdu.RJReasonCalledAENotRecognized {
		t.Errorf("reject reason = %#x", rejErr.Reason)
	}
}

func TestRejectedErrorMessage(t *testing.T) {
	e := &RejectedError{Result: 1, Source: 1, Reason: 7}
	if e.Error() == "" {
		t.Error("RejectedError.Error() should be non-empty")
	}
}

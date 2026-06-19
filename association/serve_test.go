// SPDX-License-Identifier: Apache-2.0

package association

import (
	"net"
	"testing"
	"time"

	"github.com/pravesh707/go-dicom/dicom"
	"github.com/pravesh707/go-dicom/dimse"
	"github.com/pravesh707/go-dicom/pdu"
)

// establishPair returns a connected requestor/acceptor association pair over an
// in-memory pipe, with the Verification context negotiated.
func establishPair(t *testing.T) (requestor, acceptor *Association) {
	t.Helper()
	client, server := net.Pipe()

	ch := make(chan *Association, 1)
	go func() {
		a, err := Request(client, RequestParams{
			CallingAETitle:    "SCU",
			CalledAETitle:     "SCP",
			RequestedContexts: []RequestedContext{{AbstractSyntax: verification, TransferSyntaxes: []string{dicom.ImplicitVRLittleEndian}}},
			MaximumLength:     16384,
		})
		if err != nil {
			t.Errorf("request: %v", err)
		}
		ch <- a
	}()

	b, err := Accept(server, AcceptParams{
		AETitle:           "SCP",
		SupportedContexts: []SupportedContext{{AbstractSyntax: verification, TransferSyntaxes: []string{dicom.ImplicitVRLittleEndian}}},
		MaximumLength:     16384,
	})
	if err != nil {
		t.Fatalf("accept: %v", err)
	}
	return <-ch, b
}

func TestServeCEchoAndRelease(t *testing.T) {
	a, b := establishPair(t)

	var established, released, gotEcho bool
	serveDone := make(chan error, 1)
	go func() {
		serveDone <- b.Serve([]HandlerBinding{
			{Event: EvtEstablished, Handle: func(*Event) dimse.Status { established = true; return dimse.StatusSuccess }},
			{Event: EvtReleased, Handle: func(*Event) dimse.Status { released = true; return dimse.StatusSuccess }},
			{Event: EvtCEcho, Handle: func(e *Event) dimse.Status {
				gotEcho = true
				if e.MessageID() == 0 {
					t.Error("echo event missing message id")
				}
				return dimse.StatusSuccess
			}},
		})
	}()

	status, err := a.SendCEcho()
	if err != nil || !status.IsSuccess() {
		t.Fatalf("send c-echo: status=%#x err=%v", uint16(status), err)
	}
	if err := a.Release(); err != nil {
		t.Errorf("release: %v", err)
	}

	select {
	case err := <-serveDone:
		if err != nil {
			t.Errorf("serve returned error: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after release")
	}

	if !established || !released || !gotEcho {
		t.Errorf("events: established=%v released=%v echo=%v", established, released, gotEcho)
	}
}

func TestServeUnhandledServiceAborts(t *testing.T) {
	a, b := establishPair(t)

	// Drain anything the acceptor sends back (the A-ABORT) so its write does not
	// block on the synchronous pipe.
	go func() {
		for {
			if _, err := pdu.ReadPDU(a.reader); err != nil {
				return
			}
		}
	}()

	serveDone := make(chan error, 1)
	go func() { serveDone <- b.Serve(nil) }()

	// Send an unregistered DIMSE service (C-MOVE) the server cannot handle.
	ctx, _ := a.contextForSyntax(verification)
	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUI(dicom.TagAffectedSOPClassUID, "1.2.3"))
	ds.Set(dicom.NewUS(dicom.TagCommandField, uint16(dimse.CCancelRQ)))
	ds.Set(dicom.NewUS(dicom.TagCommandDataSetType, 0x0101))
	raw := &dimse.RawMessage{Command: dimse.CCancelRQ, Set: ds}
	go func() { _ = a.sendMessage(ctx, raw, nil) }()

	select {
	case err := <-serveDone:
		if err == nil {
			t.Error("serve should error on unhandled DIMSE service")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return")
	}
	a.Close()
}

func TestServeReturnsOnConnectionClose(t *testing.T) {
	a, b := establishPair(t)
	serveDone := make(chan error, 1)
	go func() { serveDone <- b.Serve(nil) }()

	// Abruptly close the requestor side; the acceptor's read should end the loop.
	a.Close()

	select {
	case <-serveDone: // nil (EOF) or error — either way it must return
	case <-time.After(5 * time.Second):
		t.Fatal("serve did not return after connection close")
	}
}

func TestAbortClosesConnection(t *testing.T) {
	a, b := establishPair(t)
	defer b.Close()

	// Drain the peer so the A-ABORT write does not block on the pipe.
	go func() {
		for {
			if _, err := pdu.ReadPDU(b.reader); err != nil {
				return
			}
		}
	}()

	if err := a.Abort(); err != nil {
		t.Errorf("abort: %v", err)
	}
	// Second close is a no-op.
	if err := a.Close(); err != nil {
		t.Errorf("double close: %v", err)
	}
}

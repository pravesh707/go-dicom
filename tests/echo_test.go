// SPDX-License-Identifier: Apache-2.0

// Package tests holds the end-to-end / integration tests that exercise godicom
// through its public API only. Package-internal unit tests live alongside the
// code they test (Go requires white-box tests to share the package directory).
package tests

import (
	"sync"
	"testing"

	godicom "github.com/pravesh707/go-dicom"
)

// TestCEchoInProcess starts an SCP, associates as an SCU, exchanges a C-ECHO,
// and releases — the full vertical slice end to end.
func TestCEchoInProcess(t *testing.T) {
	scp := godicom.NewAE("ECHO_SCP")
	scp.AddSupportedContext(godicom.VerificationSOPClass)

	var handled bool
	srv, err := scp.StartServer("127.0.0.1:0", []godicom.HandlerBinding{
		{Event: godicom.EvtCEcho, Handle: func(e *godicom.Event) godicom.Status {
			handled = true
			if e.Assoc.CallingAETitle != "ECHO_SCU" {
				t.Errorf("calling AE = %q", e.Assoc.CallingAETitle)
			}
			return godicom.StatusSuccess
		}},
	})
	if err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer srv.Shutdown()

	scu := godicom.NewAE("ECHO_SCU")
	scu.AddRequestedContext(godicom.VerificationSOPClass)

	assoc, err := scu.Associate(srv.Addr().String())
	if err != nil {
		t.Fatalf("associate: %v", err)
	}
	status, err := assoc.SendCEcho()
	if err != nil {
		t.Fatalf("c-echo: %v", err)
	}
	if !status.IsSuccess() {
		t.Errorf("c-echo status = %#x, want success", uint16(status))
	}
	if err := assoc.Release(); err != nil {
		t.Errorf("release: %v", err)
	}
	if !handled {
		t.Error("server handler was not invoked")
	}
}

// TestConcurrentAssociations drives many simultaneous associations against one
// SCP, exercising the goroutine-per-association server — the core motivation
// for the Go port.
func TestConcurrentAssociations(t *testing.T) {
	scp := godicom.NewAE("ECHO_SCP")
	scp.AddSupportedContext(godicom.VerificationSOPClass)
	srv, err := scp.StartServer("127.0.0.1:0", []godicom.HandlerBinding{
		{Event: godicom.EvtCEcho, Handle: func(e *godicom.Event) godicom.Status { return godicom.StatusSuccess }},
	})
	if err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer srv.Shutdown()

	const n = 25
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for i := 0; i < n; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			scu := godicom.NewAE("ECHO_SCU")
			scu.AddRequestedContext(godicom.VerificationSOPClass)
			assoc, err := scu.Associate(srv.Addr().String())
			if err != nil {
				errs <- err
				return
			}
			defer assoc.Release()
			status, err := assoc.SendCEcho()
			if err != nil {
				errs <- err
				return
			}
			if !status.IsSuccess() {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Errorf("concurrent association failed: %v", err)
		}
	}
}

// TestAssociateNoCommonContext verifies that an SCU requesting a context the SCP
// does not support still associates, but finds no usable context for C-ECHO.
func TestAssociateNoCommonContext(t *testing.T) {
	scp := godicom.NewAE("SCP")
	scp.AddSupportedContext("1.2.840.10008.5.1.4.1.1.2") // CT Storage only
	srv, err := scp.StartServer("127.0.0.1:0", nil)
	if err != nil {
		t.Fatalf("start server: %v", err)
	}
	defer srv.Shutdown()

	scu := godicom.NewAE("SCU")
	scu.AddRequestedContext(godicom.VerificationSOPClass)
	assoc, err := scu.Associate(srv.Addr().String())
	if err != nil {
		t.Fatalf("associate: %v", err)
	}
	defer assoc.Release()
	if _, err := assoc.SendCEcho(); err == nil {
		t.Error("expected error: no accepted context for verification")
	}
}

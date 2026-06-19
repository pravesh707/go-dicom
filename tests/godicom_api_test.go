// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"errors"
	"net"
	"regexp"
	"testing"

	godicom "github.com/pravesh707/go-dicom"
)

func TestNewAEDefaults(t *testing.T) {
	ae := godicom.NewAE("AET")
	if ae.AETitle != "AET" {
		t.Errorf("AE title = %q", ae.AETitle)
	}
	if ae.MaximumLength != 16384 {
		t.Errorf("default max length = %d", ae.MaximumLength)
	}
	if ae.ImplementationClassUID == "" || ae.ImplementationVersionName == "" {
		t.Error("implementation identity should default to non-empty")
	}
	if ae.RequireCalledAET {
		t.Error("RequireCalledAET should default false")
	}
}

func TestNewAEOptions(t *testing.T) {
	ae := godicom.NewAE("AET",
		godicom.WithMaximumLength(32768),
		godicom.WithImplementationClassUID("1.9.9"),
		godicom.WithImplementationVersionName("APP_2"),
		godicom.RequireCalledAETitle(),
	)
	if ae.MaximumLength != 32768 {
		t.Errorf("max length = %d", ae.MaximumLength)
	}
	if ae.ImplementationClassUID != "1.9.9" {
		t.Errorf("impl class uid = %q", ae.ImplementationClassUID)
	}
	if ae.ImplementationVersionName != "APP_2" {
		t.Errorf("impl version = %q", ae.ImplementationVersionName)
	}
	if !ae.RequireCalledAET {
		t.Error("RequireCalledAET should be set")
	}
}

func TestAddContextsDefaultTransferSyntaxes(t *testing.T) {
	ae := godicom.NewAE("AET")
	ae.AddRequestedContext(godicom.VerificationSOPClass)
	ae.AddSupportedContext(godicom.VerificationSOPClass, godicom.ImplicitVRLittleEndian)

	rc := ae.RequestedContexts()
	if len(rc) != 1 || len(rc[0].TransferSyntaxes) != len(godicom.DefaultTransferSyntaxes) {
		t.Errorf("requested defaults = %+v", rc)
	}
	sc := ae.SupportedContexts()
	if len(sc) != 1 || len(sc[0].TransferSyntaxes) != 1 || sc[0].TransferSyntaxes[0] != godicom.ImplicitVRLittleEndian {
		t.Errorf("supported explicit = %+v", sc)
	}
}

// spyTransport records Dial/Listen calls and returns configured errors,
// verifying the AE depends on the injected Transport (Dependency Inversion).
type spyTransport struct {
	dialAddr  string
	dialErr   error
	listenErr error
}

func (s *spyTransport) Dial(addr string) (net.Conn, error) {
	s.dialAddr = addr
	return nil, s.dialErr
}
func (s *spyTransport) Listen(addr string) (net.Listener, error) { return nil, s.listenErr }

func TestTransportInjectionDial(t *testing.T) {
	boom := errors.New("dial boom")
	spy := &spyTransport{dialErr: boom}
	ae := godicom.NewAE("SCU", godicom.WithTransport(spy))
	ae.AddRequestedContext(godicom.VerificationSOPClass)

	_, err := ae.Associate("example:1234")
	if !errors.Is(err, boom) {
		t.Errorf("Associate error = %v, want injected boom", err)
	}
	if spy.dialAddr != "example:1234" {
		t.Errorf("dial addr = %q", spy.dialAddr)
	}
}

func TestTransportInjectionListen(t *testing.T) {
	boom := errors.New("listen boom")
	ae := godicom.NewAE("SCP", godicom.WithTransport(&spyTransport{listenErr: boom}))
	if _, err := ae.StartServer(":0", nil); !errors.Is(err, boom) {
		t.Errorf("StartServer error = %v, want injected boom", err)
	}
}

func TestAssociateDialFailure(t *testing.T) {
	ae := godicom.NewAE("SCU")
	ae.AddRequestedContext(godicom.VerificationSOPClass)
	// Port 1 on localhost should refuse quickly.
	if _, err := ae.Associate("127.0.0.1:1"); err == nil {
		t.Error("associate to closed port should fail")
	}
}

func TestServerLifecycleAndEcho(t *testing.T) {
	scp := godicom.NewAE("ECHO_SCP")
	scp.AddSupportedContext(godicom.VerificationSOPClass)
	srv, err := scp.StartServer("127.0.0.1:0", []godicom.HandlerBinding{
		{Event: godicom.EvtCEcho, Handle: func(*godicom.Event) godicom.Status { return godicom.StatusSuccess }},
	})
	if err != nil {
		t.Fatal(err)
	}
	addr := srv.Addr().String()

	scu := godicom.NewAE("ECHO_SCU")
	scu.AddRequestedContext(godicom.VerificationSOPClass)
	assoc, err := scu.Associate(addr)
	if err != nil {
		t.Fatalf("associate: %v", err)
	}
	status, err := assoc.SendCEcho()
	if err != nil || !status.IsSuccess() {
		t.Errorf("c-echo: status=%#x err=%v", uint16(status), err)
	}
	assoc.Release()

	// Shutdown should stop accepting new associations.
	srv.Shutdown()
	scu2 := godicom.NewAE("ECHO_SCU")
	scu2.AddRequestedContext(godicom.VerificationSOPClass)
	if _, err := scu2.Associate(addr); err == nil {
		t.Error("associate after shutdown should fail")
	}
}

func TestHandlerReturnedFailureStatus(t *testing.T) {
	const failure = godicom.Status(0xC000) // Cannot understand

	scp := godicom.NewAE("SCP")
	scp.AddSupportedContext(godicom.VerificationSOPClass)
	srv, _ := scp.StartServer("127.0.0.1:0", []godicom.HandlerBinding{
		{Event: godicom.EvtCEcho, Handle: func(*godicom.Event) godicom.Status { return failure }},
	})
	defer srv.Shutdown()

	scu := godicom.NewAE("SCU")
	scu.AddRequestedContext(godicom.VerificationSOPClass)
	assoc, err := scu.Associate(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer assoc.Release()

	status, err := assoc.SendCEcho()
	if err != nil {
		t.Fatalf("c-echo: %v", err)
	}
	if status.IsSuccess() {
		t.Error("expected failure status from handler")
	}
	if status != failure {
		t.Errorf("status = %#x, want %#x", uint16(status), uint16(failure))
	}
}

func TestRequireCalledAETRejection(t *testing.T) {
	scp := godicom.NewAE("REAL_SCP", godicom.RequireCalledAETitle())
	scp.AddSupportedContext(godicom.VerificationSOPClass)
	srv, _ := scp.StartServer("127.0.0.1:0", []godicom.HandlerBinding{
		{Event: godicom.EvtCEcho, Handle: func(*godicom.Event) godicom.Status { return godicom.StatusSuccess }},
	})
	defer srv.Shutdown()

	scu := godicom.NewAE("SCU")
	scu.AddRequestedContext(godicom.VerificationSOPClass)
	// Associate defaults the called AE title to "ANY-SCP", which != "REAL_SCP".
	if _, err := scu.Associate(srv.Addr().String()); err == nil {
		t.Error("expected rejection for mismatched called AE title")
	}
}

func TestSendCEchoNoContextError(t *testing.T) {
	scp := godicom.NewAE("SCP")
	scp.AddSupportedContext("1.2.840.10008.5.1.4.1.1.2") // CT Storage only
	srv, _ := scp.StartServer("127.0.0.1:0", nil)
	defer srv.Shutdown()

	scu := godicom.NewAE("SCU")
	scu.AddRequestedContext(godicom.VerificationSOPClass) // not supported by SCP
	assoc, err := scu.Associate(srv.Addr().String())
	if err != nil {
		t.Fatalf("associate: %v", err)
	}
	defer assoc.Release()
	if _, err := assoc.SendCEcho(); err == nil {
		t.Error("SendCEcho should fail with no accepted Verification context")
	}
}

func TestVersionFormat(t *testing.T) {
	if !regexp.MustCompile(`^\d+\.\d+\.\d+$`).MatchString(godicom.Version) {
		t.Errorf("Version %q is not semver-formatted", godicom.Version)
	}
}

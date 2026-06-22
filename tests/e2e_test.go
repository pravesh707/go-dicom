// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"bytes"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	godicom "github.com/pravesh707/go-dicom"
	"github.com/pravesh707/go-dicom/dicom"
)

// TestE2ELargeStoreFragmentation stores a data set whose pixel data is far
// larger than the peer's maximum PDU length, forcing the command and data set
// to be split across many P-DATA-TF PDUs, and verifies the SCP reassembles the
// full data set byte-for-byte.
func TestE2ELargeStoreFragmentation(t *testing.T) {
	pixels := make([]byte, 60000)
	for i := range pixels {
		pixels[i] = byte(i * 7)
	}

	var mu sync.Mutex
	var got *dicom.DataSet
	// A deliberately small MaximumLength forces fragmentation (the SCU honours
	// the peer's advertised maximum).
	scp := godicom.NewAE("SCP", godicom.WithMaximumLength(512))
	scp.AddSupportedContext(scImageStorage)
	srv, err := scp.StartServer("127.0.0.1:0", []godicom.HandlerBinding{
		{Event: godicom.EvtCStore, Handle: func(e *godicom.Event) godicom.Status {
			mu.Lock()
			got = e.DataSet
			mu.Unlock()
			return godicom.StatusSuccess
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	scu := godicom.NewAE("SCU")
	scu.AddRequestedContext(scImageStorage)
	assoc, err := scu.Associate(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer assoc.Release()

	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUI(dicom.TagSOPClassUID, scImageStorage))
	ds.Set(dicom.NewUI(dicom.TagSOPInstanceUID, "1.2.3.frag"))
	ds.Set(dicom.NewElement(dicom.NewTag(0x7FE0, 0x0010), dicom.VROW, pixels))

	status, err := assoc.SendCStore(ds)
	if err != nil || !status.IsSuccess() {
		t.Fatalf("c-store: status=%#x err=%v", uint16(status), err)
	}

	mu.Lock()
	defer mu.Unlock()
	if got == nil {
		t.Fatal("no data set received")
	}
	px, ok := got.Get(dicom.NewTag(0x7FE0, 0x0010))
	if !ok {
		t.Fatal("pixel data missing after reassembly")
	}
	if !bytes.Equal(px.Raw, pixels) {
		t.Errorf("reassembled pixel data differs: got %d bytes, want %d", len(px.Raw), len(pixels))
	}
}

// TestE2EFileRoundTrip exercises the whole pipeline: build a Part-10 file, read
// it back, C-STORE it to an SCP that writes the received instance to a new
// Part-10 file, then re-read that file and confirm the attributes survived.
func TestE2EFileRoundTrip(t *testing.T) {
	dir := t.TempDir()
	srcPath := filepath.Join(dir, "src.dcm")

	src := dicom.NewDataSet()
	src.Set(dicom.NewUI(dicom.TagSOPClassUID, scImageStorage))
	src.Set(dicom.NewUI(dicom.TagSOPInstanceUID, "1.2.3.4.5.6.7"))
	src.Set(dicom.NewString(dicom.TagPatientName, dicom.VRPN, "Round^Trip"))
	src.Set(dicom.NewString(dicom.TagPatientID, dicom.VRLO, "PID-RT"))
	src.Set(dicom.NewUS(dicom.TagRows, 8))
	src.Set(dicom.NewUS(dicom.TagColumns, 8))
	src.Set(dicom.NewElement(dicom.NewTag(0x7FE0, 0x0010), dicom.VROW, bytes.Repeat([]byte{0xCD}, 128)))
	if err := godicom.NewFile(src, godicom.ExplicitVRLittleEndian).WriteFile(srcPath); err != nil {
		t.Fatal(err)
	}

	// SCP writes each received instance to outDir/<SOPInstanceUID>.dcm.
	outDir := t.TempDir()
	scp := godicom.NewAE("STORE_SCP")
	scp.AddSupportedContext(scImageStorage)
	srv, err := scp.StartServer("127.0.0.1:0", []godicom.HandlerBinding{
		{Event: godicom.EvtCStore, Handle: func(e *godicom.Event) godicom.Status {
			f := godicom.NewFile(e.DataSet, e.Context.TransferSyntax)
			if err := f.WriteFile(filepath.Join(outDir, f.SOPInstanceUID()+".dcm")); err != nil {
				return godicom.Status(0xA700)
			}
			return godicom.StatusSuccess
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	// SCU reads the source file and stores it.
	f, err := godicom.ReadFile(srcPath)
	if err != nil {
		t.Fatal(err)
	}
	scu := godicom.NewAE("STORE_SCU")
	scu.AddRequestedContext(f.SOPClassUID(), f.TransferSyntax())
	assoc, err := scu.Associate(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	if status, err := assoc.SendCStore(f.DataSet); err != nil || !status.IsSuccess() {
		t.Fatalf("c-store: status=%#x err=%v", uint16(status), err)
	}
	assoc.Release()

	// Re-read what the SCP wrote and compare.
	got, err := godicom.ReadFile(filepath.Join(outDir, "1.2.3.4.5.6.7.dcm"))
	if err != nil {
		t.Fatalf("re-read stored file: %v", err)
	}
	if name, _ := got.DataSet.GetString(dicom.TagPatientName); name != "Round^Trip" {
		t.Errorf("patient name = %q", name)
	}
	if id, _ := got.DataSet.GetString(dicom.TagPatientID); id != "PID-RT" {
		t.Errorf("patient id = %q", id)
	}
	if rows, _ := got.DataSet.GetUint16(dicom.TagRows); rows != 8 {
		t.Errorf("rows = %d", rows)
	}
	if px, ok := got.DataSet.Get(dicom.NewTag(0x7FE0, 0x0010)); !ok || !bytes.Equal(px.Raw, bytes.Repeat([]byte{0xCD}, 128)) {
		t.Error("pixel data not preserved through the full pipeline")
	}
}

// TestE2EBatchStore sends several instances over a single association.
func TestE2EBatchStore(t *testing.T) {
	var mu sync.Mutex
	received := map[string]bool{}
	scp := godicom.NewAE("SCP")
	scp.AddSupportedContext(scImageStorage)
	srv, err := scp.StartServer("127.0.0.1:0", []godicom.HandlerBinding{
		{Event: godicom.EvtCStore, Handle: func(e *godicom.Event) godicom.Status {
			uid, _ := e.DataSet.GetString(dicom.TagSOPInstanceUID)
			mu.Lock()
			received[uid] = true
			mu.Unlock()
			return godicom.StatusSuccess
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	scu := godicom.NewAE("SCU")
	scu.AddRequestedContext(scImageStorage)
	assoc, err := scu.Associate(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer assoc.Release()

	const n = 5
	for i := 0; i < n; i++ {
		ds := makeInstance(fmt.Sprintf("1.2.3.batch.%d", i))
		if status, err := assoc.SendCStore(ds); err != nil || !status.IsSuccess() {
			t.Fatalf("store %d: status=%#x err=%v", i, uint16(status), err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(received) != n {
		t.Errorf("received %d distinct instances, want %d", len(received), n)
	}
}

// TestE2EMultipleContextsOneAssociation negotiates two abstract syntaxes and
// uses both (C-ECHO and C-STORE) over the same association.
func TestE2EMultipleContextsOneAssociation(t *testing.T) {
	var stored bool
	scp := godicom.NewAE("SCP")
	scp.AddSupportedContext(godicom.VerificationSOPClass)
	scp.AddSupportedContext(scImageStorage)
	srv, err := scp.StartServer("127.0.0.1:0", []godicom.HandlerBinding{
		{Event: godicom.EvtCEcho, Handle: func(*godicom.Event) godicom.Status { return godicom.StatusSuccess }},
		{Event: godicom.EvtCStore, Handle: func(*godicom.Event) godicom.Status { stored = true; return godicom.StatusSuccess }},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	scu := godicom.NewAE("SCU")
	scu.AddRequestedContext(godicom.VerificationSOPClass)
	scu.AddRequestedContext(scImageStorage)
	assoc, err := scu.Associate(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer assoc.Release()

	if status, err := assoc.SendCEcho(); err != nil || !status.IsSuccess() {
		t.Fatalf("c-echo on multi-context association: %v", err)
	}
	if status, err := assoc.SendCStore(makeInstance("1.2.3.multi")); err != nil || !status.IsSuccess() {
		t.Fatalf("c-store on multi-context association: %v", err)
	}
	if !stored {
		t.Error("store handler not invoked")
	}
}

// TestE2EFindWithMatching verifies a C-FIND SCP that filters by query keys.
func TestE2EFindWithMatching(t *testing.T) {
	model := godicom.StudyRootQueryRetrieveFind
	people := map[string]string{"Doe^Jane": "1.1", "Roe^John": "1.2", "Poe^Ann": "1.3"}

	scp := godicom.NewAE("FIND_SCP")
	scp.AddSupportedContext(model)
	srv, err := scp.StartServer("127.0.0.1:0", []godicom.HandlerBinding{
		{Event: godicom.EvtCFind, Handle: func(e *godicom.Event) godicom.Status {
			want, _ := e.DataSet.GetString(dicom.TagPatientName)
			for name, uid := range people {
				if want != "" && want != "*" && name != want {
					continue // honour the PatientName query key
				}
				m := dicom.NewDataSet()
				m.Set(dicom.NewString(dicom.TagQueryRetrieveLevel, dicom.VRCS, "STUDY"))
				m.Set(dicom.NewString(dicom.TagPatientName, dicom.VRPN, name))
				m.Set(dicom.NewUI(dicom.TagStudyInstanceUID, uid))
				if err := e.Yield(m); err != nil {
					return godicom.Status(0xC000)
				}
			}
			return godicom.StatusSuccess
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	scu := godicom.NewAE("FIND_SCU")
	scu.AddRequestedContext(model)
	assoc, err := scu.Associate(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer assoc.Release()

	// Specific match.
	q := dicom.NewDataSet()
	q.Set(dicom.NewString(dicom.TagQueryRetrieveLevel, dicom.VRCS, "STUDY"))
	q.Set(dicom.NewString(dicom.TagPatientName, dicom.VRPN, "Roe^John"))
	matches, status, err := assoc.SendCFind(model, q)
	if err != nil || !status.IsSuccess() {
		t.Fatalf("c-find specific: %v", err)
	}
	if len(matches) != 1 {
		t.Fatalf("specific query: got %d matches, want 1", len(matches))
	}
	if name, _ := matches[0].GetString(dicom.TagPatientName); name != "Roe^John" {
		t.Errorf("matched the wrong patient: %q", name)
	}

	// Wildcard match.
	q2 := dicom.NewDataSet()
	q2.Set(dicom.NewString(dicom.TagQueryRetrieveLevel, dicom.VRCS, "STUDY"))
	q2.Set(dicom.NewString(dicom.TagPatientName, dicom.VRPN, "*"))
	all, _, err := assoc.SendCFind(model, q2)
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != len(people) {
		t.Errorf("wildcard query: got %d matches, want %d", len(all), len(people))
	}
}

// TestE2EShutdownWithActiveAssociation confirms Shutdown force-closes a still
// open association instead of hanging forever.
func TestE2EShutdownWithActiveAssociation(t *testing.T) {
	scp := godicom.NewAE("SCP")
	scp.AddSupportedContext(godicom.VerificationSOPClass)
	srv, err := scp.StartServer("127.0.0.1:0", []godicom.HandlerBinding{
		{Event: godicom.EvtCEcho, Handle: func(*godicom.Event) godicom.Status { return godicom.StatusSuccess }},
	})
	if err != nil {
		t.Fatal(err)
	}

	scu := godicom.NewAE("SCU")
	scu.AddRequestedContext(godicom.VerificationSOPClass)
	assoc, err := scu.Associate(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	// Intentionally do NOT release: the SCP's serve loop is now blocked reading.
	defer assoc.Close()

	done := make(chan error, 1)
	go func() { done <- srv.Shutdown() }()
	select {
	case <-done: // returned promptly — good
	case <-time.After(5 * time.Second):
		t.Fatal("Shutdown hung with an active association")
	}
}

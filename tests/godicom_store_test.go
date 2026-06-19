// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"sync"
	"testing"

	godicom "github.com/pravesh707/go-dicom"
	"github.com/pravesh707/go-dicom/dicom"
)

const scImageStorage = "1.2.840.10008.5.1.4.1.1.7" // Secondary Capture Image Storage

func TestCStoreInProcess(t *testing.T) {
	scp := godicom.NewAE("STORE_SCP")
	scp.AddSupportedContext(scImageStorage)

	var mu sync.Mutex
	var received *dicom.DataSet
	var receivedSOP string
	srv, err := scp.StartServer("127.0.0.1:0", []godicom.HandlerBinding{
		{Event: godicom.EvtCStore, Handle: func(e *godicom.Event) godicom.Status {
			mu.Lock()
			received = e.DataSet
			if r, ok := e.Request.(interface{ GetMessageID() uint16 }); ok && r.GetMessageID() == 0 {
				t.Error("store request should have a non-zero message id")
			}
			mu.Unlock()
			return godicom.StatusSuccess
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()
	_ = receivedSOP

	scu := godicom.NewAE("STORE_SCU")
	scu.AddRequestedContext(scImageStorage)
	assoc, err := scu.Associate(srv.Addr().String())
	if err != nil {
		t.Fatalf("associate: %v", err)
	}

	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUI(dicom.TagSOPClassUID, scImageStorage))
	ds.Set(dicom.NewUI(dicom.TagSOPInstanceUID, "1.2.3.4.5.6"))
	ds.Set(dicom.NewString(dicom.TagPatientName, dicom.VRPN, "Doe^Jane"))
	ds.Set(dicom.NewUS(dicom.TagRows, 64))

	status, err := assoc.SendCStore(ds)
	if err != nil {
		t.Fatalf("c-store: %v", err)
	}
	if !status.IsSuccess() {
		t.Errorf("c-store status = %#x", uint16(status))
	}
	assoc.Release()

	mu.Lock()
	defer mu.Unlock()
	if received == nil {
		t.Fatal("handler did not receive a data set")
	}
	if name, _ := received.GetString(dicom.TagPatientName); name != "Doe^Jane" {
		t.Errorf("received patient name = %q", name)
	}
	if rows, _ := received.GetUint16(dicom.TagRows); rows != 64 {
		t.Errorf("received rows = %d", rows)
	}
}

func TestCStoreNoSOPClassError(t *testing.T) {
	scp := godicom.NewAE("STORE_SCP")
	scp.AddSupportedContext(scImageStorage)
	srv, _ := scp.StartServer("127.0.0.1:0", []godicom.HandlerBinding{
		{Event: godicom.EvtCStore, Handle: func(*godicom.Event) godicom.Status { return godicom.StatusSuccess }},
	})
	defer srv.Shutdown()

	scu := godicom.NewAE("STORE_SCU")
	scu.AddRequestedContext(scImageStorage)
	assoc, err := scu.Associate(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer assoc.Release()

	// Data set without (0008,0016) SOP Class UID.
	ds := dicom.NewDataSet()
	ds.Set(dicom.NewString(dicom.TagPatientName, dicom.VRPN, "X"))
	if _, err := assoc.SendCStore(ds); err == nil {
		t.Error("expected error for data set missing SOP Class UID")
	}
}

func TestCStoreNoHandlerRefused(t *testing.T) {
	scp := godicom.NewAE("STORE_SCP")
	scp.AddSupportedContext(scImageStorage)
	srv, _ := scp.StartServer("127.0.0.1:0", nil) // no C-STORE handler
	defer srv.Shutdown()

	scu := godicom.NewAE("STORE_SCU")
	scu.AddRequestedContext(scImageStorage)
	assoc, err := scu.Associate(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer assoc.Release()

	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUI(dicom.TagSOPClassUID, scImageStorage))
	ds.Set(dicom.NewUI(dicom.TagSOPInstanceUID, "1.2.3"))
	status, err := assoc.SendCStore(ds)
	if err != nil {
		t.Fatalf("c-store: %v", err)
	}
	if status.IsSuccess() {
		t.Error("expected non-success when no store handler is bound")
	}
}

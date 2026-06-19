// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"testing"

	godicom "github.com/pravesh707/go-dicom"
	"github.com/pravesh707/go-dicom/dicom"
	"github.com/pravesh707/go-dicom/dimse"
)

func TestNServicesInProcess(t *testing.T) {
	const printerSOP = "1.2.840.10008.5.1.1.16" // Printer SOP Class
	printerStatus := dicom.NewTag(0x2110, 0x0010)

	scp := godicom.NewAE("N_SCP")
	scp.AddSupportedContext(printerSOP)
	srv, err := scp.StartServer("127.0.0.1:0", []godicom.HandlerBinding{
		{Event: godicom.EvtNGet, Handle: func(e *godicom.Event) godicom.Status {
			attrs := dicom.NewDataSet()
			attrs.Set(dicom.NewString(printerStatus, dicom.VRCS, "NORMAL"))
			e.SetResponse(attrs)
			return godicom.StatusSuccess
		}},
		{Event: godicom.EvtNAction, Handle: func(e *godicom.Event) godicom.Status {
			if r, ok := e.Request.(*dimse.NRequest); !ok || r.ActionTypeID != 3 {
				t.Errorf("N-ACTION action type wrong: %#v", e.Request)
			}
			return godicom.StatusSuccess
		}},
		{Event: godicom.EvtNCreate, Handle: func(e *godicom.Event) godicom.Status {
			e.SetResponse(e.DataSet) // echo the created attributes back
			return godicom.StatusSuccess
		}},
		{Event: godicom.EvtNEventReport, Handle: func(e *godicom.Event) godicom.Status {
			if r, ok := e.Request.(*dimse.NRequest); !ok || r.EventTypeID != 1 {
				t.Errorf("N-EVENT-REPORT event type wrong: %#v", e.Request)
			}
			return godicom.StatusSuccess
		}},
		{Event: godicom.EvtNDelete, Handle: func(*godicom.Event) godicom.Status { return godicom.StatusSuccess }},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	scu := godicom.NewAE("N_SCU")
	scu.AddRequestedContext(printerSOP)
	assoc, err := scu.Associate(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer assoc.Release()

	// N-GET returns an attribute list.
	rsp, err := assoc.SendNGet(printerSOP, "1.2.3.4")
	if err != nil || !rsp.Status.IsSuccess() {
		t.Fatalf("N-GET: status=%#x err=%v", uint16(rsp.Status), err)
	}
	if rsp.DataSet == nil {
		t.Fatal("N-GET response missing data set")
	}
	if s, _ := rsp.DataSet.GetString(printerStatus); s != "NORMAL" {
		t.Errorf("PrinterStatus = %q, want NORMAL", s)
	}

	// N-ACTION with an action type.
	if rsp, err := assoc.SendNAction(printerSOP, "1.2.3.4", 3, nil); err != nil || !rsp.Status.IsSuccess() {
		t.Errorf("N-ACTION: status=%#x err=%v", uint16(rsp.Status), err)
	}

	// N-CREATE echoes the attribute list.
	attrs := dicom.NewDataSet()
	attrs.Set(dicom.NewString(dicom.TagPatientName, dicom.VRPN, "Doe^Jane"))
	rsp, err = assoc.SendNCreate(printerSOP, "1.2.3.5", attrs)
	if err != nil || !rsp.Status.IsSuccess() {
		t.Fatalf("N-CREATE: status=%#x err=%v", uint16(rsp.Status), err)
	}
	if name, _ := rsp.DataSet.GetString(dicom.TagPatientName); name != "Doe^Jane" {
		t.Errorf("N-CREATE echoed name = %q", name)
	}

	// N-EVENT-REPORT with an event type and info.
	info := dicom.NewDataSet()
	info.Set(dicom.NewString(dicom.TagPatientID, dicom.VRLO, "PID"))
	if rsp, err := assoc.SendNEventReport(printerSOP, "1.2.3.4", 1, info); err != nil || !rsp.Status.IsSuccess() {
		t.Errorf("N-EVENT-REPORT: status=%#x err=%v", uint16(rsp.Status), err)
	}

	// N-DELETE.
	if rsp, err := assoc.SendNDelete(printerSOP, "1.2.3.4"); err != nil || !rsp.Status.IsSuccess() {
		t.Errorf("N-DELETE: status=%#x err=%v", uint16(rsp.Status), err)
	}
}

func TestNServiceUnhandled(t *testing.T) {
	const printerSOP = "1.2.840.10008.5.1.1.16"
	scp := godicom.NewAE("N_SCP")
	scp.AddSupportedContext(printerSOP)
	srv, _ := scp.StartServer("127.0.0.1:0", nil) // no N handlers
	defer srv.Shutdown()

	scu := godicom.NewAE("N_SCU")
	scu.AddRequestedContext(printerSOP)
	assoc, err := scu.Associate(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer assoc.Release()

	rsp, err := assoc.SendNGet(printerSOP, "1.2.3")
	if err != nil {
		t.Fatalf("N-GET: %v", err)
	}
	if rsp.Status.IsSuccess() {
		t.Error("expected non-success when no N-GET handler is bound")
	}
}

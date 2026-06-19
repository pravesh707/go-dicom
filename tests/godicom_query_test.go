// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"fmt"
	"testing"

	godicom "github.com/pravesh707/go-dicom"
	"github.com/pravesh707/go-dicom/dicom"
)

func TestCFindInProcess(t *testing.T) {
	model := godicom.StudyRootQueryRetrieveFind

	scp := godicom.NewAE("FIND_SCP")
	scp.AddSupportedContext(model)
	srv, err := scp.StartServer("127.0.0.1:0", []godicom.HandlerBinding{
		{Event: godicom.EvtCFind, Handle: func(e *godicom.Event) godicom.Status {
			for i, name := range []string{"Doe^Jane", "Roe^John"} {
				m := dicom.NewDataSet()
				m.Set(dicom.NewString(dicom.TagQueryRetrieveLevel, dicom.VRCS, "STUDY"))
				m.Set(dicom.NewString(dicom.TagPatientName, dicom.VRPN, name))
				m.Set(dicom.NewUI(dicom.TagStudyInstanceUID, fmt.Sprintf("1.2.3.%d", i)))
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

	query := dicom.NewDataSet()
	query.Set(dicom.NewString(dicom.TagQueryRetrieveLevel, dicom.VRCS, "STUDY"))
	query.Set(dicom.NewString(dicom.TagPatientName, dicom.VRPN, "*"))

	matches, status, err := assoc.SendCFind(model, query)
	if err != nil {
		t.Fatalf("c-find: %v", err)
	}
	if !status.IsSuccess() {
		t.Errorf("final status = %#x", uint16(status))
	}
	if len(matches) != 2 {
		t.Fatalf("got %d matches, want 2", len(matches))
	}
	if name, _ := matches[0].GetString(dicom.TagPatientName); name != "Doe^Jane" {
		t.Errorf("match[0] patient = %q", name)
	}
	if uid, _ := matches[1].GetString(dicom.TagStudyInstanceUID); uid != "1.2.3.1" {
		t.Errorf("match[1] study uid = %q", uid)
	}
}

func TestCFindNoHandlerEmptyResult(t *testing.T) {
	model := godicom.StudyRootQueryRetrieveFind
	scp := godicom.NewAE("FIND_SCP")
	scp.AddSupportedContext(model)
	srv, _ := scp.StartServer("127.0.0.1:0", nil) // no C-FIND handler
	defer srv.Shutdown()

	scu := godicom.NewAE("FIND_SCU")
	scu.AddRequestedContext(model)
	assoc, err := scu.Associate(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer assoc.Release()

	query := dicom.NewDataSet()
	query.Set(dicom.NewString(dicom.TagQueryRetrieveLevel, dicom.VRCS, "STUDY"))
	matches, status, err := assoc.SendCFind(model, query)
	if err != nil {
		t.Fatalf("c-find: %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("expected 0 matches, got %d", len(matches))
	}
	if !status.IsSuccess() {
		t.Errorf("final status = %#x", uint16(status))
	}
}

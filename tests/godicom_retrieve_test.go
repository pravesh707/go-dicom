// SPDX-License-Identifier: Apache-2.0

package tests

import (
	"sync"
	"testing"

	godicom "github.com/pravesh707/go-dicom"
	"github.com/pravesh707/go-dicom/dicom"
)

func makeInstance(uid string) *dicom.DataSet {
	ds := dicom.NewDataSet()
	ds.Set(dicom.NewUI(dicom.TagSOPClassUID, scImageStorage))
	ds.Set(dicom.NewUI(dicom.TagSOPInstanceUID, uid))
	ds.Set(dicom.NewString(dicom.TagPatientName, dicom.VRPN, "Doe^Jane"))
	return ds
}

func TestCGetInProcess(t *testing.T) {
	model := godicom.StudyRootQueryRetrieveGet

	scp := godicom.NewAE("GET_SCP")
	scp.AddSupportedContext(model)
	scp.AddSupportedContext(scImageStorage)
	instances := []*dicom.DataSet{makeInstance("1.1"), makeInstance("1.2"), makeInstance("1.3")}
	srv, err := scp.StartServer("127.0.0.1:0", []godicom.HandlerBinding{
		{Event: godicom.EvtCGet, Handle: func(e *godicom.Event) godicom.Status {
			for _, inst := range instances {
				if err := e.Yield(inst); err != nil {
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

	scu := godicom.NewAE("GET_SCU")
	scu.AddRequestedContext(model)
	scu.AddRequestedContext(scImageStorage)
	assoc, err := scu.Associate(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer assoc.Release()

	var got []string
	query := dicom.NewDataSet()
	query.Set(dicom.NewString(dicom.TagQueryRetrieveLevel, dicom.VRCS, "STUDY"))
	res, err := assoc.SendCGet(model, query, func(ds *dicom.DataSet) godicom.Status {
		uid, _ := ds.GetString(dicom.TagSOPInstanceUID)
		got = append(got, uid)
		return godicom.StatusSuccess
	})
	if err != nil {
		t.Fatalf("c-get: %v", err)
	}
	if !res.Status.IsSuccess() {
		t.Errorf("final status = %#x", uint16(res.Status))
	}
	if res.Completed != 3 || res.Failed != 0 {
		t.Errorf("sub-ops completed=%d failed=%d, want 3/0", res.Completed, res.Failed)
	}
	if len(got) != 3 || got[0] != "1.1" || got[2] != "1.3" {
		t.Errorf("received instances = %v", got)
	}
}

// TestCGetWithScpRoleNegotiation exercises the public C-GET path with SCP/SCU
// Role Selection on the storage context — the negotiation a standards-strict
// C-GET SCP (e.g. pynetdicom) requires before it will return instances.
func TestCGetWithScpRoleNegotiation(t *testing.T) {
	model := godicom.StudyRootQueryRetrieveGet

	scp := godicom.NewAE("GET_SCP")
	scp.AddSupportedContext(model)
	scp.AddSupportedContext(scImageStorage)
	srv, err := scp.StartServer("127.0.0.1:0", []godicom.HandlerBinding{
		{Event: godicom.EvtCGet, Handle: func(e *godicom.Event) godicom.Status {
			if err := e.Yield(makeInstance("2.1")); err != nil {
				return godicom.Status(0xC000)
			}
			return godicom.StatusSuccess
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	scu := godicom.NewAE("GET_SCU")
	scu.AddRequestedContext(model)
	// Propose SCP role for the storage context so the peer may store back to us.
	scu.AddRequestedContextWithRole(scImageStorage, false, true)
	assoc, err := scu.Associate(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer assoc.Release()

	var got []string
	query := dicom.NewDataSet()
	query.Set(dicom.NewString(dicom.TagQueryRetrieveLevel, dicom.VRCS, "STUDY"))
	res, err := assoc.SendCGet(model, query, func(ds *dicom.DataSet) godicom.Status {
		uid, _ := ds.GetString(dicom.TagSOPInstanceUID)
		got = append(got, uid)
		return godicom.StatusSuccess
	})
	if err != nil {
		t.Fatalf("c-get: %v", err)
	}
	if !res.Status.IsSuccess() || res.Completed != 1 || res.Failed != 0 {
		t.Errorf("result status=%#x completed=%d failed=%d, want success 1/0", uint16(res.Status), res.Completed, res.Failed)
	}
	if len(got) != 1 || got[0] != "2.1" {
		t.Errorf("received instances = %v, want [2.1]", got)
	}
}

func TestCMoveInProcess(t *testing.T) {
	// Destination Storage SCP that records received instances.
	var mu sync.Mutex
	var stored []string
	dest := godicom.NewAE("DEST_SCP")
	dest.AddSupportedContext(scImageStorage)
	destSrv, err := dest.StartServer("127.0.0.1:0", []godicom.HandlerBinding{
		{Event: godicom.EvtCStore, Handle: func(e *godicom.Event) godicom.Status {
			uid, _ := e.DataSet.GetString(dicom.TagSOPInstanceUID)
			mu.Lock()
			stored = append(stored, uid)
			mu.Unlock()
			return godicom.StatusSuccess
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer destSrv.Shutdown()

	// Move SCP (Q/R provider) that forwards matched instances to the destination.
	model := godicom.StudyRootQueryRetrieveMove
	moveSCP := godicom.NewAE("MOVE_SCP")
	moveSCP.AddSupportedContext(model)
	moveSCP.AddSupportedContext(scImageStorage)
	moveSCP.AddMoveDestination("DEST_SCP", destSrv.Addr().String())
	instances := []*dicom.DataSet{makeInstance("2.1"), makeInstance("2.2")}
	moveSrv, err := moveSCP.StartServer("127.0.0.1:0", []godicom.HandlerBinding{
		{Event: godicom.EvtCMove, Handle: func(e *godicom.Event) godicom.Status {
			for _, inst := range instances {
				if err := e.Yield(inst); err != nil {
					return godicom.Status(0xC000)
				}
			}
			return godicom.StatusSuccess
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer moveSrv.Shutdown()

	// Move SCU.
	scu := godicom.NewAE("MOVE_SCU")
	scu.AddRequestedContext(model)
	assoc, err := scu.Associate(moveSrv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer assoc.Release()

	query := dicom.NewDataSet()
	query.Set(dicom.NewString(dicom.TagQueryRetrieveLevel, dicom.VRCS, "STUDY"))
	res, err := assoc.SendCMove(model, "DEST_SCP", query)
	if err != nil {
		t.Fatalf("c-move: %v", err)
	}
	if !res.Status.IsSuccess() {
		t.Errorf("final status = %#x", uint16(res.Status))
	}
	if res.Completed != 2 {
		t.Errorf("completed sub-ops = %d, want 2", res.Completed)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(stored) != 2 {
		t.Errorf("destination stored %d instances, want 2: %v", len(stored), stored)
	}
}

func TestCMoveUnknownDestination(t *testing.T) {
	model := godicom.StudyRootQueryRetrieveMove
	moveSCP := godicom.NewAE("MOVE_SCP")
	moveSCP.AddSupportedContext(model)
	srv, err := moveSCP.StartServer("127.0.0.1:0", []godicom.HandlerBinding{
		{Event: godicom.EvtCMove, Handle: func(e *godicom.Event) godicom.Status { return godicom.StatusSuccess }},
	})
	if err != nil {
		t.Fatal(err)
	}
	defer srv.Shutdown()

	scu := godicom.NewAE("MOVE_SCU")
	scu.AddRequestedContext(model)
	assoc, err := scu.Associate(srv.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer assoc.Release()

	query := dicom.NewDataSet()
	query.Set(dicom.NewString(dicom.TagQueryRetrieveLevel, dicom.VRCS, "STUDY"))
	res, err := assoc.SendCMove(model, "NOPE", query)
	if err != nil {
		t.Fatalf("c-move: %v", err)
	}
	if res.Status.IsSuccess() {
		t.Error("expected failure for unknown move destination")
	}
}

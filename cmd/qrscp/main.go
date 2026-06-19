// SPDX-License-Identifier: Apache-2.0

// Command qrscp is a minimal Query/Retrieve + Storage SCP. It loads the DICOM
// Part-10 files in a directory into memory and serves C-FIND, C-MOVE and C-GET
// over them (plus C-ECHO and C-STORE). C-MOVE forwards matched instances to a
// destination registered with -dest.
//
//	qrscp -addr :11112 -aet QR_SCP -dir ./store -dest DEST_SCP=127.0.0.1:11113
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"

	godicom "github.com/pravesh707/go-dicom"
	"github.com/pravesh707/go-dicom/dicom"
)

func main() {
	addr := flag.String("addr", ":11112", "listen address host:port")
	aet := flag.String("aet", "QR_SCP", "this SCP's AE title")
	dir := flag.String("dir", ".", "directory of .dcm files to serve")
	dest := flag.String("dest", "", "C-MOVE destination as AETITLE=host:port (repeatable, comma-separated)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("qrscp", godicom.VersionString(""))
		os.Exit(0)
	}

	instances := loadInstances(*dir)
	log.Printf("loaded %d instance(s) from %s", len(instances), *dir)

	ae := godicom.NewAE(*aet)
	ae.AddSupportedContext(godicom.VerificationSOPClass)
	for _, uid := range []string{
		godicom.StudyRootQueryRetrieveFind, godicom.StudyRootQueryRetrieveMove, godicom.StudyRootQueryRetrieveGet,
		godicom.PatientRootQueryRetrieveFind, godicom.PatientRootQueryRetrieveMove, godicom.PatientRootQueryRetrieveGet,
		"1.2.840.10008.5.1.4.1.1.7", "1.2.840.10008.5.1.4.1.1.2", "1.2.840.10008.5.1.4.1.1.4",
	} {
		ae.AddSupportedContext(uid)
	}
	for _, d := range strings.Split(*dest, ",") {
		if kv := strings.SplitN(d, "=", 2); len(kv) == 2 {
			ae.AddMoveDestination(kv[0], kv[1])
			log.Printf("C-MOVE destination %q -> %s", kv[0], kv[1])
		}
	}

	handlers := []godicom.HandlerBinding{
		{Event: godicom.EvtCEcho, Handle: func(*godicom.Event) godicom.Status { return godicom.StatusSuccess }},
		{Event: godicom.EvtCFind, Handle: func(e *godicom.Event) godicom.Status {
			for _, f := range instances {
				m := dicom.NewDataSet()
				m.Set(dicom.NewString(dicom.TagQueryRetrieveLevel, dicom.VRCS, "STUDY"))
				for _, tag := range []dicom.Tag{dicom.TagPatientName, dicom.TagPatientID, dicom.TagStudyInstanceUID, dicom.TagStudyDate, dicom.TagAccessionNumber, dicom.TagModality} {
					if el, ok := f.DataSet.Get(tag); ok {
						m.Set(el)
					}
				}
				if err := e.Yield(m); err != nil {
					return godicom.Status(0xC000)
				}
			}
			return godicom.StatusSuccess
		}},
		{Event: godicom.EvtCMove, Handle: yieldAll(instances)},
		{Event: godicom.EvtCGet, Handle: yieldAll(instances)},
		{Event: godicom.EvtCStore, Handle: func(e *godicom.Event) godicom.Status {
			log.Printf("stored instance from %q", e.Assoc.CallingAETitle)
			return godicom.StatusSuccess
		}},
	}

	srv, err := ae.StartServer(*addr, handlers)
	if err != nil {
		log.Fatalf("start server: %v", err)
	}
	log.Printf("qrscp listening on %s (AE title %q)", srv.Addr(), *aet)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	srv.Shutdown()
}

func yieldAll(instances []*godicom.File) godicom.Handler {
	return func(e *godicom.Event) godicom.Status {
		for _, f := range instances {
			if err := e.Yield(f.DataSet); err != nil {
				return godicom.Status(0xC000)
			}
		}
		return godicom.StatusSuccess
	}
}

func loadInstances(dir string) []*godicom.File {
	var out []*godicom.File
	entries, err := os.ReadDir(dir)
	if err != nil {
		log.Fatalf("read dir: %v", err)
	}
	for _, ent := range entries {
		if ent.IsDir() || !strings.HasSuffix(strings.ToLower(ent.Name()), ".dcm") {
			continue
		}
		f, err := godicom.ReadFile(filepath.Join(dir, ent.Name()))
		if err != nil {
			log.Printf("skip %s: %v", ent.Name(), err)
			continue
		}
		out = append(out, f)
	}
	return out
}

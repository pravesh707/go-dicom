// SPDX-License-Identifier: Apache-2.0

// Command findscu is a Query SCU: it issues a C-FIND against a remote Q/R SCP
// and prints the matching identifiers.
//
//	findscu -addr 127.0.0.1:11112 -called QR_SCP -level STUDY -patient-name "*"
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	godicom "github.com/pravesh707/go-dicom"
	"github.com/pravesh707/go-dicom/dicom"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:11112", "remote SCP address host:port")
	calling := flag.String("calling", "FIND_SCU", "calling AE title")
	called := flag.String("called", "ANY-SCP", "called AE title")
	model := flag.String("model", "study", "query model: study or patient")
	level := flag.String("level", "STUDY", "QueryRetrieveLevel (PATIENT/STUDY/SERIES/IMAGE)")
	patientName := flag.String("patient-name", "*", "PatientName query key")
	studyUID := flag.String("study-uid", "", "StudyInstanceUID query key")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("findscu", godicom.VersionString(""))
		os.Exit(0)
	}

	sopClass := godicom.StudyRootQueryRetrieveFind
	if *model == "patient" {
		sopClass = godicom.PatientRootQueryRetrieveFind
	}

	ae := godicom.NewAE(*calling)
	ae.AddRequestedContext(sopClass)

	assoc, err := ae.AssociateAs(*addr, *called)
	if err != nil {
		log.Fatalf("association failed: %v", err)
	}
	defer assoc.Release()

	// Build the query identifier: the level, the supplied keys, and a few
	// universal return keys (empty value = "return this attribute").
	q := dicom.NewDataSet()
	q.Set(dicom.NewString(dicom.TagQueryRetrieveLevel, dicom.VRCS, *level))
	q.Set(dicom.NewString(dicom.TagPatientName, dicom.VRPN, *patientName))
	q.Set(dicom.NewUI(dicom.TagStudyInstanceUID, *studyUID))
	q.Set(dicom.NewString(dicom.TagStudyDate, dicom.VRDA, ""))
	q.Set(dicom.NewString(dicom.TagAccessionNumber, dicom.VRSH, ""))

	matches, status, err := assoc.SendCFind(sopClass, q)
	if err != nil {
		log.Fatalf("C-FIND failed: %v", err)
	}
	log.Printf("C-FIND complete: %d match(es), final status 0x%04X (%s)", len(matches), uint16(status), status.Category())
	for i, m := range matches {
		name, _ := m.GetString(dicom.TagPatientName)
		study, _ := m.GetString(dicom.TagStudyInstanceUID)
		fmt.Printf("  [%d] PatientName=%q StudyInstanceUID=%q\n", i+1, name, study)
	}
}

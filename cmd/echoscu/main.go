// SPDX-License-Identifier: Apache-2.0

// Command echoscu is a Verification SCU: it associates with a remote SCP and
// sends a C-ECHO, the DICOM equivalent of a ping.
//
// Install:
//
//	go install ./cmd/echoscu                                      # local copy
//	go install github.com/pravesh707/go-dicom/cmd/echoscu@latest  # once published
//
// Run:
//
//	echoscu -addr 127.0.0.1:11112 -called STORE_SCP
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	godicom "github.com/pravesh707/go-dicom"
)

// commit is optionally injected at build time:
//
//	go build -ldflags "-X main.commit=$(git rev-parse --short HEAD)" ./cmd/echoscu
var commit = ""

func main() {
	addr := flag.String("addr", "127.0.0.1:11112", "remote SCP address host:port")
	calling := flag.String("calling", "ECHO_SCU", "calling AE title")
	called := flag.String("called", "ANY-SCP", "called AE title")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("echoscu", godicom.VersionString(commit))
		os.Exit(0)
	}

	ae := godicom.NewAE(*calling)
	ae.AddRequestedContext(godicom.VerificationSOPClass)

	assoc, err := ae.AssociateAs(*addr, *called)
	if err != nil {
		log.Fatalf("association failed: %v", err)
	}
	defer assoc.Release()

	status, err := assoc.SendCEcho()
	if err != nil {
		log.Fatalf("C-ECHO failed: %v", err)
	}
	log.Printf("C-ECHO response status: 0x%04X (%s)", uint16(status), status.Category())
}

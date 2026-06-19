// SPDX-License-Identifier: Apache-2.0

// Command echoscp is a Verification SCP: it listens for associations and
// answers C-ECHO requests with a Success status. Each association is served on
// its own goroutine.
//
// Install:
//
//	go install ./cmd/echoscp                                      # local copy
//	go install github.com/pravesh707/go-dicom/cmd/echoscp@latest  # once published
//
// Run:
//
//	echoscp -addr :11112 -aet ECHO_SCP
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	godicom "github.com/pravesh707/go-dicom"
)

// commit is optionally injected at build time via
// -ldflags "-X main.commit=$(git rev-parse --short HEAD)".
var commit = ""

func main() {
	addr := flag.String("addr", ":11112", "listen address host:port")
	aet := flag.String("aet", "ECHO_SCP", "this SCP's AE title")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("echoscp", godicom.VersionString(commit))
		os.Exit(0)
	}

	ae := godicom.NewAE(*aet)
	ae.AddSupportedContext(godicom.VerificationSOPClass)

	handlers := []godicom.HandlerBinding{
		{Event: godicom.EvtCEcho, Handle: func(e *godicom.Event) godicom.Status {
			log.Printf("C-ECHO from %q", e.Assoc.CallingAETitle)
			return godicom.StatusSuccess
		}},
	}

	srv, err := ae.StartServer(*addr, handlers)
	if err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
	log.Printf("echoscp listening on %s (AE title %q)", srv.Addr(), *aet)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	log.Println("shutting down...")
	srv.Shutdown()
}

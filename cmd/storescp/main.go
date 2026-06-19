// SPDX-License-Identifier: Apache-2.0

// Command storescp is a Storage SCP: it accepts associations for common Storage
// SOP Classes and writes each received instance to a directory as a Part-10
// file named <SOPInstanceUID>.dcm. Each association is served on its own
// goroutine.
//
//	storescp -addr :11112 -aet STORE_SCP -out ./received
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"

	godicom "github.com/pravesh707/go-dicom"
)

// storageSOPClasses is a representative set of Storage SOP Classes the SCP
// accepts. Extend as needed for other modalities.
var storageSOPClasses = []string{
	"1.2.840.10008.5.1.4.1.1.7",     // Secondary Capture Image Storage
	"1.2.840.10008.5.1.4.1.1.2",     // CT Image Storage
	"1.2.840.10008.5.1.4.1.1.2.1",   // Enhanced CT Image Storage
	"1.2.840.10008.5.1.4.1.1.4",     // MR Image Storage
	"1.2.840.10008.5.1.4.1.1.4.1",   // Enhanced MR Image Storage
	"1.2.840.10008.5.1.4.1.1.6.1",   // Ultrasound Image Storage
	"1.2.840.10008.5.1.4.1.1.1",     // Computed Radiography Image Storage
	"1.2.840.10008.5.1.4.1.1.1.1",   // Digital X-Ray Image Storage
	"1.2.840.10008.5.1.4.1.1.128",   // PET Image Storage
	"1.2.840.10008.5.1.4.1.1.20",    // NM Image Storage
	"1.2.840.10008.5.1.4.1.1.481.1", // RT Image Storage
	"1.2.840.10008.5.1.4.1.1.104.1", // Encapsulated PDF Storage
}

func main() {
	addr := flag.String("addr", ":11112", "listen address host:port")
	aet := flag.String("aet", "STORE_SCP", "this SCP's AE title")
	out := flag.String("out", ".", "directory to write received instances")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("storescp", godicom.VersionString(""))
		os.Exit(0)
	}
	if err := os.MkdirAll(*out, 0o755); err != nil {
		log.Fatalf("output dir: %v", err)
	}

	ae := godicom.NewAE(*aet)
	for _, uid := range storageSOPClasses {
		ae.AddSupportedContext(uid)
	}

	handlers := []godicom.HandlerBinding{
		{Event: godicom.EvtCStore, Handle: func(e *godicom.Event) godicom.Status {
			f := godicom.NewFile(e.DataSet, e.Context.TransferSyntax)
			name := f.SOPInstanceUID()
			if name == "" {
				name = "instance"
			}
			path := filepath.Join(*out, name+".dcm")
			if err := f.WriteFile(path); err != nil {
				log.Printf("failed to write %s: %v", path, err)
				return godicom.Status(0xA700) // out of resources
			}
			log.Printf("stored %s (%s) from %q", path, e.Context.AbstractSyntax, e.Assoc.CallingAETitle)
			return godicom.StatusSuccess
		}},
	}

	srv, err := ae.StartServer(*addr, handlers)
	if err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
	log.Printf("storescp listening on %s (AE title %q), writing to %s", srv.Addr(), *aet, *out)

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt)
	<-stop
	srv.Shutdown()
}

// SPDX-License-Identifier: Apache-2.0

// Command storescu is a Storage SCU: it reads one or more DICOM Part-10 files
// and sends each to a remote Storage SCP via C-STORE.
//
//	storescu -addr 127.0.0.1:11112 -called STORE_SCP image1.dcm image2.dcm
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	godicom "github.com/pravesh707/go-dicom"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:11112", "remote SCP address host:port")
	calling := flag.String("calling", "STORE_SCU", "calling AE title")
	called := flag.String("called", "ANY-SCP", "called AE title")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println("storescu", godicom.VersionString(""))
		os.Exit(0)
	}
	files := flag.Args()
	if len(files) == 0 {
		log.Fatal("usage: storescu [options] file.dcm ...")
	}

	// Read every file first so we can propose the right presentation contexts.
	type item struct {
		path string
		file *godicom.File
	}
	var items []item
	ae := godicom.NewAE(*calling)
	seen := map[string]bool{}
	for _, p := range files {
		f, err := godicom.ReadFile(p)
		if err != nil {
			log.Fatalf("read %s: %v", p, err)
		}
		items = append(items, item{p, f})
		// One requested context per (SOP class, transfer syntax) pair.
		key := f.SOPClassUID() + "|" + f.TransferSyntax()
		if !seen[key] {
			ae.AddRequestedContext(f.SOPClassUID(), f.TransferSyntax())
			seen[key] = true
		}
	}

	assoc, err := ae.AssociateAs(*addr, *called)
	if err != nil {
		log.Fatalf("association failed: %v", err)
	}
	defer assoc.Release()

	var failures int
	for _, it := range items {
		status, err := assoc.SendCStore(it.file.DataSet)
		if err != nil {
			log.Printf("%s: C-STORE error: %v", it.path, err)
			failures++
			continue
		}
		if !status.IsSuccess() {
			log.Printf("%s: C-STORE status 0x%04X (%s)", it.path, uint16(status), status.Category())
			failures++
			continue
		}
		log.Printf("%s: stored OK", it.path)
	}
	if failures > 0 {
		os.Exit(1)
	}
}

// SPDX-License-Identifier: Apache-2.0

package godicom_test

import (
	"fmt"
	"log"

	"github.com/pravesh707/go-dicom"
)

// ExampleAE_echo starts an in-process Verification SCP, associates with it as an
// SCU, and performs a C-ECHO — the canonical end-to-end round trip.
func ExampleAE_echo() {
	// --- SCP ---
	scp := godicom.NewAE("ECHO_SCP")
	scp.AddSupportedContext(godicom.VerificationSOPClass)
	srv, err := scp.StartServer("127.0.0.1:0", []godicom.HandlerBinding{
		{Event: godicom.EvtCEcho, Handle: func(e *godicom.Event) godicom.Status {
			return godicom.StatusSuccess
		}},
	})
	if err != nil {
		log.Fatal(err)
	}
	defer srv.Shutdown()

	// --- SCU ---
	scu := godicom.NewAE("ECHO_SCU")
	scu.AddRequestedContext(godicom.VerificationSOPClass)
	assoc, err := scu.Associate(srv.Addr().String())
	if err != nil {
		log.Fatal(err)
	}
	defer assoc.Release()

	status, err := assoc.SendCEcho()
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(status.Category())
	// Output: Success
}

// ExampleNewAE_options configures an AE with functional options.
func ExampleNewAE_options() {
	ae := godicom.NewAE("MY_SCU",
		godicom.WithMaximumLength(32768),
		godicom.WithImplementationVersionName("MYAPP_1_0"),
	)
	ae.AddRequestedContext(godicom.VerificationSOPClass)
	fmt.Println(ae.AETitle, ae.MaximumLength)
	// Output: MY_SCU 32768
}

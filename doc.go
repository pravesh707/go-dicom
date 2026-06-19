// SPDX-License-Identifier: Apache-2.0

// Package godicom is a from-scratch, concurrency-first DICOM networking library
// for Go, It implements the DICOM Upper Layer protocol
// (PS3.8), ACSE association negotiation, and the DIMSE service elements (PS3.7),
// with a high-level Application Entity (AE) API for building Service Class Users
// (SCU) and Service Class Providers (SCP).
//
// Unlike a Python implementation constrained by the GIL, each inbound
// association is served on its own goroutine, so a single SCP scales across CPU
// cores without multiprocessing.
//
// # Layers
//
// The library is built bottom-up, each package depending only on those below:
//
//	dicom        data sets, VRs, the data dictionary, transfer-syntax codecs
//	pdu          the seven Upper Layer PDUs and their items
//	dimse        DIMSE messages, command sets and status codes
//	association  ACSE negotiation, the DUL lifecycle, PDV framing, events
//	godicom      the high-level AE, Associate and StartServer API (this package)
//
// # Example (Verification SCU)
//
//	ae := godicom.NewAE("ECHO_SCU")
//	ae.AddRequestedContext(godicom.VerificationSOPClass)
//	assoc, err := ae.Associate("127.0.0.1:11112")
//	if err != nil {
//		log.Fatal(err)
//	}
//	defer assoc.Release()
//	status, err := assoc.SendCEcho()
//
// # Example (Verification SCP)
//
//	ae := godicom.NewAE("ECHO_SCP")
//	ae.AddSupportedContext(godicom.VerificationSOPClass)
//	srv, _ := ae.StartServer(":11112", []godicom.HandlerBinding{
//		{Event: godicom.EvtCEcho, Handle: func(e *godicom.Event) godicom.Status {
//			return godicom.StatusSuccess
//		}},
//	})
//	defer srv.Shutdown()
package godicom

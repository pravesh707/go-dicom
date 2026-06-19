# godicom

A from-scratch, concurrency-first **DICOM networking library for Go**, inspired
by [pynetdicom](https://pydicom.github.io/pynetdicom/stable/). It implements the
DICOM Upper Layer protocol (PS3.8), ACSE association negotiation, and the DIMSE
service elements (PS3.7), with a high-level Application Entity (AE) API for
building Service Class Users (SCU) and Providers (SCP).

It is a normal, importable Go module — the equivalent of a Ruby gem or a Python
package — with **no third-party dependencies**. The DICOM data layer (tags, VRs,
data sets, the data dictionary, and the transfer-syntax codecs — the "pydicom"
part) is implemented from scratch.

All DIMSE-C services — **C-ECHO, C-STORE, C-FIND, C-MOVE, C-GET** — work as both
SCU and SCP and are verified to interoperate with pynetdicom 3.0.4. The DIMSE-N
services (N-GET/SET/ACTION/CREATE/DELETE/EVENT-REPORT) and DICOM Part-10 file
read/write are implemented and tested in-process.

- **Module path:** `github.com/pravesh707/go-dicom`
- **Package name:** `godicom` — so you import the repo path but write `godicom.*`:

  ```go
  import "github.com/pravesh707/go-dicom" // used as godicom.NewAE(...)
  ```

## Requirements

- **Go 1.22 or newer** (`go version` to check).
- No third-party dependencies, so nothing else to install.

## Install

Once the repo is pushed to `github.com/pravesh707/go-dicom`:

```bash
go get github.com/pravesh707/go-dicom
```

Before it's pushed (developing against a local checkout), require it and point
at the local folder with a `replace` directive in your app's `go.mod`:

```bash
cd /path/to/your-app
go mod edit -require=github.com/pravesh707/go-dicom@v0.0.0
go mod edit -replace=github.com/pravesh707/go-dicom=/path/to/go-dicom
go mod tidy
```

## Run it now (from this folder)

The folder containing `go.mod` is the module — build, test, and run without
installing anything:

```bash
cd /path/to/go-dicom          # the folder containing go.mod

go test ./...                 # run the test suite
go run ./cmd/echoscp -addr :11112 -aet ECHO_SCP        # start a Verification SCP
go run ./cmd/echoscu -addr 127.0.0.1:11112 -called ECHO_SCP   # ping it (another terminal)
```

The SCU prints `C-ECHO response status: 0x0000 (Success)`.

## Install the command-line tools

The module ships two binaries, `echoscu` and `echoscp`:

```bash
# from a local copy (run inside this folder) — installs to $(go env GOBIN)
# or $(go env GOPATH)/bin, which should be on your PATH:
go install ./cmd/echoscu
go install ./cmd/echoscp

# once the repo is published:
go install github.com/pravesh707/go-dicom/cmd/echoscu@latest
go install github.com/pravesh707/go-dicom/cmd/echoscp@latest
```

## Library quick start

### Verification SCP (server)

```go
package main

import (
	"log"

	godicom "github.com/pravesh707/go-dicom"
)

func main() {
	ae := godicom.NewAE("ECHO_SCP")
	ae.AddSupportedContext(godicom.VerificationSOPClass)

	srv, err := ae.StartServer(":11112", []godicom.HandlerBinding{
		{Event: godicom.EvtCEcho, Handle: func(e *godicom.Event) godicom.Status {
			log.Printf("C-ECHO from %q", e.Assoc.CallingAETitle)
			return godicom.StatusSuccess
		}},
	})
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("listening on %s", srv.Addr())
	select {} // each association runs on its own goroutine
}
```

### Verification SCU (client)

```go
package main

import (
	"log"

	godicom "github.com/pravesh707/go-dicom"
)

func main() {
	ae := godicom.NewAE("ECHO_SCU",
		godicom.WithMaximumLength(32768), // functional options
	)
	ae.AddRequestedContext(godicom.VerificationSOPClass)

	assoc, err := ae.Associate("127.0.0.1:11112")
	if err != nil {
		log.Fatal(err)
	}
	defer assoc.Release()

	status, err := assoc.SendCEcho()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("C-ECHO status: 0x%04X (%s)", uint16(status), status.Category())
}
```

> The explicit `godicom "github.com/pravesh707/go-dicom"` alias is optional — Go
> already names the package `godicom` — but it makes the intent obvious.

## Verify against pynetdicom (optional)

The Go tools interoperate with pynetdicom's console scripts
(`pip install pynetdicom`):

```bash
# our SCU against pynetdicom's SCP
echoscp 11113 &
go run ./cmd/echoscu -addr 127.0.0.1:11113

# pynetdicom's SCU against our SCP
go run ./cmd/echoscp -addr :11112 &
echoscu 127.0.0.1 11112
```

> Verified against pynetdicom 3.0.4: C-ECHO succeeds in both directions
> (`0x0000 Success`).

## Project layout

Standard Go layout: importable packages at the module root and subdirectories,
binaries under `cmd/`, docs under `docs/`. (Library packages are intentionally
*not* buried under `/pkg`, which would only lengthen import paths.)

```
go-dicom/
├── doc.go, godicom.go, client.go, server.go   # high-level AE API (package godicom)
├── options.go, transport.go, version.go
├── dicom/        # data sets, VRs, dictionary, transfer-syntax codecs, Part-10 files
├── pdu/          # the 7 Upper Layer PDUs and their items
├── dimse/        # DIMSE-C/N messages, command sets, status codes
├── association/  # ACSE negotiation, DUL lifecycle, PDV framing, events, services
├── cmd/
│   ├── echoscu/  echoscp/   # Verification SCU/SCP
│   ├── storescu/ storescp/  # Storage SCU/SCP
│   ├── findscu/             # Query SCU
│   └── qrscp/               # Query/Retrieve + Storage SCP (C-FIND/MOVE/GET/STORE)
├── tests/        # end-to-end / integration tests (public-API only)
├── docs/ARCHITECTURE.md
├── LICENSE, NOTICE, CONTRIBUTING.md, Makefile
└── .github/workflows/ci.yml
```

Tests are split by kind: the integration tests live in `tests/`, while each
package keeps its own white-box unit tests next to the code (Go requires tests
that touch unexported identifiers to share the package directory). Run all of
them with `go test ./...`.

## Why a Go port

A Python SCP is bounded by the GIL. Serving many concurrent associations — or
doing CPU-bound work per association — pushes you into `multiprocessing` and all
its overhead. godicom serves **one goroutine per association**, so a single
process scales across every core. The test suite runs 25 simultaneous
associations through one server, clean under `go test -race`.

## Development

```bash
make all      # gofmt check + vet + test
make race     # tests with the race detector
make cover    # coverage report
make examples # build cmd/ tools into ./bin
```

## License

Licensed under the [Apache License 2.0](LICENSE). See [NOTICE](NOTICE).

Before deploying, replace the placeholder Implementation Class UID in
`dicom/uid.go` (`GoDICOMImplementationClassUID`) with one under your own
organisation's UID root.

# Contributing to godicom

Thanks for your interest in improving godicom.

## Development

Requires Go 1.22+. There are no third-party dependencies.

```bash
make all      # gofmt check, vet, test
make race     # tests with the race detector
make examples # build cmd/ tools into ./bin
```

## Conventions

- Run `gofmt` (or `make fmt`) before committing; CI rejects unformatted code.
- Keep packages layered: `dicom` → `pdu`/`dimse` → `association` → root. Lower
  layers must not import higher ones.
- Reference the relevant DICOM standard part in doc comments (e.g. "PS3.8
  §9.3.2") when implementing protocol structures.
- Add a test for new behaviour. Wire-format changes should be checked against a
  reference implementation where practical.

## Adding a DIMSE service (the extension pattern)

New services slot in without editing core dispatch:

1. Add the primitive structs in `dimse` implementing the `Message` interface.
2. Register their parsers in an `init()` via `dimse.RegisterMessage`.
3. Add an intervention event in `association` and a dispatch case for it in the
   serve loop.

## Commit / PR

- One logical change per pull request.
- Describe the DICOM behaviour and cite the standard where relevant.
- Ensure `make all` and `make race` pass.

CI (`.github/workflows/ci.yml`) runs gofmt, `go vet`, `go build`, and
`go test -race` with coverage across the supported Go versions on every push and
pull request.

## Versioning & releases

The project follows [Semantic Versioning](https://semver.org/). The single
source of truth for the version is the `Version` constant in `version.go`.

To cut release `vX.Y.Z`:

1. Bump `Version` in `version.go` to `X.Y.Z`.
2. Commit, then tag and push:

   ```bash
   git tag vX.Y.Z
   git push origin vX.Y.Z
   ```

The release workflow (`.github/workflows/release.yml`) then verifies that the
tag matches the `Version` constant, runs the tests, builds the `cmd/` tools, and
publishes a GitHub release with notes generated automatically from the merged
pull requests. Consumers pin a version with
`go get github.com/pravesh707/go-dicom@vX.Y.Z`.

By contributing you agree your contributions are licensed under the project's
Apache-2.0 license.

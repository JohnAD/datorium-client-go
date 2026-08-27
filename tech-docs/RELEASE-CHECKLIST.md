# Release checklist

1. `gofmt -l .` is empty; `go vet ./...` clean.
2. `go mod tidy -diff` clean; `go test ./... -race -count=1` green.
3. `go build ./...` and tool binaries build.
4. Confirm `go.mod` module path is `github.com/JohnAD/datorium-client-go/v2`
   (major version ≥ 2 requires the `/v2` import path suffix).
5. Update [COMPATIBILITY.md](COMPATIBILITY.md) against the target DatoriumDB tag.
6. Move `[Unreleased]` notes in `CHANGELOG.md` into a dated version section.
7. Tag `vX.Y.Z` matching SemVer (for this line: `v2.1.0`, …). Users install with
   `go get github.com/JohnAD/datorium-client-go/v2@vX.Y.Z`.
8. Optional: run `./start_integration_test.sh` on a Docker host before tagging.

# Release checklist (`v0.1.0` and later)

1. `gofmt -l .` is empty; `go vet ./...` clean.
2. `go mod tidy -diff` clean; `go test ./... -race -count=1` green.
3. `go build ./...` and tool binaries build.
4. Update [COMPATIBILITY.md](COMPATIBILITY.md) against the target DatoriumDB tag.
5. Move `[Unreleased]` notes in `CHANGELOG.md` into a dated version section.
6. Tag `vX.Y.Z` matching SemVer and module expectations.
7. Optional: run `./start_integration_test.sh` on a Docker host before tagging.

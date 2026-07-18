# Contributing

Thanks for helping improve the DatoriumDB Go client.

## Prerequisites

- Go 1.25.11+
- Optional for the Todo integration demo: Docker with `docker compose`, plus a
  local checkout of [`datoriumdb`](https://github.com/JohnAD/datoriumdb)

## Local checks

From the repository root:

```bash
gofmt -w .
go vet ./...
go mod tidy
go test ./... -race -count=1
```

Optional end-to-end demonstration (creates and deletes a Docker Compose stack):

```bash
./start_integration_test.sh
```

## Compatibility expectations

- Treat DatoriumDB HTTP API `v1` and the access-language contract as the
  source of truth.
- Prefer independent protocol logic in this repository over importing
  `datoriumdb` internal packages.
- When server goldens or docs change, update client tests and
  [tech-docs/COMPATIBILITY.md](tech-docs/COMPATIBILITY.md).

## Pull requests

- Keep changes focused and covered by unit or mocked HTTP tests.
- Update `CHANGELOG.md` under `[Unreleased]` for user-visible changes.
- Do not commit integration runtime artifacts under `.test-run/`.

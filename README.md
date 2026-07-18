# datorium-client-go

Idiomatic Go smart client for [DatoriumDB](https://github.com/JohnAD/datoriumdb).

This library talks to DatoriumDB's HTTP API `v1`: it caches establishment
config, routes create/read/patch/delete/search commands to the correct shard
members, retries `wrongMachine` responses, and resolves document references.

Status: early development. Compatible with DatoriumDB `v0.0.2` / API `v1`.

## Install

```bash
go get github.com/JohnAD/datorium-client-go@latest
```

Requires Go 1.25.11 or newer (matching the DatoriumDB module).

## Quick start

```go
package main

import (
	"context"
	"fmt"
	"log"

	datorium "github.com/JohnAD/datorium-client-go"
)

func main() {
	ctx := context.Background()
	client, err := datorium.New(datorium.Config{
		EstablishmentURL: "http://127.0.0.1:8081",
		Token:            "Bearer-token-here",
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	if err := client.Establish(ctx); err != nil {
		log.Fatal(err)
	}

	created, err := client.Create(ctx, "Todos", "", map[string]any{
		"$":     "Todos:0",
		"title": "Buy milk",
		"status": "open",
	})
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println("created", created.ID, "version", created.Version)
}
```

## Features

- Bearer-authenticated HTTP transport with JSON envelopes (`ok` / `errors`)
- Establishment fetch + in-memory cache with config-version tracking
- CRC32 shard slot routing for writes (SOT) and reads (read members)
- Bounded `wrongMachine` retry with optional host URL rewriting for Docker
- Typed CRUD + search helpers, ULID `operationId` support
- Direct (`@`) and cached (`@@`) reference helpers
- Front-page helpers for arrays of cached refs (`AppendCachedRefOp`, `SummariesForArrayField`)
- Opt-in two-shard Todo integration demo (`./start_integration_test.sh`)

## Documentation

- [tech-docs/ROADMAP.md](tech-docs/ROADMAP.md)
- [tech-docs/ARCHITECTURE.md](tech-docs/ARCHITECTURE.md)
- [tech-docs/PROTOCOL.md](tech-docs/PROTOCOL.md)
- [tech-docs/COMPATIBILITY.md](tech-docs/COMPATIBILITY.md)
- [tech-docs/TESTING.md](tech-docs/TESTING.md)

Server protocol source of truth lives in the sibling DatoriumDB repository
(`tech-docs/ACCESS-LANGUAGE.md`, `SHARDING.md`, `AUTHENTICATION.md`,
`ESTABLISHMENT-CONFIG.md`, `SEARCHING.md`).

## Integration demo

Requires Docker with Compose support and a checkout of `datoriumdb` next to
this repository (or set `DATORIUMDB_SRC`):

```bash
./start_integration_test.sh
```

The script builds a two-shard Compose stack (`00-7F` / `80-FF`), runs a host
Todo CLI through this client library, then tears the stack down.

## Development

```bash
gofmt -w .
go test ./... -race -count=1
./start_integration_test.sh   # optional; needs Docker + sibling datoriumdb
```

See [CONTRIBUTING.md](CONTRIBUTING.md) and [tech-docs/RELEASE-CHECKLIST.md](tech-docs/RELEASE-CHECKLIST.md).

## License

MIT — see [LICENSE](LICENSE).

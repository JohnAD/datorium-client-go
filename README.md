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

	// Empty id → client mints a ULID (server never assigns create IDs).
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

> **Order warning:** Raw helpers that take `map[string]any` for document content are
> **order-unsafe**. Go maps randomize key order; DatoriumDB honors client field
> order for non-schema (non-SOT) fields when storing git-tracked JSON. Prefer the
> typed collection API below (or build details with [`ojson`](https://github.com/JohnAD/ojson))
> whenever document field order matters. `Create` marshals the command line once
> before network attempts so retries cannot reshuffle keys, and may confirm
> ambiguous create failures with a follow-up read.

## Typed collections

Declare a `Collection[T]` descriptor, verify it in `Establish`, then `Bind` a
typed `CollectionClient[T]` and use its methods. Pass `nil` as the create id to
mint a ULID locally (the server never assigns create IDs). See
[`docs/documents.md`](docs/documents.md) and [`docs/patches.md`](docs/patches.md).

```go
package main

import (
	"context"
	"fmt"
	"log"

	datorium "github.com/JohnAD/datorium-client-go"
)

type Todo struct {
	Title  string `json:"title"`
	Status string `json:"status"`
}

var Todos = datorium.MustCollection[Todo]("Todos", 0)

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

	if err := client.Establish(ctx, Todos); err != nil {
		log.Fatal(err) // CatalogError if name/version mismatch
	}
	todos, err := Todos.Bind(client)
	if err != nil {
		log.Fatal(err)
	}

	created, err := todos.CreateDoc(ctx, nil, Todo{
		Title: "Buy milk", Status: "open",
	})
	if err != nil {
		log.Fatal(err)
	}
	item, err := todos.GetDoc(ctx, created.ID)
	if err != nil {
		log.Fatal(err)
	}

	item.Doc.Status = "done"
	patch, err := todos.CreatePatchFromChanges(item)
	if err != nil {
		log.Fatal(err)
	}
	patched, err := todos.PatchDoc(ctx, patch)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(item.Doc.Title, patched.Version)
}
```

(`CollectionClient[T]` carries the type parameter so methods work; Go does not
allow type parameters on methods of the non-generic `*Client`.)

## Features

- Bearer-authenticated HTTP transport with JSON envelopes (`ok` / `errors`)
- Establishment fetch + in-memory cache with config-version tracking
- CRC32 shard slot routing for writes (SOT) and reads (read members)
- Bounded `wrongMachine` retry with optional host URL rewriting for Docker
- Typed collection clients (`Collection[T].Bind` → `CollectionClient[T]` methods)
- Raw CRUD + search helpers, ULID `operationId` support
- Direct (`@`) and cached (`@@`) reference helpers
- Front-page helpers for arrays of cached refs (`AppendCachedRefOp`, `SummariesForArrayField`)
- Opt-in two-shard Todo integration demo (`./start_integration_test.sh`)

## Documentation

**Using the library:** start at [`docs/README.md`](docs/README.md) (API guide for application authors).

**Developing this library** (internals, protocol notes, roadmap, release):
start at [`tech-docs/ROADMAP.md`](tech-docs/ROADMAP.md).

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

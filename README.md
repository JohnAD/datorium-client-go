# datorium-client-go

Idiomatic Go client for [DatoriumDB](https://github.com/JohnAD/datoriumdb).

DatoriumDB lets you store your database as git-trackable JSON documents in
ordinary directories — and still scale with sharding and clusters.

Status: early development. Compatible with DatoriumDB `v1.0.0` / API `v1`.

## Install

```bash
go get github.com/JohnAD/datorium-client-go/v2@latest
```

Requires Go 1.25.11 or newer (matching the DatoriumDB module).

## Quick start (typed collections)

Create a general client, then `Bind` a typed client for each collection you use.
The first `Bind` connects to the establishment server and checks the schema.
See
[`docs/documents.md`](docs/documents.md) and [`docs/patches.md`](docs/patches.md).

```go
package main

import (
	"context"
	"fmt"
	"log"

	datorium "github.com/JohnAD/datorium-client-go/v2"
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

	todos, err := Todos.Bind(ctx, client)
	if err != nil {
		log.Fatal(err) // CatalogError if name/version mismatch
	}

	// CREATE — nil id means the client mints a ULID
	created, err := todos.CreateDoc(ctx, nil, Todo{
		Title: "Buy milk", Status: "open",
	})
	if err != nil {
		log.Fatal(err)
	}

	// READ
	item, err := todos.GetDoc(ctx, created.ID)
	if err != nil {
		log.Fatal(err)
	}

	// PATCH
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

	// DELETE — use id + version from the write result
	if _, err := todos.DeleteDoc(ctx, patched.ID, patched.Version); err != nil {
		log.Fatal(err)
	}
}
```

## Raw API (escape hatch)

Stringly-typed `map[string]any` helpers remain available, but they are **not**
recommended for application code: Go maps randomize key order, and DatoriumDB
honors client field order for non-schema fields in git-tracked JSON. Prefer
typed collections (or [`ojson`](https://github.com/JohnAD/ojson)) whenever order
matters.

```go
created, err := client.Create(ctx, "Todos", "", map[string]any{
	"$": "Todos:0", "title": "Buy milk", "status": "open",
})
```

Empty id → this client mints a ULID (the server never assigns create IDs).
Raw helpers fetch establishment config on first use; you do not need a separate
`Establish` call unless you want an explicit whole-catalog check.

## Features

- Talks to DatoriumDB over HTTP with bearer auth
- Discovers the cluster layout on first use and keeps it cached
- Sends each request to the right shard automatically
- Typed collection clients for create, read, patch, and delete
- Optional raw helpers when you need an escape hatch
- Helpers for live and cached document references

## Documentation

**Using the library:** start at [`docs/README.md`](docs/README.md).

**Developing this library:** start at [`tech-docs/ROADMAP.md`](tech-docs/ROADMAP.md)
(testing, integration demo, architecture, release checklist). Also see
[CONTRIBUTING.md](CONTRIBUTING.md).

## License

MIT — see [LICENSE](LICENSE).

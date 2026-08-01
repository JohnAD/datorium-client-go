# Getting started

## Install

```bash
go get github.com/JohnAD/datorium-client-go@latest
```

Requires Go **1.25.11** or newer.

## Minimal client

```go
client, err := datorium.New(datorium.Config{
    EstablishmentURL: "http://127.0.0.1:8081", // scheme + host[:port], no path
    Token:            "your-bearer-token",
})
if err != nil {
    log.Fatal(err)
}
defer client.Close()
```

You need:

1. A reachable establishment base URL.
2. A bearer token (static string or a `TokenSource`).

Raw helpers lazy-fetch establishment on first routed command. Typed apps usually call `Bind` instead (see below).

## Recommended: typed collections

Prefer structs + `Collection[T]` so document field order stays stable and call sites avoid stringly-typed collection names. `Bind` lazily establishes and validates that collection, then returns a `CollectionClient[T]`. For patches, edit `item.Doc` and call `CreatePatchFromChanges` — see [Patch instructions](patches.md).

```go
type Todo struct {
    Title  string `json:"title"`
    Status string `json:"status"`
}

var Todos = datorium.MustCollection[Todo]("Todos", 0)

todos, err := Todos.Bind(ctx, client)
if err != nil {
    log.Fatal(err) // CatalogError if the live schema does not match
}

wr, err := todos.CreateDoc(ctx, nil, Todo{
    Title: "Buy milk", Status: "open",
})
// wr.ID and wr.Version identify the new document
```

Optional: call `client.Establish(ctx, Todos, Users, …)` once at startup to validate a whole catalog before binding. See [Client and config](client.md).

See [Documents](documents.md), [Patch instructions](patches.md), and [Client and config](client.md).

## Raw maps (escape hatch)

```go
wr, err := client.Create(ctx, "Todos", "", map[string]any{
    "$": "Todos:0", "title": "Buy milk", "status": "open",
})
```

Empty `id` means **this client mints a ULID** — the server never assigns create IDs.

**Caveat:** `map[string]any` document bodies are **order-unsafe** (Go randomizes map keys). Use typed collection clients or `ojson` when non-schema field order matters for git-tracked JSON.

## Next

- [Client and config](client.md)
- [Documents](documents.md)
- [Patch instructions](patches.md)
- [Errors](errors.md)

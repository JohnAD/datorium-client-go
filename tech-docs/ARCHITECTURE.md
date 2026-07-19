# Architecture

## Package layout

| Path | Role |
|------|------|
| Root `datorium` | Public `Client`, config/options, CRUD/search/establish APIs, typed `Collection[T]` layer |
| `shard/` | Document ID CRC32 slot + range parsing (ported from DatoriumDB) |
| `searchpath/` | Search result path encoding + shard slot |
| `refs/` | Parse `@__Collection__id` / `@@__Collection__id` |
| `internal/testtoken/` | Dev-only Ed25519 JWT minting for integration tests |
| `cmd/todo-integration/` | Host CLI for Milestone 6 demo |
| `test/integration/todo/` | Compose + establishment fixtures |

The client does **not** depend on `github.com/JohnAD/datoriumdb` packages. Protocol
logic is independently implemented and verified against contract shapes.

## Client boundaries

```mermaid
flowchart LR
  App --> Client
  Client --> EstablishCache
  Client --> Router
  Client --> Transport
  EstablishCache --> Router
  Router --> Transport
  Transport --> ServerA
  Transport --> ServerB
```

- **Transport** sends `Authorization: Bearer …`, posts `text/plain; charset=utf-8`
  command bodies, and always JSON-decodes the response body.
- **EstablishCache** stores `general`, `servers`, `shardMap`, `schemas`, `searches`, `auth`.
- **Router** picks SOT vs read-member base URLs from document/search shard slots.
- **Host rewrite** maps Docker-internal `baseURL`s to host-reachable URLs when set.

## Concurrency

- `Client` methods are safe for concurrent use after construction.
- Establishment refresh uses a mutex; in-flight readers may see the previous config until refresh completes.
- Callers own token lifecycle via a static string or `TokenSource`.

## Retry policy

1. On transport failure: optional bounded backoff retries (configurable).
2. On `wrongMachine`: refresh establish if `configVersion` is newer/stale, rewrite URL, retry up to a small bound.
3. On `versionMismatch`: surface typed error; optional helper re-reads and retries once.
4. Writes include a client-generated ULID `operationId` unless the caller supplies one.

## Typed collections and document order

Applications may declare `Collection[T]` values (name + schema version) and pass
them to `Establish` for catalog validation against live `schemas`. Typed writes
build command detail objects with [`ojson`](https://github.com/JohnAD/ojson)
(`NewObjectFromStructTry` → `ToJSONBytes`) so non-SOT field order matches Go
struct declaration order. Typed reads re-parse `sot` from the original response
bytes with ojson into `T` (plus `DocMeta` for `!` / `$` / `#`).

**Invariant:** typed document paths must never store or round-trip document
bodies as `map[string]any`. Go maps randomize keys; the server honors client
order for non-SOT fields when persisting git-friendly JSON.

Raw `Create`/`Patch`/`Delete` helpers that accept `map[string]any` remain as an
escape hatch and are order-unsafe for document content. Do not “fix” maps by
feeding them through ojson `NewObjectFromMap` (that sorts keys). Prefer typed
APIs or hand-built `ojson.JSONValue` details via `BuildCommandOrdered`.

### Patch follow-up (not implemented)

RFC6902 patch ops are path-oriented; a full typed “replace struct” API is a
separate product. Future helpers should emit ojson-ordered op values whenever
patch `value`s contain objects. Candidates: `PatchDoc(ctx, col, id, version,
ops)` and/or typed path helpers / `ReplaceFields` from struct field names.

# Client and config

Package: `github.com/JohnAD/datorium-client-go`

## `datorium.New`

```go
func New(cfg Config) (*Client, error)
```

Builds a smart client. Requires `EstablishmentURL` and either `Token` or `TokenSource`.

## `Config`

| Field | Role |
|-------|------|
| `EstablishmentURL` | Base URL of the establishment server (`http://host:port`, no path). **Required.** |
| `Token` | Static bearer token (without or with a `Bearer ` prefix; both work). Ignored if `TokenSource` is set. |
| `TokenSource` | Dynamic token provider (`Token(ctx) (string, error)`). |
| `HTTPClient` | Optional custom `*http.Client` (default 30s timeout). |
| `BaseURLRewrite` | Map Docker-internal base URLs or server names to host-reachable URLs. |
| `PreferServer` | Preferred server name when dual-role eligible for local routing. |
| `WrongMachineRetries` | Max `wrongMachine` bounce retries (default `3`). |
| `TransportRetries` | Retries on transport failures (default `0`). |
| `CreateAmbiguousVerifyDelay` | Wait before follow-up read after a create **transport** failure. `0` → 3s default; negative → disable. |
| `UserAgent` | `User-Agent` header (default `datorium-client-go`). |

### Tokens

```go
type TokenSource interface {
    Token(ctx context.Context) (string, error)
}

type StaticToken string // implements TokenSource
```

## Lifecycle

```go
func (c *Client) Close() error
func (c *Client) CachedEstablishment() *Establishment
```

- `Close` is safe and currently a no-op (reserved for future resources).
- `CachedEstablishment` returns the last successful establish document, or `nil`.

Methods are safe for concurrent use after `New`.

## Health and readiness

```go
func (c *Client) Health(ctx context.Context) (Result, error)
func (c *Client) Ready(ctx context.Context) (Result, error)
```

Unauthenticated probes against the establishment URL (`/datoriumdb/v1/health` and `/ready`).

## Establish

```go
func (c *Client) Establish(ctx context.Context, cols ...CollectionRef) error
```

Fetches and caches establishment config (servers, shard map, schemas, searches, auth).

- No `cols`: fetch and cache only (raw / dynamic apps).
- With `cols`: also validates each declared collection name and schema version against live `schemas`. Mismatch → [`CatalogError`](errors.md). Extra server collections not listed in `cols` are fine.

Typed apps usually skip an explicit `Establish`: `Collection.Bind` lazy-establishes and validates that one collection. Use `Establish` when you want one startup check for a whole catalog:

```go
err := client.Establish(ctx, Todos, Users, TodoLists)
todos, err := Todos.Bind(ctx, client) // reuses cache
```

## Schema history

```go
func (c *Client) Schema(ctx context.Context, collection string, version int) (Result, error)
```

Fetches a historic schema document from the establishment server.

## Establishment types (read-only view)

After establish you can inspect:

- `Establishment.General` — cluster name, version, establishment server
- `Establishment.Servers` — name → `baseURL`
- `Establishment.ShardMap` — range → SOT / read / proxy members
- `Establishment.Schemas` — collection → `{Version, Doc}` (`Doc` is ordered `ojson.JSONValue`)
- `Establishment.Searches`, `Establishment.Auth` — ordered `ojson.JSONValue`

Helpers:

```go
func (e *Establishment) AssignmentForSlot(slot byte) (ShardAssignment, bool)
func (e *Establishment) ServerBaseURL(name string) string
```

For typed apps, call `Todos.Bind(ctx, client)`; the bound `CollectionClient` exposes `CompiledSchema()` from the schema compiled at bind time. Most applications never need the other helpers; the client routes automatically.

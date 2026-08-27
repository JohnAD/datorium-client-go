# Client and config

Package: `github.com/JohnAD/datorium-client-go/v2`

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

The client calls `Token(ctx)` **on every request** (the result is not cached).
That makes a self-renewing `TokenSource` the natural pattern for long-running
services whose tokens expire: mint (or fetch) a token, cache it, and refresh
it shortly before expiry.

```go
type renewingSource struct {
    mu     sync.Mutex
    token  string
    expiry time.Time
}

// Token returns the cached token, refreshing it when it is about to expire.
// The client invokes this per request, so it must be cheap in the common case.
func (s *renewingSource) Token(ctx context.Context) (string, error) {
    s.mu.Lock()
    defer s.mu.Unlock()
    if s.token == "" || time.Until(s.expiry) < 30*time.Second {
        tok, exp, err := mintToken(ctx) // your identity system's call
        if err != nil {
            if s.token != "" {
                return s.token, nil // keep serving the soon-to-expire token
            }
            return "", err
        }
        s.token, s.expiry = tok, exp
    }
    return s.token, nil
}
```

Guidelines:

- **Refresh with a margin** (e.g. 30 s) so in-flight requests never carry an
  already-expired token.
- **Serve the old token on refresh failure** when one exists, so a transient
  identity-provider outage does not take down the service.
- Return the token **without** the `Bearer ` prefix (a prefix is tolerated
  and stripped, but plain is preferred). Never return an empty string.
- `TokenSource` must be safe for concurrent use; the client is.
- For short-lived processes and tests, a plain `Token` string or
  `StaticToken` is enough — see [Testing and integration setup](testing.md).

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

## Admin catalog (establishment only)

Requires an **admin** JWT (`datoriumdb.kind=admin`) and posts to the establishment URL:

```go
func (c *Client) EnsureCollection(ctx context.Context, collection string, schema, upgrade map[string]any) (Result, error)
func (c *Client) EnsureSearch(ctx context.Context, collection, searchName string, definition map[string]any) (Result, error)
func (c *Client) DeleteSearch(ctx context.Context, collection, searchName string) (Result, error)
```

Successful calls refresh the establishment cache so subsequent `Bind` / routing see the new catalog. Document `$` migration after a schema upgrade continues asynchronously on the server.

## Schema history

```go
func (c *Client) Schema(ctx context.Context, collection string, version int) (Result, error)
```

Fetches a historic schema document from the establishment server.

## Establishment types (read-only view)

After establish you can inspect:

- `Establishment.General` — cluster name, version, establishment server, optional `MaxFileBytes`
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

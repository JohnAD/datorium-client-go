# Protocol notes (client view)

Source of truth: DatoriumDB `internal/server/http_command.go`, `docs/api.md`,
and `tech-docs/ACCESS-LANGUAGE.md` / `BINARY-FILES.md`.

## Endpoints used by this client

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| `GET` | `/datoriumdb/v1/health` | No | Liveness |
| `GET` | `/datoriumdb/v1/ready` | No | Config loaded |
| `GET` | `/datoriumdb/v1/establish` | Bearer | Combined establishment document |
| `POST` | `/datoriumdb/v1/command` | Bearer | All public commands (JSON or multipart) |
| `GET` | `/datoriumdb/v1/schema/{collection}/{ver}` | Bearer | Historic schema |

Not used by application clients: `/datoriumdb/v1/sys/*`, machine-token bootstrap.
Public `/datoriumdb/v1/files/...` routes are **removed**.

## Command transport

```text
POST /datoriumdb/v1/command
Content-Type: application/json
Authorization: Bearer {jwt}

{"command":"create","target":"Todos","parameter":"01KWD65CFQPEZS7H1WJE4MK990","detail":{"$":"Todos:0","title":"Buy milk","status":"open"}}
```

Root fields are exactly `command`, `target`, `parameter`, and `detail`
(`detail` is always a JSON object). `BuildCommand` / `BuildCommandOrdered`
return this body as `[]byte`, marshaled once before retries so field order and
document id stay stable.

Create IDs are always client-supplied (typically a ULID). The server rejects
`null` / omitted ids. This client mints an ID when callers pass `""` (raw
`Create`) or `nil` (`CollectionClient.CreateDoc`).

### Admin commands

| Client API | Command | Notes |
|------------|---------|-------|
| `EnsureCollection` | `collectionEnsure` | Admin JWT; establishment URL only |
| `EnsureSearch` | `searchEnsure` | Admin JWT; `detail` = search definition |
| `DeleteSearch` | `searchDelete` | Admin JWT |

Configure the client with an admin token (`datoriumdb.kind=admin`). Responses
return after config write + reload; document migration stays asynchronous.

## Envelope

HTTP status is typically `200` for application outcomes. Inspect body:

```json
{"ok": true, "...": "..."}
{"ok": false, "errors": [{"code":"...", "message":"..."}], "...": "..."}
```

Successful `fileRead` responses are raw streams identified by
`X-DatoriumDB-File-Version` / `X-DatoriumDB-SHA256` headers.

`wrongMachine` may place a diagnostic `configVersion` on the **top-level**
envelope (what that refusing server believes). It does not include
`correctServer`, `baseURL`, or `shardSlot`. Clients always re-fetch
establishment and recompute the next hop locally.

## Auth

MVP tokens are EdDSA JWTs with claims `iss`, `aud`, `sub`, `iat`, `exp`,
`datoriumdb.kind` = `client` (CRUD) or `admin` (catalog ensure). The client
library does not issue tokens; callers supply them.

## Commands

`create`, `read`, `patch`, `delete`, `search`, and `file*` as defined in
DatoriumDB `ACCESS-LANGUAGE.md` / `BINARY-FILES.md`. Patch details require `$`,
`#`, and `RFC6902: [...]`.

## Write distribution (DatoriumDB `v0.0.6+`)

Successful `create` / `patch` / `delete` / file mutation envelopes include
informational `distributionComplete`. When true, the relevant one-shot
distribution finished in the response window (or no such work was required).
When false, the SOT write still succeeded; remaining work continues
asynchronously. Incomplete document or binary replication may also include a
top-level `note` object. This client surfaces both on write results
(`DistributionComplete` and optional `Note`).

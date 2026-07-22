# Protocol notes (client view)

Source of truth: DatoriumDB `internal/server/http.go` and its `tech-docs/`.

## Endpoints used by this client

| Method | Path | Auth | Notes |
|--------|------|------|-------|
| `GET` | `/datoriumdb/v1/health` | No | Liveness |
| `GET` | `/datoriumdb/v1/ready` | No | Config loaded |
| `GET` | `/datoriumdb/v1/establish` | Bearer | Combined establishment document |
| `POST` | `/datoriumdb/v1/command` | Bearer | Access-language body |
| `GET` | `/datoriumdb/v1/schema/{collection}/{ver}` | Bearer | Historic schema |

Not used by application clients: `/datoriumdb/v1/sys/*`, machine-token bootstrap.

## Command transport

```text
POST /datoriumdb/v1/command
Content-Type: text/plain; charset=utf-8
Authorization: Bearer {jwt}

create Todos 01KWD65CFQPEZS7H1WJE4MK990 {"$":"Todos:0","title":"Buy milk","status":"open"}
```

Create IDs are always client-supplied (typically a ULID). The server rejects
`null` / omitted ids. This client mints an ID when callers pass `""` (raw
`Create`) or `nil` (`CollectionClient.CreateDoc`).

This client emits **strict JSON** detail objects. The server also accepts
pseudo-JSON; responses are normal JSON envelopes. Create command lines are
marshaled once before attempts so retries keep stable field order and the same
document id (idempotency under lost responses).

## Envelope

HTTP status is typically `200` for application outcomes. Inspect body:

```json
{"ok": true, "...": "..."}
{"ok": false, "errors": [{"code":"...", "message":"..."}], "...": "..."}
```

`wrongMachine` also places `shardSlot`, `correctServer`, `baseURL`, and
`configVersion` on the **top-level** envelope.

## Auth

MVP tokens are EdDSA JWTs with claims `iss`, `aud`, `sub`, `iat`, `exp`,
`datoriumdb.kind=client`. The client library does not issue tokens; callers
supply them (integration tests mint with the fixture signing key).

## Commands

`create`, `read`, `patch`, `delete`, `search` as defined in
DatoriumDB `ACCESS-LANGUAGE.md`. Patch details require `$`, `#`, and
`RFC6902: [...]`.

# Compatibility

## Supported server

| Server | HTTP API | Client status |
|--------|----------|---------------|
| DatoriumDB `v1.1.0` | `/datoriumdb/v1` JSON/multipart `/command` | Current target for unreleased client |

This client requires the unified JSON command API: four-field
`application/json` bodies (multipart for `fileCreate` / `fileUpdate`), plus
optional single-range `fileRead` downloads (`Range` / `206` / `416`) from
DatoriumDB `v1.1.0`. Older servers that only accept `text/plain` command lines
or public `/files/...` REST routes are **not** supported.

The smart-client HTTP contract (`/health`, `/ready`, `/establish`, `/command`,
`/schema/...`) remains API `v1`.

## Drift detection

1. Re-run unit tests that encode shard/slot and envelope shapes.
2. Diff DatoriumDB `test/contract/golden/` when updating fixtures under
   `testdata/contract/` in this repo.
3. Run `./start_integration_test.sh` against a `datoriumdb` checkout at
   tag `v1.1.0` (or `main` containing that release). Set `DATORIUMDB_SRC`
   if the sibling path is not `../datoriumdb`.

## Known ambiguities / caveats

- Search results and cached summaries are eventually consistent; poll with a
  timeout rather than assuming immediate visibility after writes. When a write
  returns `WriteResult.DistributionComplete == true`, one-shot document,
  search, and cache distribution finished in the response window.
- Establishment `servers[].baseURL` may be Docker-internal; use
  `Config.BaseURLRewrite` when calling from the host.
- Document ID period-prefix sharding follows DatoriumDB `shard.Slot`; IDs
  without a qualifying period hash the whole ID.
- No multi-document transaction API exists; optimistic concurrency is per
  document via `#` versions.
- `general.maxFileBytes` (optional) caps streamed uploads; zero/absent means
  the server default (1 GiB).
- Attachment `Range` requests may be ignored (full `200` body) when the unit
  is unrecognized; unsatisfiable `bytes=` ranges return `416` /
  `invalidRange`.

## Versioning policy

- Client follows SemVer.
- Import path for this major line is `github.com/JohnAD/datorium-client-go/v2`.
- Breaking public Go API changes require a major bump (`/v3`, …).
- Server API `v1` incompatible changes require a new client major or an
  explicit compatibility gate in this document.

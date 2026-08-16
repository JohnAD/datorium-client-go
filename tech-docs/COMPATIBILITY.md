# Compatibility

## Supported server

| Server | HTTP API | Client status |
|--------|----------|---------------|
| DatoriumDB `v0.0.6` | `/datoriumdb/v1` | Current target for unreleased client |

Older DatoriumDB tags (`v0.0.5` and earlier) are not supported: write success
envelopes now include informational `distributionComplete` (and optional
replication `note`), and this client expects that shape.

The smart-client HTTP contract (`/health`, `/ready`, `/establish`, `/command`,
`/schema/...`) remains API `v1`.

## Drift detection

1. Re-run unit tests that encode shard/slot and envelope shapes.
2. Diff DatoriumDB `test/contract/golden/` when updating fixtures under
   `testdata/contract/` in this repo.
3. Run `./start_integration_test.sh` against a `datoriumdb` checkout at
   tag `v0.0.6` (or `main` containing that release). Set `DATORIUMDB_SRC`
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

## Versioning policy

- Client follows SemVer.
- Breaking public Go API changes require a major bump.
- Server API `v1` incompatible changes require a new client major or an
  explicit compatibility gate in this document.

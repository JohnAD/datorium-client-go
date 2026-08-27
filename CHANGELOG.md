# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added

- Binary attachment helpers: `PutFile`, `DownloadFile`, `ListFiles`, `DeleteFile`
  (`fileCreate` / `fileUpdate` / `fileRead` / `fileList` / `fileDelete`).
- `EnsureCollection` / `EnsureSearch` / `DeleteSearch` for establishment-only
  admin catalog commands (requires admin JWT); successful calls refresh the
  establishment cache.
- `GeneralConfig.MaxFileBytes` from establishment `general.maxFileBytes`.
- Additional stable error constants: `adminRequired`, `establishmentRequired`,
  `schemaDrift`, `invalidRequest`, `contentTypeRequired`, plus file codes.
- `mint-token -kind admin` / `testtoken.MintAdminToken` for integration tests.

### Changed

- **Module path:** `github.com/JohnAD/datorium-client-go/v2` (import and
  `go get` use the `/v2` suffix). Tag line remains `v2.x.y`.
- **Breaking:** All commands post JSON
  `{"command","target","parameter","detail"}` to `POST /datoriumdb/v1/command`
  with `Content-Type: application/json`. `BuildCommand` /
  `BuildCommandOrdered` and `Client.Command` return/accept `[]byte` instead of
  text/plain lines.
- Compatibility target is DatoriumDB `v1.0.0` (HTTP API path `v1`). Older
  servers (text/plain command lines or public `/files/...` REST) are not
  supported.
- Contract fixtures refreshed from DatoriumDB `v1.0.0` goldens (including
  `distributionComplete` and `file_*` envelopes).

## [2.0.1] - 2026-08-16

### Added

- `WriteResult.DistributionComplete` from successful create/patch/delete envelopes.
- Optional `WriteResult.Note` (`ReplicationNote`) when document replication was incomplete.
- `Result.BoolField` for top-level boolean envelope fields.

### Changed

- Compatibility target is DatoriumDB `v0.0.6` (HTTP API `v1`).

## [2.0.0] - 2026-07-31

### Changed

- Compatibility target is DatoriumDB `v0.0.5` (HTTP API `v1` unchanged).
- `Collection.Bind(ctx, client)` lazy-establishes on first use and validates that collection; explicit `Establish` remains optional for whole-catalog startup checks.
- `CollectionClient.DeleteDoc` takes `id` and `version` (for example from a `WriteResult`) instead of a `CollectionItem`.
- README and app docs lead with typed collections; contributor-only runbooks stay under `tech-docs/`.

## [1.0.1] - 2026-07-31

### Fixed

- On `wrongMachine`, always re-fetch establishment and recompute the next route. Bounce `correctServer` / `baseURL` / `shardSlot` are no longer used for routing (`configVersion` on the bounce is diagnostic only).

## [1.0.0] - 2026-07-22

### Added

- Initial smart Go client for DatoriumDB HTTP API `v1`.
- Establishment caching, CRC32 shard routing, and `wrongMachine` retry.
- Typed collection API: `Collection[T]`, `Bind`, `CollectionClient[T]`, `CollectionItem`, `CollectionPatch`, plus `CreateDoc` / `GetDoc` / `PatchDoc` / `DeleteDoc` and schema-enforced `CreatePatchFromChanges` / `CreatePatch`.
- `BuildCommandOrdered` for access-language commands with stable JSON field order.
- `NewDocumentID` and create idempotency helpers (`documentExists` / transport-failure follow-up read via `CreateAmbiguousVerifyDelay`).
- Raw `Create` / `Read` / `Patch` / `Delete` / `Search` escape hatches.
- Reference helpers (`@` / `@@`), including `AppendCachedRefOp`, `PatchDetailAppendingCachedRef`, and `ReadResult.SummariesForArrayField`.
- Opt-in two-shard Todo integration demo via `./start_integration_test.sh` (raw and typed coverage).
- User-facing API guide under [`docs/README.md`](docs/README.md) (separate from contributor `tech-docs/`).
- Dependency on ojson for ordered JSON and schema-aware patches, including registered `DatoriumDirectRef` / `DatoriumCachedRef` string formats at `Bind`.

### Changed

- Compatibility target is DatoriumDB `v0.0.2` (HTTP API `v1`).
- Create IDs are always client-supplied: empty/`nil` mints a ULID locally (server no longer accepts `null` create parms). Create command lines are marshaled once so retries keep stable bytes.
- Inbound DB JSON is parsed with ojson (`Result.Env`, `ReadResult` fields, establishment schemas) rather than `map[string]any`.

[Unreleased]: https://github.com/JohnAD/datorium-client-go/compare/v2.0.1...HEAD
[2.0.1]: https://github.com/JohnAD/datorium-client-go/compare/v2.0.0...v2.0.1
[2.0.0]: https://github.com/JohnAD/datorium-client-go/compare/v1.0.1...v2.0.0
[1.0.1]: https://github.com/JohnAD/datorium-client-go/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/JohnAD/datorium-client-go/releases/tag/v1.0.0

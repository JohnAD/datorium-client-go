# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

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

[Unreleased]: https://github.com/JohnAD/datorium-client-go/compare/v1.0.1...HEAD
[1.0.1]: https://github.com/JohnAD/datorium-client-go/compare/v1.0.0...v1.0.1
[1.0.0]: https://github.com/JohnAD/datorium-client-go/releases/tag/v1.0.0

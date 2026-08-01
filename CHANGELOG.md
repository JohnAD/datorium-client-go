# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Compatibility target is DatoriumDB `v0.0.5` (HTTP API `v1` unchanged).
- Todo integration Users schema includes `todoLists` (array of cached TodoList refs) for the O(1) front-page pattern.
- Create IDs are always client-supplied: empty/`Void` mints a ULID locally (server no longer accepts `null` create parms). Create command lines are marshaled once; ambiguous failures may be confirmed with a follow-up read.
- Inbound DB JSON is parsed with ojson: `Result.Env` replaces `Result.Raw`; `ReadResult` SOT/extras/cache summaries, establishment schemas/searches/auth, and APIError expected/actual are `ojson.JSONValue`.
- Typed document API is collection-scoped: `Collection[T].Bind` → `CollectionClient[T]` methods. Package-level `CreateDoc` / `ReadDoc` / `PatchDoc` / `DeleteDoc` / `CompiledSchema` and `TypedRead` are removed.
- Todo integration scenario covers create/read/patch/delete in both raw `Client` and typed `CollectionClient` forms (including `CreatePatchFromChanges` and hand-built `CreatePatch`).
- `Collection.Bind` compiles establishment schemas with registered `DatoriumDirectRef` / `DatoriumCachedRef` ojson string formats.
- `Collection.Bind(ctx, client)` lazy-establishes on first use (validates that collection); explicit `Establish` is optional for whole-catalog startup checks.
- `CollectionClient.DeleteDoc` takes `id` and `version` (e.g. from a `WriteResult`) rather than a `CollectionItem`.

### Added

- Initial smart Go client library for DatoriumDB HTTP API `v1`.
- Establishment caching, CRC32 shard routing, and `wrongMachine` retry.
- Typed create/read/patch/delete/search helpers and reference resolution.
- `AppendCachedRefOp`, `PatchDetailAppendingCachedRef`, and `ReadResult.SummariesForArrayField` for maintaining/reading arrays of `@@` refs.
- Opt-in two-shard Todo integration demonstration via `start_integration_test.sh`.
- Typed collection clients: `Collection[T]`, catalog validation on `Establish`, `Bind`, `CollectionItem` / `CollectionPatch`, ordered `CreateDoc` / `GetDoc` / `DeleteDoc`, schema-enforced `CreatePatchFromChanges` / `CreatePatch` / `PatchDoc`.
- `BuildCommandOrdered` for access-language commands with stable JSON field order.
- `NewDocumentID` and create idempotency helpers (`documentExists` / transport-failure follow-up read via `CreateAmbiguousVerifyDelay`).
- User-facing API guide under [`docs/README.md`](docs/README.md) (separate from contributor `tech-docs/`).
- Depend on ojson with schema-aware patch support.

# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Changed

- Compatibility target is DatoriumDB `v0.0.2` (HTTP API `v1` unchanged).
- Todo integration Users schema includes `todoLists` (array of cached TodoList refs) for the O(1) front-page pattern.

### Added

- Initial smart Go client library for DatoriumDB HTTP API `v1`.
- Establishment caching, CRC32 shard routing, and `wrongMachine` retry.
- Typed create/read/patch/delete/search helpers and reference resolution.
- `AppendCachedRefOp`, `PatchDetailAppendingCachedRef`, and `ReadResult.SummariesForArrayField` for maintaining/reading arrays of `@@` refs.
- Opt-in two-shard Todo integration demonstration via `start_integration_test.sh`.

# Roadmap

Milestones for `github.com/JohnAD/datorium-client-go/v2`.

## Milestone 1 — Transport and single-node CRUD

- Authenticated HTTP client with timeouts and body limits
- Envelope decode (`ok` / `errors`) treating HTTP 200 application failures as typed errors
- `Health`, `Ready`, `Establish`, `Schema`, raw `Command`
- Typed `Create`, `Read`, `Patch`, `Delete`
- Mocked HTTP unit tests

**Done when:** a single-baseURL client can complete CRUD against a mock or single-node server.

## Milestone 2 — Smart routing

- Establishment cache with config version
- CRC32 document shard slots matching DatoriumDB
- Write → `SHARD_SOT_MEMBER`, read → `SHARD_READ_MEMBER`
- Bounded `wrongMachine` retry + host URL rewrite map

**Done when:** multi-baseURL routing unit tests pass and wrong-machine recovery is covered.

## Milestone 3 — Resilience

- Configurable retry/backoff for transport failures
- ULID `operationId` on writes
- Helpers for `versionMismatch` re-read/retry
- Concurrent-safe establishment cache

**Done when:** race tests pass and retry policies are documented.

## Milestone 4 — Full smart features

- Search path encoding + search-shard routing
- `extraFields` / `cacheSummaries` on read
- `@` / `@@` reference parse + direct-ref resolution
- Historic schema fetch helper

**Done when:** search and reference unit tests pass.

## Milestone 5 — Release hardening

- Package docs and examples
- Compatibility matrix for DatoriumDB `v1.1.0`
- Semantic-version release checklist

**Done when:** README/examples/CI green and `CHANGELOG` ready for `v0.1.0`.

## Milestone 6 — Todo integration demonstration

- `start_integration_test.sh` + Compose two-shard (`00-7F` / `80-FF`) Todo fixtures
- Host-built `cmd/todo-integration` exercising CRUD, cached refs, live refs, search
- Unconditional teardown of the Compose project

**Done when:** `./start_integration_test.sh` succeeds on a Docker-capable host with a sibling `datoriumdb` checkout.

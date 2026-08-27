# datorium-client-go documentation

User guide for applications that talk to [DatoriumDB](https://github.com/JohnAD/datoriumdb) through this Go client.

These pages describe **what to call and how**. For library internals, protocol edge cases, and contributor notes, see [`tech-docs/ROADMAP.md`](../tech-docs/ROADMAP.md).

| Page | Contents |
|------|----------|
| [Getting started](getting-started.md) | Install, first client, recommended typed path |
| [Client and config](client.md) | `New`, `Config`, health/ready, establish, schema |
| [Documents](documents.md) | Bound `CollectionClient` and raw create / read / patch / delete |
| [Binary attachments](files.md) | `PutFile` / `DownloadFile` / `ListFiles` / `DeleteFile` |
| [Patch instructions](patches.md) | `CreatePatchFromChanges`, hand-built `ojson.Patch`, path bags |
| [Search](search.md) | Precompiled search helpers |
| [References](references.md) | `@` / `@@` refs, resolve, front-page helpers |
| [Errors](errors.md) | `AppError`, transport errors, common codes |
| [Utilities and packages](utilities.md) | IDs, command builders, `refs` / `searchpath` / `shard` |
| [Testing and integration setup](testing.md) | Unit-test seams, Compose cluster, minting test tokens |

Compatible with DatoriumDB `v1.0.0` / HTTP API `v1`.

Import path: `github.com/JohnAD/datorium-client-go/v2`.

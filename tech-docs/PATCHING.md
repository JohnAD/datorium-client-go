# Patching (library design)

Design notes for typed document patches in `datorium-client-go`.

Application-facing docs:

- [`docs/documents.md`](../docs/documents.md) — `CollectionClient`, `GetDoc`, `PatchDoc`
- [`docs/patches.md`](../docs/patches.md) — `CreatePatchFromChanges`, hand-built ops

ojson patch reference: sibling [`ojson/tech-docs/patch.md`](../../ojson/tech-docs/patch.md) (also on GitHub `main`).

## Recommended app pattern

1. Declare `Collection[T]` and optionally a path bag.
2. `Establish` + `Bind` → `CollectionClient[T]`.
3. `GetDoc` → mutate `item.Doc` → `CreatePatchFromChanges` → `PatchDoc`.

Hand-crafted ops use `CreatePatch(item, ojson.Patch)` so schema validation and id/version attachment stay collection-scoped.

## Two app paths

1. **Diff from edited content** — `CreatePatchFromChanges` (private original baseline vs `Doc`, bound schema).
2. **Hand-crafted ops** — path bag + `ReplaceAt` / … (or raw `ojson.PatchReplace`), then `CreatePatch`.

## Dependency on ojson (locked)

| Capability | Owner |
|------------|--------|
| Apply / Diff / Validate RFC6902 with schema | ojson |
| Typed paths / path generation | ojson |
| Ordered object `value`s in ops | ojson |
| Datorium `$` / `#` / access-language / routing / binding identity | **this client** |

ojson does **not** treat `#` / `$` / `!` as special. Client and server share the same patch type.

## Current state

| Layer | Status |
|-------|--------|
| Raw `Client.Patch` / `PatchWithVersionRetry` | Implemented (map-based escape hatch) |
| ojson schema-aware RFC6902 | Available |
| `CollectionClient` + `CreatePatchFromChanges` / `CreatePatch` / `PatchDoc` | **Implemented** |
| Binding identity (items/patches) | **Implemented** |
| Typed `PatchWithVersionRetry` | Not yet |
| Front-page helpers | Still raw RFC6902 maps |

## Open follow-ups

1. Typed equivalent of `PatchWithVersionRetry` on `CollectionClient`
2. Migrate `AppendCachedRefOp` onto ojson / `CollectionClient`

## Wire reminder (server)

Access-language patch detail must include `$`, `#`, and `RFC6902: [...]`. See DatoriumDB `ACCESS-LANGUAGE.md`. This client sets those from the collection marker and patch version; ops come from `ojson.Patch.ToJSONValue()`.

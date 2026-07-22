# Patch instructions

How to patch documents through a bound [`CollectionClient`](documents.md).

Under the hood this is [RFC6902](https://datatracker.ietf.org/doc/html/rfc6902) via [`ojson`](https://github.com/JohnAD/ojson) (ordered JSON, schema validation). Full patch model: ojson [`tech-docs/patch.md`](https://github.com/JohnAD/ojson/blob/main/tech-docs/patch.md). DatoriumDB applies the same ojson patch type on the server.

## Edit then diff (recommended)

```go
todos, err := Todos.Bind(client)

item, err := todos.GetDoc(ctx, id)
if err != nil {
    return err
}

item.Doc.Status = "done"

patch, err := todos.CreatePatchFromChanges(item)
if err != nil {
    return err
}

wr, err := todos.PatchDoc(ctx, patch)
```

`CreatePatchFromChanges` diffs the item's private original content baseline against the current `Doc` using the schema compiled at `Bind`. Mutating `OriginalDoc` does not affect that baseline. Empty diffs are rejected. Version conflicts stay explicit (`versionMismatch`); local edits are not auto-rebased.

## Hand-built ojson ops

Mirror fields in a path bag (`todoPath.Status`), or use string pointers. Wrap with `CreatePatch` so the collection validates against the original document + schema and attaches id/version:

```go
var todoPath = struct {
    Title  ojson.TypedPath[Todo, string]
    Status ojson.TypedPath[Todo, string]
}{
    Title:  ojson.NewTypedPath[Todo, string]("/title"),
    Status: ojson.NewTypedPath[Todo, string]("/status"),
}

item, err := todos.GetDoc(ctx, id)

op, err := ojson.ReplaceAt(todoPath.Status, "done")
raw, err := ojson.NewPatch(op)

patch, err := todos.CreatePatch(item, raw)
wr, err := todos.PatchDoc(ctx, patch)
```

Runtime constructors (no path bag):

```go
raw, err := ojson.NewPatch(
    ojson.PatchReplace("/status", ojson.NewString("done")),
    ojson.PatchAdd("/tags/-", ojson.NewString("urgent")),
    ojson.PatchRemove("/tags/0"),
    ojson.PatchTest("/status", ojson.NewString("open")),
)
patch, err := todos.CreatePatch(item, raw)
```

Object values in ops must be built with ojson (not `map[string]any`) so field order stays stable in git-tracked documents.

Helpers on paths: `ReplaceAt`, `AddAt`, `TestAt`, `RemoveAt`, `MoveAt`, `CopyAt`, plus `Index` / `Append` / `Child` for nested or array paths.

## `CollectionPatch`

| API | Purpose |
|-----|---------|
| `CreatePatchFromChanges(item)` | Diff original baseline → `item.Doc` with bound schema |
| `CreatePatch(item, ojson.Patch)` | Validate a hand-built patch against original + schema |
| `PatchDoc(ctx, collectionPatch)` | Send `$` / `#` / `RFC6902` for the patch's id and version |
| `CollectionPatch.ID()` / `.Version()` | Read-only accessors |
| `CollectionPatch.Patch` | Underlying `ojson.Patch` |
| `CompiledSchema()` | Schema compiled at `Bind` (advanced ojson use) |

Items and patches carry a private binding identity. Using them with another `CollectionClient` (even another `Bind` of the same descriptor) errors.

## Related

- [`documents.md`](documents.md) — bind, get, create, delete
- Library design: [`tech-docs/PATCHING.md`](../tech-docs/PATCHING.md)
- ojson: [`tech-docs/patch.md`](https://github.com/JohnAD/ojson/blob/main/tech-docs/patch.md)

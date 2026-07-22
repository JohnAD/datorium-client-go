# Documents

Create, read, patch, and delete documents. Prefer the **typed collection client** for application code; use **raw** helpers as an escape hatch.

## Typed API (recommended)

Declare a catalog descriptor, establish, bind a typed client, then call methods:

```go
var Todos = datorium.MustCollection[Todo]("Todos", 0)

if err := client.Establish(ctx, Todos); err != nil {
    return err
}
todos, err := Todos.Bind(client)

wr, err := todos.CreateDoc(ctx, nil, todo)
item, err := todos.GetDoc(ctx, id)
patch, err := todos.CreatePatchFromChanges(item) // or CreatePatch(item, ojsonPatch)
wr, err = todos.PatchDoc(ctx, patch)
wr, err = todos.DeleteDoc(ctx, item)
```

### Declare a collection

Binds a Go struct type to a collection name and schema version. Declare once; pass into `Establish`, then `Bind`.

```go
type Todo struct {
    Title  string `json:"title"`
    Status string `json:"status"`
}

var Todos = datorium.MustCollection[Todo]("Todos", 0)
// Todos.SchemaMarker() == "Todos:0"
```

`MustCollection` panics on empty name or negative version.

Do **not** put `$` / `!` / `#` on content structs for normal creates — typed create injects `$` from the collection. After a read, those system fields are on `DocMeta`, not on `T`.

### Bind

```go
func (col Collection[T]) Bind(c *Client) (CollectionClient[T], error)
```

Requires a successful `Establish`. Verifies the live schema name/version and compiles the ojson schema (including registered `DatoriumDirectRef` / `DatoriumCachedRef` string formats). Returns a `CollectionClient[T]` whose items and patches cannot be used with another binding (including another bind of the same descriptor).

### CreateDoc

```go
func (cc CollectionClient[T]) CreateDoc(ctx context.Context, id *string, doc T) (WriteResult, error)
```

```go
// Auto id
wr, err := todos.CreateDoc(ctx, nil, Todo{
    Title: "Buy milk", Status: "open",
})
fmt.Println(wr.ID, wr.Version)

// Explicit id
myID := "todo-42"
wr, err = todos.CreateDoc(ctx, &myID, Todo{
    Title: "Buy milk", Status: "open",
})
```

- Serializes `doc` to ordered JSON for storage (struct field declaration order).
- Sets `"$"` from the collection (errors if `doc` already set a disagreeing `$`).

### GetDoc / GetDocOpts

```go
func (cc CollectionClient[T]) GetDoc(ctx context.Context, id string) (CollectionItem[T], error)
func (cc CollectionClient[T]) GetDocOpts(ctx context.Context, id string, opts *ReadOptions) (CollectionItem[T], error)
```

```go
item, err := todos.GetDoc(ctx, wr.ID)
fmt.Println(item.Doc.Title, item.Meta.Version)
```

```go
type CollectionItem[T any] struct {
    Doc         T // mutable working copy
    OriginalDoc T // independently decoded read-time content
    Meta        DocMeta
    Result      Result
    // ExtraFields and CacheSummaries when requested via GetDocOpts
}

type DocMeta struct {
    ID      string // document id (!)
    Schema  string // schema marker ($)
    Version string // version (#) — needed for patch/delete
}
```

### DeleteDoc

```go
func (cc CollectionClient[T]) DeleteDoc(ctx context.Context, item CollectionItem[T]) (WriteResult, error)
```

```go
_, err := todos.DeleteDoc(ctx, item)
```

Uses `item.Meta.ID` / `item.Meta.Version` and rejects items from another binding.

### WriteResult

Summary of a successful create, patch, or delete:

```go
type WriteResult struct {
    Result        Result // raw envelope if you need it
    Collection    string
    ID            string
    Schema        string
    Version       string // new version after the write
    VersionBefore string // patch only
    OperationID   string // echoed by the server for this write
}
```

### PatchDoc

See [`patches.md`](patches.md). Short form:

```go
item, err := todos.GetDoc(ctx, id)
item.Doc.Status = "done"
patch, err := todos.CreatePatchFromChanges(item)
wr, err := todos.PatchDoc(ctx, patch)
fmt.Println(wr.Version)
```

---

## Raw API

### Create

Creates a document from a `map[string]any` body. Empty `id` mints a ULID. Usually include `"$"` (schema marker). Returns [`WriteResult`](#writeresult).

```go
func (c *Client) Create(ctx context.Context, collection, id string, content map[string]any)
```

→ ([`WriteResult`](#writeresult), error)

### Read

Reads a document into loosely typed ojson values (`SOT`, optional extras).

```go
func (c *Client) Read(ctx context.Context, collection, id string, opts *ReadOptions) (ReadResult, error)

type ReadOptions struct {
    ExtraFields    bool
    CacheSummaries bool
}

type ReadResult struct {
    Result         Result
    Collection     string
    ID             string
    SOT            ojson.JSONValue
    ExtraFields    ojson.JSONValue
    CacheSummaries ojson.JSONValue
}
```

### Patch

Applies RFC6902 ops. `detail` must include `"$"` and `"#"` (current version), and usually `"RFC6902"`.

```go
func (c *Client) Patch(ctx context.Context, collection, id string, detail map[string]any)
```

→ ([`WriteResult`](#writeresult), error)

`PatchWithVersionRetry` reads, builds a patch from SOT, patches; on `versionMismatch`, re-reads and retries once:

```go
func (c *Client) PatchWithVersionRetry(
    ctx context.Context,
    collection, id string,
    build func(sot ojson.JSONValue) (map[string]any, error),
)
```

→ ([`WriteResult`](#writeresult), error)

### Delete

Deletes by id. `detail` must include `"#"`; `"$"` is recommended.

```go
func (c *Client) Delete(ctx context.Context, collection, id string, detail map[string]any)
```

→ ([`WriteResult`](#writeresult), error)

---

## Order safety

| Path | Document field order |
|------|----------------------|
| Typed (`CollectionClient` create/patch) | Stable (struct / ojson declaration order) |
| Raw `map[string]any` | **Not** stable across remarshals |

DatoriumDB honors client order for non-schema fields when persisting JSON. Prefer typed APIs for anything that lands in git-tracked documents.

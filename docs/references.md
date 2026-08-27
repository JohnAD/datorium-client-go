# References and front-page helpers

DatoriumDB documents can point at other documents with reference strings:

| Form | Meaning |
|------|---------|
| `@__Collection__id` | Direct (live) reference |
| `@@__Collection__id` | Cached reference (summary stored on the referring document’s cache) |

## Package `refs`

```go
import "github.com/JohnAD/datorium-client-go/v2/refs"

r, ok, err := refs.Parse(s)
direct := refs.FormatDirect("TodoLists", listID)   // @__TodoLists__…
cached := refs.FormatCached("TodoLists", listID)   // @@__TodoLists__…
```

```go
type Ref struct {
    Kind       Kind // Direct or Cached
    Collection string
    ID         string
    Raw        string
}
```

Non-reference strings return `ok == false` without error.

## Resolve helpers (`datorium`)

```go
func (c *Client) ResolveDirectRef(ctx context.Context, ref string, opts *ReadOptions) (ReadResult, error)
```

Parses a direct `@__…` ref and `Read`s the target.

```go
func (c *Client) ResolveRefsInSOT(ctx context.Context, sot ojson.JSONValue, maxDepth int) (map[string]ReadResult, error)
```

Walks top-level string fields in a SOT object, resolves direct refs, optionally recurses (`maxDepth`; minimum 1). Returns `collection/id` → `ReadResult`.

## Front-page pattern (arrays of `@@` refs)

Typical flow: a “home” document holds an array of cached refs (for example `Users.todoLists`). One read with `cacheSummaries: true` yields ordered summaries without N+1 reads.

### Append a cached ref (patch helper)

```go
func AppendCachedRefOp(arrayField, collection, id string) map[string]any

func PatchDetailAppendingCachedRef(
    schemaMarker, version, arrayField, refCollection, refID string,
) map[string]any
```

Example:

```go
detail := datorium.PatchDetailAppendingCachedRef(
    userRR.SOT.Get("$").ToStringOrEmpty(),
    userRR.SOT.Get("#").ToStringOrEmpty(),
    "todoLists",
    "TodoLists",
    listID,
)
_, err = client.Patch(ctx, "Users", userID, detail)
```

### Read summaries in array order

```go
rr, err := client.Read(ctx, "Users", userID, &datorium.ReadOptions{CacheSummaries: true})
summaries, err := rr.SummariesForArrayField("todoLists")
```

`SummariesForArrayField` walks `sot[arrayField]`, keeps `@@` refs, and returns matching cache summary objects (`[]ojson.JSONValue`) in array order (skips missing / unresolved / stub `#:null` entries).

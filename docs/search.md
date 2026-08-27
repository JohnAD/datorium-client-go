# Search

```go
func (c *Client) Search(
    ctx context.Context,
    collection, searchName string,
    vars map[string]any,
    pathSegments []string,
) (SearchResult, error)

type SearchResult struct {
    Result     Result
    Collection string
    Search     string
    Matches    []string // document ids
}
```

Runs a **precompiled** search defined in establishment config.

## Routing with path segments

For equals-string style searches, build path segments so the client can CRC32-route to the correct search shard:

```go
import "github.com/JohnAD/datorium-client-go/v2/searchpath"

segs := searchpath.EqualsStringSegments("done") // example: status value
sr, err := client.Search(ctx, "Todos", "byStatus", map[string]any{
    "status": "done",
}, segs)
```

If `pathSegments` is `nil`, the command is sent to `PreferServer` / the establishment server and may bounce with `wrongMachine` until it lands correctly.

## Helpers (`searchpath`)

| Function | Purpose |
|----------|---------|
| `EqualsStringSegments(values ...string)` | Encode string equals clauses for routing |
| `EncodeStringValue(s string)` | Encode one string path component |
| `EncodeTruth(b bool)` | Encode a boolean path component |
| `EncodeNull` | Literal `"null"` component |
| `ShardSlot(segments []string)` | CRC32 slot used for search routing |

Search definitions and semantics live in the DatoriumDB server docs (`SEARCHING.md`). This client only issues the command and routes it.

# Utilities and packages

## Document and operation IDs

```go
func NewDocumentID() string  // ULID for create document ids
func NewOperationID() string // ULID for write operationId fields
```

Prefer letting raw `Create` / typed `CollectionClient.CreateDoc` mint document ids (`""` / `nil`). Call `NewDocumentID` yourself when you need the id before the create call (for example to embed it elsewhere in the same transaction of work).

## JSON command helpers

```go
func BuildCommand(word, target, parm string, detail any) ([]byte, error)
func BuildCommandOrdered(word, target, parm string, detail ojson.JSONValue) ([]byte, error)

func (c *Client) Command(ctx context.Context, baseURL string, body []byte) (Result, error)
```

- `BuildCommand` — builds `{"command","target","parameter","detail"}` with
  `encoding/json` for `detail` (maps are order-unsafe).
- `BuildCommandOrdered` — splices an ojson object so `detail` keeps stable
  field order (used by typed writes).
- `Command` — posts a pre-built JSON body without smart routing (`baseURL`
  empty → establishment URL).

Normal apps should use a bound `CollectionClient` (`CreateDoc` / `GetDoc` /
`PatchDoc` / `DeleteDoc`) or raw `Create` / `Read` / `Patch` / `Delete` /
`Search` / file helpers instead of hand-building bodies.

## Subpackages

### `refs`

Parse and format `@__Collection__id` / `@@__Collection__id`. See [References](references.md).

### `searchpath`

Encode search path segments and compute the search shard slot. See [Search](search.md).

### `shard`

Low-level CRC32 document-id slot and range helpers (`Slot`, `ParseRange`, `ValidateFullCoverage`). The smart client uses these internally for routing; most applications do not import `shard` directly.

## What this guide is not

Protocol wire formats, sharding theory, auth JWT minting, and release process live under [`tech-docs/ROADMAP.md`](../tech-docs/ROADMAP.md) (and sibling files there) and in the DatoriumDB server repository. Use those when you are changing this library or operating the database itself.

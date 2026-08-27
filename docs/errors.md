# Errors

The HTTP status is often still `200` for application failures. Always check the returned `error` (and, when present, envelope `ok` / `errors`).

## Application errors

```go
type AppError struct {
    Code          string
    Message       string
    Errors        []APIError
    Result        Result
    ShardSlot     string
    CorrectServer string
    BaseURL       string
    ConfigVersion int
    Collection    string
    ID            string
    Command       string
}

func IsAppCode(err error, code string) bool
```

### Common codes

| Constant | Code string | Typical meaning |
|----------|-------------|-----------------|
| `CodeWrongMachine` | `wrongMachine` | Routed to the wrong shard member (client retries automatically) |
| `CodeVersionMismatch` | `versionMismatch` | Patch/delete `#` was stale |
| `CodeDocumentNotFound` | `documentNotFound` | Missing document |
| `CodeDocumentExists` | `documentExists` | Create id already taken (client may treat as idempotent success after read) |
| `CodeUnauthenticated` | `unauthenticated` | Missing/invalid auth |
| `CodeInvalidToken` | `invalidToken` | Bad token |
| `CodeTokenExpired` | `tokenExpired` | Expired token |
| `CodeDocumentStale` | `documentStale` | Stale document view |
| `CodeReadMemberStale` | `readMemberStale` | Read member behind |
| `CodeSearchNotFound` | `searchNotFound` | Unknown search |
| `CodeFileNotFound` | `fileNotFound` | Missing binary attachment |
| `CodeFileExists` | `fileExists` | `fileCreate` when filename already exists |
| `CodeFileVersionMismatch` | `fileVersionMismatch` | Stale file `version` on update/delete |
| `CodeInvalidFileName` | `invalidFileName` | Illegal attachment basename |
| `CodeFileTooLarge` | `fileTooLarge` | Upload exceeds `maxFileBytes` |
| `CodeFileStale` | `fileStale` | Attachment pending catch-up on read member |
| `CodeContentTypeRequired` | `contentTypeRequired` | Missing content type where required |
| `CodeInvalidRequest` | `invalidRequest` | Malformed command / detail |
| `CodeAdminRequired` | `adminRequired` | Catalog ensure without admin JWT |
| `CodeEstablishmentRequired` | `establishmentRequired` | Admin command not sent to establishment server |
| `CodeSchemaDrift` | `schemaDrift` | `collectionEnsure` schema does not match live |

Example:

```go
wr, err := client.Patch(ctx, "Todos", id, detail)
if datorium.IsAppCode(err, datorium.CodeVersionMismatch) {
    // re-read and retry, or use PatchWithVersionRetry
}
```

## Catalog errors

```go
type CatalogError struct {
    Mismatches []CatalogMismatch
}
```

Returned by `Establish(ctx, cols...)` or a lazy `Collection.Bind` when a declared collection is missing or its schema version does not match the live establishment document. Fix the binary’s catalog (or deploy the matching server schemas) before shipping.

## Transport errors

```go
type TransportError struct {
    StatusCode int
    Body       string
    Err        error
}
```

Non-application failures: network errors, unexpected HTTP statuses, oversized bodies, envelope decode failures.

After a **create** transport failure, the client may wait (`CreateAmbiguousVerifyDelay`) and read by id to see if the write actually committed.

## Envelope `Result`

```go
type Result struct {
    OK     bool
    Errors []APIError
    Env    ojson.JSONValue // ordered envelope parse
    Body   []byte
}

func DecodeResult(body []byte) (Result, error)
func (r Result) FirstErrorCode() string
func (r Result) StringField(key string) string
func (r Result) ValueField(key string) ojson.JSONValue
func (r Result) IntField(key string) int
func (r Result) BoolField(key string) bool
```


Most callers use the typed `WriteResult` / `CollectionItem` wrappers (or raw `ReadResult`) instead of digging through `Result` unless they need a rare envelope field.

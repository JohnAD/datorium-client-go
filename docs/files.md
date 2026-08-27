# Binary attachments

Binary attachments use the same `POST /datoriumdb/v1/command` endpoint as
document commands. Uploads are multipart; downloads are JSON `fileRead` with a
raw streamed success body.

```go
f, err := os.Open("poster.png")
if err != nil { /* ... */ }
defer f.Close()

wr, err := client.PutFile(ctx, "Movies", docID, "poster.png", f, &datorium.PutFileOptions{
    ContentType: "image/png",
})
// wr.DistributionComplete is a freshness hint only.

var buf bytes.Buffer
meta, err := client.DownloadFile(ctx, "Movies", docID, "poster.png", &buf)

list, err := client.ListFiles(ctx, "Movies", docID)

_, err = client.DeleteFile(ctx, "Movies", docID, "poster.png", meta.Version)
```

`ListFiles` returns the parent document’s current attachment list only (no
file bytes): a `[]FileMetadata` with one entry per file. Soft-deleted or
pending-only names are omitted. `ETag` is empty on list entries (it is set
from download response headers).

```go
type FileMetadata struct {
    Name        string // file name; pass to DownloadFile / DeleteFile / PutFile
    ContentType string
    ByteSize    int64
    SHA256      string
    Version     string // optimistic concurrency token for update/delete
    OperationID string
    ETag        string // download headers only; empty from ListFiles
}
```

Uploads must be seekable (`io.ReadSeeker`) or supply `PutFileOptions.Reopen`
so wrongMachine / routing retries can resend. `IfMatch` empty selects
`fileCreate`; a non-empty `IfMatch` selects `fileUpdate` with
`detail.version`. Downloads stream without the JSON 8 MiB response cap.
Errors still use JSON envelopes.

See DatoriumDB `tech-docs/BINARY-FILES.md` for server semantics.

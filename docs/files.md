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

// Single-range read (DatoriumDB v1.1.0+); may return 206 or full 200.
rangeMeta, err := client.DownloadFileRange(ctx, "Movies", docID, "poster.png", "bytes=0-1023", &buf)

list, err := client.ListFiles(ctx, "Movies", docID)

wr, err = client.DeleteFile(ctx, "Movies", docID, "poster.png", meta.Version)
```

`PutFile` and `DeleteFile` return a `FileWriteResult` summarizing the write
(filename, new `Version`, size, hash, `DistributionComplete`, optional `Note`).
`DownloadFile` returns `FileMetadata` plus the bytes written to your `io.Writer`.
`DownloadFileRange` accepts one standard byte range (`bytes=0-1023`,
`bytes=1024-`, or `bytes=-512`) for video seeking and resumable reads, and
returns `FileDownloadMetadata`.
`DownloadFileWithOptions` additionally provides `OnResponse`, called with
validated metadata before the first byte is written so an HTTP proxy can set
its status and response headers without buffering the attachment.

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

type FileDownloadMetadata struct {
    FileMetadata
    StatusCode int
    ContentRange string
    AcceptRanges string
    TotalByteSize int64 // complete file size; ByteSize is this response size
}

type FileWriteResult struct {
    Result               Result
    Command              string
    Collection           string
    ID                   string
    Filename             string
    Version              string
    ByteSize             int64
    SHA256               string
    ContentType          string
    OperationID          string
    DistributionComplete bool
    Note                 *ReplicationNote // nil when omitted
}
```

Uploads must be seekable (`io.ReadSeeker`) or supply `PutFileOptions.Reopen`
so wrongMachine / routing retries can resend. `IfMatch` empty selects
`fileCreate`; a non-empty `IfMatch` selects `fileUpdate` with
`detail.version`. Downloads stream without the JSON 8 MiB response cap.
For a 206 response, `ByteSize` is the selected range's `Content-Length`; for a
200 response it is the complete response length. `TotalByteSize` comes from
`Content-Range` when available. Servers may ignore `Range` and return the full
file with 200, following standard HTTP behavior.

Unsatisfiable ranges normally return 416. If the response contains a DatoriumDB
error envelope, the client returns its `AppError` (normally `invalidRange`);
otherwise it returns `TransportError`. In either case `DownloadFileRange`
returns status/range metadata, including `Content-Range: bytes */size`, without
writing the error body to the destination. Error envelopes received during a
`wrongMachine` retry are likewise never written.

See DatoriumDB `tech-docs/BINARY-FILES.md` for server semantics.

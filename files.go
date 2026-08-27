package datorium

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strconv"
	"strings"

	"github.com/JohnAD/ojson"
)

// FileMetadata describes one attachment (list entry or download headers).
type FileMetadata struct {
	Name        string
	ContentType string
	ByteSize    int64
	SHA256      string
	Version     string
	OperationID string
	ETag        string
}

// FileWriteResult is a successful fileCreate / fileUpdate / fileDelete summary.
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
	// Note is set when the SOT write succeeded but file replication was
	// incomplete; nil when omitted (typically when DistributionComplete).
	Note *ReplicationNote
}

// PutFileOptions configures a binary attachment create or update.
type PutFileOptions struct {
	// ContentType is sent in detail.contentType and on the multipart content part.
	ContentType string
	// IfMatch is the current file version required for update. Empty means
	// create-only (server rejects if the file already exists).
	IfMatch string
	// OperationID optionally supplies a client-chosen operation id.
	OperationID string
	// ContentLength, when > 0, sets Content-Length on the upload content part.
	ContentLength int64
	// Reopen returns a fresh body reader for each attempt. Required when body
	// is not an io.ReadSeeker so wrongMachine / transport retries can resend.
	Reopen func() (io.ReadCloser, error)
}

// PutFile creates or updates a binary attachment for an existing parent document.
// Writes route to the shard SOT member. body must be an io.ReadSeeker, or opts.Reopen
// must be set, so retries can resend the same bytes.
func (c *Client) PutFile(ctx context.Context, collection, docID, filename string, body io.Reader, opts *PutFileOptions) (FileWriteResult, error) {
	if collection == "" || docID == "" || filename == "" {
		return FileWriteResult{}, fmt.Errorf("datorium: collection, docID, and filename are required")
	}
	if err := requireUploadSource(body, opts); err != nil {
		return FileWriteResult{}, err
	}
	est, err := c.ensureEstablished(ctx)
	if err != nil {
		return FileWriteResult{}, err
	}
	route, err := c.routeDocument(est, docID, RouteWrite)
	if err != nil {
		return FileWriteResult{}, err
	}

	command := "fileCreate"
	contentType := "application/octet-stream"
	var contentLen int64
	detail := map[string]any{"filename": filename}
	if opts != nil {
		if opts.ContentType != "" {
			contentType = opts.ContentType
			detail["contentType"] = contentType
		}
		if opts.OperationID != "" {
			detail["operationId"] = opts.OperationID
		}
		if opts.IfMatch != "" {
			command = "fileUpdate"
			detail["version"] = opts.IfMatch
		}
		contentLen = opts.ContentLength
	}
	detail = ensureOperationID(detail)
	cmdBody, err := BuildCommand(command, collection, docID, detail)
	if err != nil {
		return FileWriteResult{}, err
	}

	res, err := c.executeRoutedMultipart(ctx, route, cmdBody, contentType, contentLen, func() (io.Reader, func(), error) {
		return openUploadBody(body, opts)
	}, func(est *Establishment) (Route, error) {
		return c.routeDocument(est, docID, RouteWrite)
	})
	if err != nil {
		return FileWriteResult{}, err
	}
	return fileWriteResultFrom(res), nil
}

// DownloadFile streams a binary attachment to w. Reads route to a shard read member.
// On success, metadata comes from response headers (not the JSON 8 MiB path).
func (c *Client) DownloadFile(ctx context.Context, collection, docID, filename string, w io.Writer) (FileMetadata, error) {
	if collection == "" || docID == "" || filename == "" {
		return FileMetadata{}, fmt.Errorf("datorium: collection, docID, and filename are required")
	}
	if w == nil {
		return FileMetadata{}, fmt.Errorf("datorium: writer is required")
	}
	est, err := c.ensureEstablished(ctx)
	if err != nil {
		return FileMetadata{}, err
	}
	cmdBody, err := BuildCommand("fileRead", collection, docID, map[string]any{"filename": filename})
	if err != nil {
		return FileMetadata{}, err
	}
	route, err := c.routeDocument(est, docID, RouteRead)
	if err != nil {
		return FileMetadata{}, err
	}
	return c.executeRoutedFileDownload(ctx, route, cmdBody, filename, w, func(est *Establishment) (Route, error) {
		return c.routeDocument(est, docID, RouteRead)
	})
}

// ListFiles returns the attachment manifest for a document (metadata only).
func (c *Client) ListFiles(ctx context.Context, collection, docID string) ([]FileMetadata, error) {
	if collection == "" || docID == "" {
		return nil, fmt.Errorf("datorium: collection and docID are required")
	}
	est, err := c.ensureEstablished(ctx)
	if err != nil {
		return nil, err
	}
	body, err := BuildCommand("fileList", collection, docID, map[string]any{})
	if err != nil {
		return nil, err
	}
	route, err := c.routeDocument(est, docID, RouteRead)
	if err != nil {
		return nil, err
	}
	res, err := c.executeRouted(ctx, route, body, func(est *Establishment) (Route, error) {
		return c.routeDocument(est, docID, RouteRead)
	})
	if err != nil {
		return nil, err
	}
	return fileListFrom(res), nil
}

// DeleteFile deletes a binary attachment. version is required (detail.version).
func (c *Client) DeleteFile(ctx context.Context, collection, docID, filename, version string) (FileWriteResult, error) {
	if collection == "" || docID == "" || filename == "" {
		return FileWriteResult{}, fmt.Errorf("datorium: collection, docID, and filename are required")
	}
	if version == "" {
		return FileWriteResult{}, fmt.Errorf("datorium: file version is required")
	}
	est, err := c.ensureEstablished(ctx)
	if err != nil {
		return FileWriteResult{}, err
	}
	detail := ensureOperationID(map[string]any{
		"filename": filename,
		"version":  version,
	})
	body, err := BuildCommand("fileDelete", collection, docID, detail)
	if err != nil {
		return FileWriteResult{}, err
	}
	route, err := c.routeDocument(est, docID, RouteWrite)
	if err != nil {
		return FileWriteResult{}, err
	}
	res, err := c.executeRouted(ctx, route, body, func(est *Establishment) (Route, error) {
		return c.routeDocument(est, docID, RouteWrite)
	})
	if err != nil {
		return FileWriteResult{}, err
	}
	return fileWriteResultFrom(res), nil
}

func requireUploadSource(body io.Reader, opts *PutFileOptions) error {
	if body == nil && (opts == nil || opts.Reopen == nil) {
		return fmt.Errorf("datorium: PutFile body is required")
	}
	if opts != nil && opts.Reopen != nil {
		return nil
	}
	if _, ok := body.(io.ReadSeeker); ok {
		return nil
	}
	return fmt.Errorf("datorium: PutFile body must be io.ReadSeeker or provide PutFileOptions.Reopen for retries")
}

func openUploadBody(body io.Reader, opts *PutFileOptions) (io.Reader, func(), error) {
	noop := func() {}
	if opts != nil && opts.Reopen != nil {
		rc, err := opts.Reopen()
		if err != nil {
			return nil, noop, err
		}
		return rc, func() { _ = rc.Close() }, nil
	}
	rs, ok := body.(io.ReadSeeker)
	if !ok {
		return nil, noop, fmt.Errorf("datorium: PutFile body must be io.ReadSeeker or provide PutFileOptions.Reopen for retries")
	}
	if _, err := rs.Seek(0, io.SeekStart); err != nil {
		return nil, noop, err
	}
	return rs, noop, nil
}

type bodyOpener func() (io.Reader, func(), error)

func (c *Client) executeRoutedMultipart(
	ctx context.Context,
	initial Route,
	commandJSON []byte,
	contentType string,
	contentLength int64,
	openBody bodyOpener,
	resolve routeResolver,
) (Result, error) {
	base := initial.BaseURL
	if base == "" {
		base = c.cfg.EstablishmentURL
	}
	var last Result
	for attempt := 0; attempt <= c.wmRetries; attempt++ {
		res, err := c.doMultipartCommand(ctx, base, commandJSON, contentType, contentLength, openBody)
		if err != nil {
			return Result{}, err
		}
		last = res
		if res.OK {
			return res, nil
		}
		ae := appErrorFromResult(res)
		if ae.Code != CodeWrongMachine {
			return res, ae
		}
		if attempt == c.wmRetries {
			return res, ae
		}
		if err := c.Establish(ctx); err != nil {
			return res, err
		}
		est := c.cache.get()
		if est == nil || resolve == nil {
			return res, ae
		}
		route, err := resolve(est)
		if err != nil {
			return res, err
		}
		if route.BaseURL == "" {
			return res, ae
		}
		base = route.BaseURL
	}
	return last, appErrorFromResult(last)
}

func (c *Client) executeRoutedFileDownload(
	ctx context.Context,
	initial Route,
	commandJSON []byte,
	filename string,
	w io.Writer,
	resolve routeResolver,
) (FileMetadata, error) {
	base := initial.BaseURL
	if base == "" {
		base = c.cfg.EstablishmentURL
	}
	for attempt := 0; attempt <= c.wmRetries; attempt++ {
		meta, res, isJSON, err := c.doFileDownloadOnce(ctx, base, commandJSON, filename, w)
		if err != nil {
			return FileMetadata{}, err
		}
		if !isJSON {
			return meta, nil
		}
		ae := appErrorFromResult(res)
		if ae.Code != CodeWrongMachine {
			return FileMetadata{}, ae
		}
		if attempt == c.wmRetries {
			return FileMetadata{}, ae
		}
		if err := c.Establish(ctx); err != nil {
			return FileMetadata{}, err
		}
		est := c.cache.get()
		if est == nil || resolve == nil {
			return FileMetadata{}, ae
		}
		route, err := resolve(est)
		if err != nil {
			return FileMetadata{}, err
		}
		if route.BaseURL == "" {
			return FileMetadata{}, ae
		}
		base = route.BaseURL
	}
	return FileMetadata{}, fmt.Errorf("datorium: download exhausted wrongMachine retries")
}

func (c *Client) doMultipartCommand(ctx context.Context, baseURL string, commandJSON []byte, contentType string, contentLength int64, openBody bodyOpener) (Result, error) {
	pr, pw := io.Pipe()
	mw := multipart.NewWriter(pw)
	go func() {
		var err error
		defer func() {
			if err != nil {
				_ = pw.CloseWithError(err)
				return
			}
			_ = pw.Close()
		}()

		cmdPart, err := mw.CreateFormField("command")
		if err != nil {
			return
		}
		if _, err = cmdPart.Write(commandJSON); err != nil {
			return
		}

		contentPartHeader := make(textproto.MIMEHeader)
		contentPartHeader.Set("Content-Disposition", `form-data; name="content"`)
		if contentType != "" {
			contentPartHeader.Set("Content-Type", contentType)
		}
		if contentLength > 0 {
			contentPartHeader.Set("Content-Length", strconv.FormatInt(contentLength, 10))
		}
		contentPart, err := mw.CreatePart(contentPartHeader)
		if err != nil {
			return
		}
		if openBody != nil {
			body, closer, openErr := openBody()
			if openErr != nil {
				err = openErr
				return
			}
			if closer != nil {
				defer closer()
			}
			if body != nil {
				if _, err = io.Copy(contentPart, body); err != nil {
					return
				}
			}
		}
		err = mw.Close()
	}()

	reqURL := strings.TrimRight(baseURL, "/") + apiPrefix + "/command"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, pr)
	if err != nil {
		return Result{}, &TransportError{Err: err}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("User-Agent", c.userAgent)
	tok, err := c.bearer(ctx)
	if err != nil {
		return Result{}, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, &TransportError{Err: err}
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, maxResponseBodyBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return Result{}, &TransportError{StatusCode: resp.StatusCode, Err: err}
	}
	if len(data) > maxResponseBodyBytes {
		return Result{}, &TransportError{StatusCode: resp.StatusCode, Err: fmt.Errorf("response body exceeds %d bytes", maxResponseBodyBytes)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, &TransportError{StatusCode: resp.StatusCode, Body: string(data)}
	}
	res, err := DecodeResult(data)
	if err != nil {
		return Result{}, &TransportError{StatusCode: resp.StatusCode, Body: string(data), Err: err}
	}
	return res, nil
}

func (c *Client) doFileDownloadOnce(ctx context.Context, baseURL string, commandJSON []byte, filename string, w io.Writer) (meta FileMetadata, res Result, isJSON bool, err error) {
	reqURL := strings.TrimRight(baseURL, "/") + apiPrefix + "/command"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, reqURL, bytes.NewReader(commandJSON))
	if err != nil {
		return FileMetadata{}, Result{}, false, &TransportError{Err: err}
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "*/*")
	req.Header.Set("User-Agent", c.userAgent)
	tok, err := c.bearer(ctx)
	if err != nil {
		return FileMetadata{}, Result{}, false, err
	}
	req.Header.Set("Authorization", "Bearer "+tok)
	resp, err := c.http.Do(req)
	if err != nil {
		return FileMetadata{}, Result{}, false, &TransportError{Err: err}
	}
	defer resp.Body.Close()

	// Success streams set file metadata headers; errors are JSON envelopes
	// (often still HTTP 200). Prefer metadata headers over Content-Type so a
	// stored application/json attachment is not treated as an error.
	if resp.Header.Get("X-DatoriumDB-File-Version") != "" || resp.Header.Get("X-DatoriumDB-SHA256") != "" {
		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			return FileMetadata{}, Result{}, false, &TransportError{StatusCode: resp.StatusCode, Err: fmt.Errorf("unexpected status for file download")}
		}
		meta = fileMetadataFromHeaders(filename, resp.Header)
		if _, err := io.Copy(w, resp.Body); err != nil {
			return FileMetadata{}, Result{}, false, &TransportError{StatusCode: resp.StatusCode, Err: err}
		}
		return meta, Result{}, false, nil
	}

	limited := io.LimitReader(resp.Body, maxResponseBodyBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return FileMetadata{}, Result{}, false, &TransportError{StatusCode: resp.StatusCode, Err: err}
	}
	if len(data) > maxResponseBodyBytes {
		return FileMetadata{}, Result{}, false, &TransportError{StatusCode: resp.StatusCode, Err: fmt.Errorf("response body exceeds %d bytes", maxResponseBodyBytes)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if decoded, derr := DecodeResult(data); derr == nil && (!decoded.OK || len(decoded.Errors) > 0) {
			return FileMetadata{}, decoded, true, nil
		}
		return FileMetadata{}, Result{}, false, &TransportError{StatusCode: resp.StatusCode, Body: string(data)}
	}
	res, err = DecodeResult(data)
	if err != nil {
		return FileMetadata{}, Result{}, false, &TransportError{StatusCode: resp.StatusCode, Body: string(data), Err: err}
	}
	return FileMetadata{}, res, true, nil
}

func fileMetadataFromHeaders(filename string, h http.Header) FileMetadata {
	meta := FileMetadata{
		Name:        filename,
		ContentType: h.Get("Content-Type"),
		SHA256:      h.Get("X-DatoriumDB-SHA256"),
		Version:     h.Get("X-DatoriumDB-File-Version"),
		OperationID: h.Get("X-DatoriumDB-Operation-Id"),
		ETag:        h.Get("ETag"),
	}
	if cl := h.Get("Content-Length"); cl != "" {
		if n, err := strconv.ParseInt(cl, 10, 64); err == nil {
			meta.ByteSize = n
		}
	}
	return meta
}

func fileWriteResultFrom(res Result) FileWriteResult {
	return FileWriteResult{
		Result:               res,
		Command:              res.StringField("command"),
		Collection:           res.StringField("collection"),
		ID:                   res.StringField("id"),
		Filename:             res.StringField("filename"),
		Version:              res.StringField("version"),
		ByteSize:             int64Field(res, "byteSize"),
		SHA256:               res.StringField("sha256"),
		ContentType:          res.StringField("contentType"),
		OperationID:          res.StringField("operationId"),
		DistributionComplete: res.BoolField("distributionComplete"),
		Note:                 replicationNoteFrom(res.ValueField("note")),
	}
}

func fileListFrom(res Result) []FileMetadata {
	arr := res.ValueField("files")
	if !arr.IsArray() {
		return nil
	}
	out := make([]FileMetadata, 0, len(arr.Items()))
	for _, item := range arr.Items() {
		if !item.IsObject() {
			continue
		}
		out = append(out, FileMetadata{
			Name:        item.Get("name").ToStringOrEmpty(),
			ContentType: item.Get("contentType").ToStringOrEmpty(),
			ByteSize:    int64FromValue(item.Get("byteSize")),
			SHA256:      item.Get("sha256").ToStringOrEmpty(),
			Version:     item.Get("version").ToStringOrEmpty(),
			OperationID: item.Get("operationId").ToStringOrEmpty(),
		})
	}
	return out
}

func int64Field(res Result, key string) int64 {
	return int64FromValue(res.Env.Get(key))
}

func int64FromValue(v ojson.JSONValue) int64 {
	if v.IsMissing() || v.IsNull() {
		return 0
	}
	raw := ""
	switch {
	case v.IsNumber():
		raw = v.ToJSON()
	case v.IsString():
		raw = v.ToStringOrEmpty()
	default:
		return 0
	}
	n, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

package datorium_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	datorium "github.com/JohnAD/datorium-client-go/v2"
)

func TestFilePutDownloadListDelete(t *testing.T) {
	var putBody []byte
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, establishDoc(r.Host))
	})
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if strings.HasPrefix(ct, "multipart/") {
			mr, err := r.MultipartReader()
			if err != nil {
				t.Fatal(err)
			}
			var cmd commandReq
			for {
				part, err := mr.NextPart()
				if err == io.EOF {
					break
				}
				if err != nil {
					t.Fatal(err)
				}
				switch part.FormName() {
				case "command":
					raw, _ := io.ReadAll(part)
					if err := json.Unmarshal(raw, &cmd); err != nil {
						t.Fatal(err)
					}
				case "content":
					putBody, _ = io.ReadAll(part)
				}
				_ = part.Close()
			}
			ver := "v1"
			if cmd.Command == "fileUpdate" {
				ver = "v2"
			}
			writeEnv(w, map[string]any{
				"ok": true, "command": cmd.Command, "collection": "Movies", "id": "doc1",
				"filename": "a.bin", "version": ver, "byteSize": 3, "sha256": "abc",
				"contentType": "application/octet-stream", "operationId": "op1",
				"distributionComplete": true,
			})
			return
		}

		b, _ := io.ReadAll(r.Body)
		req := parseCommandBody(t, string(b))
		switch req.Command {
		case "fileRead":
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Accept-Ranges", "bytes")
			if r.Header.Get("Range") == "bytes=99-" {
				w.Header().Set("Content-Range", "bytes */3")
				w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
				writeEnv(w, map[string]any{"ok": false, "errors": []any{map[string]any{"code": "invalidRange", "message": "unsatisfiable"}}})
				return
			}
			w.Header().Set("X-DatoriumDB-SHA256", "abc")
			w.Header().Set("X-DatoriumDB-File-Version", "v1")
			w.Header().Set("X-DatoriumDB-Operation-Id", "op1")
			if r.Header.Get("Range") == "bytes=1-" {
				w.Header().Set("Content-Length", "2")
				w.Header().Set("Content-Range", "bytes 1-2/3")
				w.WriteHeader(http.StatusPartialContent)
				_, _ = w.Write([]byte("yz"))
				return
			}
			w.Header().Set("Content-Length", "3")
			_, _ = w.Write([]byte("xyz"))
		case "fileList":
			writeEnv(w, map[string]any{
				"ok": true, "command": "fileList", "collection": "Movies", "id": "doc1",
				"files": []any{map[string]any{
					"name": "a.bin", "contentType": "application/octet-stream",
					"byteSize": 3, "sha256": "abc", "version": "v1", "operationId": "op1",
				}},
			})
		case "fileDelete":
			var detail map[string]any
			_ = json.Unmarshal(req.Detail, &detail)
			if detail["version"] != "v1" {
				writeEnv(w, map[string]any{"ok": false, "errors": []any{map[string]any{"code": "fileVersionMismatch", "message": "bad"}}})
				return
			}
			writeEnv(w, map[string]any{
				"ok": true, "command": "fileDelete", "collection": "Movies", "id": "doc1",
				"filename": "a.bin", "version": "v1", "byteSize": 3, "sha256": "abc",
				"contentType": "application/octet-stream", "operationId": "op3",
				"distributionComplete": true,
			})
		default:
			writeEnv(w, map[string]any{"ok": false, "errors": []any{map[string]any{"code": "unknownCommand", "message": "nope"}}})
		}
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client, err := datorium.New(datorium.Config{
		EstablishmentURL: ts.URL,
		Token:            "tok",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := client.Establish(ctx); err != nil {
		t.Fatal(err)
	}
	wr, err := client.PutFile(ctx, "Movies", "doc1", "a.bin", bytes.NewReader([]byte("xyz")), nil)
	if err != nil {
		t.Fatal(err)
	}
	if !wr.DistributionComplete || string(putBody) != "xyz" || wr.Command != "fileCreate" {
		t.Fatalf("put %#v body=%q", wr, putBody)
	}
	var buf bytes.Buffer
	meta, err := client.DownloadFile(ctx, "Movies", "doc1", "a.bin", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if buf.String() != "xyz" || meta.Version != "v1" {
		t.Fatalf("download meta=%#v body=%q", meta, buf.String())
	}
	buf.Reset()
	callbackBeforeWrite := false
	rangeMeta, err := client.DownloadFileWithOptions(ctx, "Movies", "doc1", "a.bin", &buf, &datorium.DownloadFileOptions{
		Range: "bytes=1-",
		OnResponse: func(got datorium.FileDownloadMetadata) error {
			callbackBeforeWrite = buf.Len() == 0 && got.StatusCode == http.StatusPartialContent
			return nil
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !callbackBeforeWrite || buf.String() != "yz" || rangeMeta.StatusCode != http.StatusPartialContent ||
		rangeMeta.ContentRange != "bytes 1-2/3" || rangeMeta.ByteSize != 2 || rangeMeta.TotalByteSize != 3 {
		t.Fatalf("range meta=%#v body=%q", rangeMeta, buf.String())
	}
	buf.Reset()
	rangeMeta, err = client.DownloadFileRange(ctx, "Movies", "doc1", "a.bin", "bytes=0-99", &buf)
	if err != nil || rangeMeta.StatusCode != http.StatusOK || buf.String() != "xyz" {
		t.Fatalf("ignored range meta=%#v body=%q err=%v", rangeMeta, buf.String(), err)
	}
	_, err = client.DownloadFileRange(ctx, "Movies", "doc1", "a.bin", "bytes=99-", io.Discard)
	if err == nil || !datorium.IsAppCode(err, datorium.CodeInvalidRange) {
		t.Fatalf("expected invalidRange, got %v", err)
	}
	list, err := client.ListFiles(ctx, "Movies", "doc1")
	if err != nil || len(list) != 1 || list[0].Name != "a.bin" {
		t.Fatalf("list %#v err=%v", list, err)
	}
	del, err := client.DeleteFile(ctx, "Movies", "doc1", "a.bin", "v1")
	if err != nil || del.Command != "fileDelete" {
		t.Fatalf("delete %#v err=%v", del, err)
	}
}

func TestPutFileRequiresSeekable(t *testing.T) {
	client, err := datorium.New(datorium.Config{EstablishmentURL: "http://127.0.0.1:1", Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	_, err = client.PutFile(context.Background(), "Movies", "doc1", "a.bin", io.NopCloser(bytes.NewBufferString("x")), nil)
	if err == nil || !strings.Contains(err.Error(), "ReadSeeker") {
		t.Fatalf("expected seekable error, got %v", err)
	}
}

func TestPutFileUpdateUsesMultipart(t *testing.T) {
	var gotCmd commandReq
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, establishDoc(r.Host))
	})
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, r *http.Request) {
		mr, err := r.MultipartReader()
		if err != nil {
			t.Fatal(err)
		}
		for {
			part, err := mr.NextPart()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Fatal(err)
			}
			if part.FormName() == "command" {
				raw, _ := io.ReadAll(part)
				if err := json.Unmarshal(raw, &gotCmd); err != nil {
					t.Fatal(err)
				}
			} else {
				_, _ = io.Copy(io.Discard, part)
			}
			_ = part.Close()
		}
		writeEnv(w, map[string]any{
			"ok": true, "command": "fileUpdate", "collection": "Movies", "id": "doc1",
			"filename": "a.bin", "version": "v2", "byteSize": 1, "sha256": "abc",
			"distributionComplete": true,
		})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client, err := datorium.New(datorium.Config{EstablishmentURL: ts.URL, Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := client.Establish(ctx); err != nil {
		t.Fatal(err)
	}
	_, err = client.PutFile(ctx, "Movies", "doc1", "a.bin", bytes.NewReader([]byte("x")), &datorium.PutFileOptions{
		IfMatch: "v1",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotCmd.Command != "fileUpdate" || gotCmd.Target != "Movies" || gotCmd.Parameter != "doc1" {
		t.Fatalf("cmd %#v", gotCmd)
	}
	if !strings.Contains(string(gotCmd.Detail), `"version":"v1"`) {
		t.Fatalf("detail %q", gotCmd.Detail)
	}
}

func TestDownloadFileRangeFormsAndMetadata(t *testing.T) {
	responses := map[string]struct {
		body         string
		contentRange string
	}{
		"bytes=1-3": {"bcd", "bytes 1-3/6"},
		"bytes=2-":  {"cdef", "bytes 2-5/6"},
		"bytes=-2":  {"ef", "bytes 4-5/6"},
	}
	var seen []string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, establishDoc(r.Host))
	})
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, r *http.Request) {
		byteRange := r.Header.Get("Range")
		seen = append(seen, byteRange)
		want, ok := responses[byteRange]
		if !ok {
			t.Fatalf("unexpected Range header %q", byteRange)
		}
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Length", strconv.Itoa(len(want.body)))
		w.Header().Set("Content-Range", want.contentRange)
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = io.WriteString(w, want.body)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	client, err := datorium.New(datorium.Config{EstablishmentURL: ts.URL, Token: "t"})
	if err != nil {
		t.Fatal(err)
	}

	for byteRange, want := range responses {
		t.Run(byteRange, func(t *testing.T) {
			var buf bytes.Buffer
			meta, err := client.DownloadFileRange(context.Background(), "Movies", "doc1", "a.bin", byteRange, &buf)
			if err != nil {
				t.Fatal(err)
			}
			if buf.String() != want.body || meta.StatusCode != http.StatusPartialContent ||
				meta.ContentRange != want.contentRange || meta.AcceptRanges != "bytes" ||
				meta.ByteSize != int64(len(want.body)) || meta.TotalByteSize != 6 {
				t.Fatalf("meta=%#v body=%q", meta, buf.String())
			}
		})
	}
	if len(seen) != len(responses) {
		t.Fatalf("saw %d requests, want %d", len(seen), len(responses))
	}
}

func TestDownloadFileRangeRejectsInvalidValues(t *testing.T) {
	client, err := datorium.New(datorium.Config{EstablishmentURL: "http://127.0.0.1:1", Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	for _, byteRange := range []string{"", "0-1", "items=0-1", "bytes=", "bytes=-", "bytes=-0", "bytes=+1-2", "bytes=3-2", "bytes=1-2,4-5", "bytes= 1-2"} {
		t.Run(byteRange, func(t *testing.T) {
			_, err := client.DownloadFileRange(context.Background(), "Movies", "doc1", "a.bin", byteRange, io.Discard)
			if err == nil || !strings.Contains(err.Error(), "byte range") {
				t.Fatalf("range %q: expected validation error, got %v", byteRange, err)
			}
		})
	}
}

func TestDownloadFileRange416MetadataAndErrors(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, establishDoc(r.Host))
	})
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Accept-Ranges", "bytes")
		w.Header().Set("Content-Range", "bytes */6")
		w.WriteHeader(http.StatusRequestedRangeNotSatisfiable)
		if r.Header.Get("Range") == "bytes=99-" {
			writeEnv(w, map[string]any{
				"ok": false,
				"errors": []any{map[string]any{
					"code": datorium.CodeInvalidRange, "message": "unsatisfiable",
				}},
			})
			return
		}
		_, _ = io.WriteString(w, "range not satisfiable")
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	client, err := datorium.New(datorium.Config{EstablishmentURL: ts.URL, Token: "t"})
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name       string
		byteRange  string
		wantAppErr bool
	}{
		{"envelope", "bytes=99-", true},
		{"plain", "bytes=98-", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var buf bytes.Buffer
			meta, err := client.DownloadFileRange(context.Background(), "Movies", "doc1", "a.bin", tc.byteRange, &buf)
			if err == nil {
				t.Fatal("expected error")
			}
			if meta.StatusCode != http.StatusRequestedRangeNotSatisfiable ||
				meta.ContentRange != "bytes */6" || meta.AcceptRanges != "bytes" ||
				meta.TotalByteSize != 6 || buf.Len() != 0 {
				t.Fatalf("meta=%#v body=%q err=%v", meta, buf.String(), err)
			}
			if tc.wantAppErr {
				if !datorium.IsAppCode(err, datorium.CodeInvalidRange) {
					t.Fatalf("expected invalidRange AppError, got %T %v", err, err)
				}
			} else {
				var transportErr *datorium.TransportError
				if !errors.As(err, &transportErr) || transportErr.StatusCode != http.StatusRequestedRangeNotSatisfiable {
					t.Fatalf("expected 416 TransportError, got %T %v", err, err)
				}
			}
		})
	}
}

func TestDownloadFileRangeWrongMachineRetryDoesNotWriteEnvelope(t *testing.T) {
	commandHits := 0
	var ranges []string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, establishDoc(r.Host))
	})
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, r *http.Request) {
		commandHits++
		ranges = append(ranges, r.Header.Get("Range"))
		if commandHits == 1 {
			writeEnv(w, map[string]any{
				"ok": false, "configVersion": 1,
				"errors": []any{map[string]any{"code": datorium.CodeWrongMachine, "message": "retry"}},
			})
			return
		}
		w.Header().Set("Content-Length", "2")
		w.Header().Set("Content-Range", "bytes 1-2/3")
		w.Header().Set("Accept-Ranges", "bytes")
		w.WriteHeader(http.StatusPartialContent)
		_, _ = io.WriteString(w, "yz")
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	client, err := datorium.New(datorium.Config{EstablishmentURL: ts.URL, Token: "t"})
	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	meta, err := client.DownloadFileRange(context.Background(), "Movies", "doc1", "a.bin", "bytes=1-", &buf)
	if err != nil {
		t.Fatal(err)
	}
	if commandHits != 2 || len(ranges) != 2 || ranges[0] != "bytes=1-" || ranges[1] != "bytes=1-" {
		t.Fatalf("hits=%d ranges=%v", commandHits, ranges)
	}
	if buf.String() != "yz" || meta.StatusCode != http.StatusPartialContent {
		t.Fatalf("meta=%#v body=%q", meta, buf.String())
	}
}

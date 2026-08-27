package datorium_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	datorium "github.com/JohnAD/datorium-client-go"
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
			w.Header().Set("Content-Length", "3")
			w.Header().Set("X-DatoriumDB-SHA256", "abc")
			w.Header().Set("X-DatoriumDB-File-Version", "v1")
			w.Header().Set("X-DatoriumDB-Operation-Id", "op1")
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

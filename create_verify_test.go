package datorium_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	datorium "github.com/JohnAD/datorium-client-go"
)

func TestCreateMintsULIDWhenEmpty(t *testing.T) {
	var gotBody string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, establishDoc(r.Host))
	})
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		parts := strings.SplitN(gotBody, " ", 4)
		id := parts[2]
		writeEnv(w, map[string]any{
			"ok": true, "command": "create", "collection": "Todos",
			"id": id, "$": "Todos:0", "#": "ver1", "operationId": "op1",
		})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client, err := datorium.New(datorium.Config{EstablishmentURL: ts.URL, Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	wr, err := client.Create(context.Background(), "Todos", "", map[string]any{
		"$": "Todos:0", "title": "x",
	})
	if err != nil {
		t.Fatal(err)
	}
	if wr.ID == "" || wr.ID == "null" {
		t.Fatalf("expected minted id, got %#v", wr)
	}
	if !strings.HasPrefix(gotBody, "create Todos "+wr.ID+" ") {
		t.Fatalf("body %q", gotBody)
	}
	if strings.Contains(gotBody, " null ") {
		t.Fatalf("must not send null parm: %q", gotBody)
	}
}

func TestCreateDocumentExistsTreatedAsIdempotentSuccess(t *testing.T) {
	var creates atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, establishDoc(r.Host))
	})
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body := string(b)
		if strings.HasPrefix(body, "create ") {
			creates.Add(1)
			writeEnv(w, map[string]any{
				"ok": false, "command": "create", "collection": "Todos", "id": "todo1",
				"errors": []any{map[string]any{"code": "documentExists", "message": "exists"}},
			})
			return
		}
		if strings.HasPrefix(body, "read ") {
			writeEnv(w, map[string]any{
				"ok": true, "command": "read", "collection": "Todos", "id": "todo1",
				"sot": map[string]any{"!": "todo1", "$": "Todos:0", "#": "ver9", "title": "Buy milk"},
			})
			return
		}
		writeEnv(w, map[string]any{"ok": false, "errors": []any{map[string]any{"code": "unknownCommand", "message": "nope"}}})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client, err := datorium.New(datorium.Config{EstablishmentURL: ts.URL, Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	wr, err := client.Create(context.Background(), "Todos", "todo1", map[string]any{
		"$": "Todos:0", "title": "Buy milk",
	})
	if err != nil {
		t.Fatal(err)
	}
	if wr.ID != "todo1" || wr.Version != "ver9" {
		t.Fatalf("got %#v", wr)
	}
	if creates.Load() != 1 {
		t.Fatalf("creates=%d", creates.Load())
	}
}

func TestCreateTransportFailureVerifiedByFollowUpRead(t *testing.T) {
	var creates atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, establishDoc(r.Host))
	})
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body := string(b)
		if strings.HasPrefix(body, "create ") {
			creates.Add(1)
			hj, ok := w.(http.Hijacker)
			if !ok {
				t.Fatal("hijack unsupported")
			}
			conn, _, err := hj.Hijack()
			if err != nil {
				t.Fatal(err)
			}
			_ = conn.Close()
			return
		}
		if strings.HasPrefix(body, "read ") {
			writeEnv(w, map[string]any{
				"ok": true, "command": "read", "collection": "Todos", "id": "todo1",
				"sot": map[string]any{"!": "todo1", "$": "Todos:0", "#": "ver3", "title": "ok"},
			})
			return
		}
		writeEnv(w, map[string]any{"ok": false, "errors": []any{map[string]any{"code": "unknownCommand", "message": "nope"}}})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client, err := datorium.New(datorium.Config{
		EstablishmentURL:           ts.URL,
		Token:                      "t",
		CreateAmbiguousVerifyDelay: time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	wr, err := client.Create(context.Background(), "Todos", "todo1", map[string]any{
		"$": "Todos:0", "title": "ok",
	})
	if err != nil {
		t.Fatal(err)
	}
	if wr.Version != "ver3" {
		t.Fatalf("got %#v", wr)
	}
	if creates.Load() != 1 {
		t.Fatalf("creates=%d", creates.Load())
	}
}

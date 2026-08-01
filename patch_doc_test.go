package datorium_test

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	datorium "github.com/JohnAD/datorium-client-go"
	"github.com/JohnAD/ojson"
)

func TestPatchDocSendsOrderedRFC6902(t *testing.T) {
	var gotBody string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, withTodosSchema(establishDoc(r.Host)))
	})
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body := string(b)
		switch {
		case strings.HasPrefix(body, "read "):
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"ok":true,"command":"read","collection":"Todos","id":"todo1","sot":{"!":"todo1","$":"Todos:0","#":"ver1","title":"Buy milk","status":"open"}}`)
		case strings.HasPrefix(body, "patch "):
			gotBody = body
			writeEnv(w, map[string]any{
				"ok": true, "command": "patch", "collection": "Todos", "id": "todo1",
				"$": "Todos:0", "operationId": "op1",
				"versions": map[string]any{"before": "ver1", "after": "ver2"},
			})
		default:
			writeEnv(w, map[string]any{"ok": false, "errors": []any{map[string]any{"code": "unknownCommand", "message": "nope"}}})
		}
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client, err := datorium.New(datorium.Config{EstablishmentURL: ts.URL, Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	Todos := datorium.MustCollection[todoDoc]("Todos", 0)
	todos, err := Todos.Bind(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}

	item, err := todos.GetDoc(context.Background(), "todo1")
	if err != nil {
		t.Fatal(err)
	}
	patch, err := ojson.NewPatch(
		ojson.PatchReplace("/status", ojson.NewString("done")),
	)
	if err != nil {
		t.Fatal(err)
	}
	cp, err := todos.CreatePatch(item, patch)
	if err != nil {
		t.Fatal(err)
	}
	if cp.ID() != "todo1" || cp.Version() != "ver1" {
		t.Fatalf("%#v", cp)
	}

	wr, err := todos.PatchDoc(context.Background(), cp)
	if err != nil {
		t.Fatal(err)
	}
	if wr.Version != "ver2" || wr.VersionBefore != "ver1" {
		t.Fatalf("%#v", wr)
	}
	if !strings.HasPrefix(gotBody, "patch Todos todo1 ") {
		t.Fatalf("%q", gotBody)
	}
	if !strings.Contains(gotBody, `"$":"Todos:0"`) || !strings.Contains(gotBody, `"#":"ver1"`) {
		t.Fatalf("missing markers: %q", gotBody)
	}
	if !strings.Contains(gotBody, `"RFC6902":[`) {
		t.Fatalf("missing RFC6902: %q", gotBody)
	}
	if !strings.Contains(gotBody, `"path":"/status"`) || !strings.Contains(gotBody, `"value":"done"`) {
		t.Fatalf("missing status replace: %q", gotBody)
	}
}

func TestCreatePatchFromChanges(t *testing.T) {
	var gotBody string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, withTodosSchema(establishDoc(r.Host)))
	})
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body := string(b)
		switch {
		case strings.HasPrefix(body, "read "):
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"ok":true,"command":"read","collection":"Todos","id":"todo1","sot":{"!":"todo1","$":"Todos:0","#":"ver1","title":"Buy milk","status":"open"}}`)
		case strings.HasPrefix(body, "patch "):
			gotBody = body
			writeEnv(w, map[string]any{
				"ok": true, "command": "patch", "collection": "Todos", "id": "todo1",
				"$": "Todos:0", "operationId": "op1",
				"versions": map[string]any{"before": "ver1", "after": "ver2"},
			})
		default:
			writeEnv(w, map[string]any{"ok": false, "errors": []any{map[string]any{"code": "unknownCommand", "message": "nope"}}})
		}
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client, err := datorium.New(datorium.Config{EstablishmentURL: ts.URL, Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	Todos := datorium.MustCollection[todoDoc]("Todos", 0)
	todos, err := Todos.Bind(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}

	item, err := todos.GetDoc(context.Background(), "todo1")
	if err != nil {
		t.Fatal(err)
	}
	item.Doc.Status = "done"
	// Mutating OriginalDoc must not affect the private baseline used for diff.
	item.OriginalDoc.Status = "corrupted"

	cp, err := todos.CreatePatchFromChanges(item)
	if err != nil {
		t.Fatal(err)
	}
	if cp.Patch.Len() != 1 {
		t.Fatalf("ops=%d", cp.Patch.Len())
	}
	wr, err := todos.PatchDoc(context.Background(), cp)
	if err != nil {
		t.Fatal(err)
	}
	if wr.Version != "ver2" {
		t.Fatalf("%#v", wr)
	}
	if !strings.Contains(gotBody, `"op":"replace"`) || !strings.Contains(gotBody, `"path":"/status"`) {
		t.Fatalf("%q", gotBody)
	}
}

func TestCreatePatchFromChangesRejectsEmpty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, withTodosSchema(establishDoc(r.Host)))
	})
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"ok":true,"command":"read","collection":"Todos","id":"todo1","sot":{"!":"todo1","$":"Todos:0","#":"ver1","title":"Buy milk","status":"open"}}`)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client, err := datorium.New(datorium.Config{EstablishmentURL: ts.URL, Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	Todos := datorium.MustCollection[todoDoc]("Todos", 0)
	todos, err := Todos.Bind(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	item, err := todos.GetDoc(context.Background(), "todo1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = todos.CreatePatchFromChanges(item)
	if err == nil || !strings.Contains(err.Error(), "no operations") {
		t.Fatalf("got %v", err)
	}
}

func TestCreatePatchRejectsEmpty(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, withTodosSchema(establishDoc(r.Host)))
	})
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"ok":true,"command":"read","collection":"Todos","id":"todo1","sot":{"!":"todo1","$":"Todos:0","#":"ver1","title":"Buy milk","status":"open"}}`)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client, err := datorium.New(datorium.Config{EstablishmentURL: ts.URL, Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	Todos := datorium.MustCollection[todoDoc]("Todos", 0)
	todos, err := Todos.Bind(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	item, err := todos.GetDoc(context.Background(), "todo1")
	if err != nil {
		t.Fatal(err)
	}
	_, err = todos.CreatePatch(item, ojson.Patch{})
	if err == nil || !strings.Contains(err.Error(), "no operations") {
		t.Fatalf("got %v", err)
	}
}

func TestCrossBindingItemRejected(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, withTodosSchema(establishDoc(r.Host)))
	})
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = fmt.Fprintf(w, `{"ok":true,"command":"read","collection":"Todos","id":"todo1","sot":{"!":"todo1","$":"Todos:0","#":"ver1","title":"Buy milk","status":"open"}}`)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client, err := datorium.New(datorium.Config{EstablishmentURL: ts.URL, Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	Todos := datorium.MustCollection[todoDoc]("Todos", 0)
	a, err := Todos.Bind(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Todos.Bind(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	item, err := a.GetDoc(context.Background(), "todo1")
	if err != nil {
		t.Fatal(err)
	}
	item.Doc.Status = "done"
	_, err = b.CreatePatchFromChanges(item)
	if err == nil || !strings.Contains(err.Error(), "different collection binding") {
		t.Fatalf("got %v", err)
	}
}

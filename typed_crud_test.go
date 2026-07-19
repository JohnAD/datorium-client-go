package datorium_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	datorium "github.com/JohnAD/datorium-client-go"
	"github.com/JohnAD/ojson"
)

type todoDoc struct {
	Title  string `json:"title"`
	Status string `json:"status"`
}

func TestEstablishCatalogMismatch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		doc := establishDoc(r.Host)
		doc["schemas"] = map[string]any{
			"Todos": map[string]any{"version": 1, "schema": map[string]any{}},
		}
		writeEnv(w, doc)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client, err := datorium.New(datorium.Config{EstablishmentURL: ts.URL, Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	Todos := datorium.MustCollection[todoDoc]("Todos", 0)
	err = client.Establish(context.Background(), Todos)
	var ce *datorium.CatalogError
	if !errors.As(err, &ce) {
		t.Fatalf("got %v, want CatalogError", err)
	}
	if len(ce.Mismatches) != 1 || ce.Mismatches[0].Code != datorium.CatalogSchemaVersionMismatch {
		t.Fatalf("mismatches %#v", ce.Mismatches)
	}
}

func TestEstablishCatalogCollectionNotFound(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, establishDoc(r.Host))
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client, err := datorium.New(datorium.Config{EstablishmentURL: ts.URL, Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	Todos := datorium.MustCollection[todoDoc]("Todos", 0)
	err = client.Establish(context.Background(), Todos)
	var ce *datorium.CatalogError
	if !errors.As(err, &ce) {
		t.Fatalf("got %v, want CatalogError", err)
	}
	if ce.Mismatches[0].Code != datorium.CatalogCollectionNotFound {
		t.Fatalf("code %q", ce.Mismatches[0].Code)
	}
}

func TestCreateDocOrderedVoidAndReadDelete(t *testing.T) {
	var createBody, deleteBody string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		doc := establishDoc(r.Host)
		doc["schemas"] = map[string]any{
			"Todos": map[string]any{"version": 0, "schema": map[string]any{}},
		}
		writeEnv(w, doc)
	})
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body := string(b)
		switch {
		case strings.HasPrefix(body, "create "):
			createBody = body
			writeEnv(w, map[string]any{
				"ok": true, "command": "create", "collection": "Todos",
				"id": "todo1", "$": "Todos:0", "#": "ver1", "operationId": "op1",
			})
		case strings.HasPrefix(body, "read "):
			// Deliberate key order in JSON text: title before status; system fields first.
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"ok":true,"command":"read","collection":"Todos","id":"todo1","sot":{"!":"todo1","$":"Todos:0","#":"ver1","title":"Buy milk","status":"open"}}`))
		case strings.HasPrefix(body, "delete "):
			deleteBody = body
			writeEnv(w, map[string]any{
				"ok": true, "command": "delete", "collection": "Todos",
				"id": "todo1", "$": "Todos:0", "#": "ver1", "operationId": "op2",
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
	ctx := context.Background()
	Todos := datorium.MustCollection[todoDoc]("Todos", 0)
	if err := client.Establish(ctx, Todos); err != nil {
		t.Fatal(err)
	}

	wr, err := datorium.CreateDoc(ctx, client, Todos, ojson.NewVoid(), todoDoc{Title: "Buy milk", Status: "open"})
	if err != nil {
		t.Fatal(err)
	}
	if wr.ID != "todo1" || wr.Version != "ver1" {
		t.Fatalf("create %#v", wr)
	}
	if !strings.HasPrefix(createBody, "create Todos null ") {
		t.Fatalf("expected null parm, got %q", createBody)
	}
	titleIdx := strings.Index(createBody, `"title"`)
	statusIdx := strings.Index(createBody, `"status"`)
	if titleIdx < 0 || statusIdx < 0 || titleIdx > statusIdx {
		t.Fatalf("field order not preserved in %q", createBody)
	}
	if !strings.Contains(createBody, `"$":"Todos:0"`) {
		t.Fatalf("missing schema marker in %q", createBody)
	}
	if !strings.Contains(createBody, `"operationId"`) {
		t.Fatalf("missing operationId in %q", createBody)
	}

	rr, err := datorium.ReadDoc(ctx, client, Todos, wr.ID, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rr.Doc.Title != "Buy milk" || rr.Doc.Status != "open" {
		t.Fatalf("doc %#v", rr.Doc)
	}
	if rr.Meta.ID != "todo1" || rr.Meta.Schema != "Todos:0" || rr.Meta.Version != "ver1" {
		t.Fatalf("meta %#v", rr.Meta)
	}

	if _, err := datorium.DeleteDoc(ctx, client, Todos, wr.ID, rr.Meta.Version); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(deleteBody, `"$":"Todos:0"`) || !strings.Contains(deleteBody, `"#":"ver1"`) {
		t.Fatalf("delete detail %q", deleteBody)
	}
}

func TestCreateDocRejectsEmptyStringID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		doc := establishDoc(r.Host)
		doc["schemas"] = map[string]any{
			"Todos": map[string]any{"version": 0, "schema": map[string]any{}},
		}
		writeEnv(w, doc)
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client, err := datorium.New(datorium.Config{EstablishmentURL: ts.URL, Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	Todos := datorium.MustCollection[todoDoc]("Todos", 0)
	if err := client.Establish(ctx, Todos); err != nil {
		t.Fatal(err)
	}
	_, err = datorium.CreateDoc(ctx, client, Todos, ojson.NewString(""), todoDoc{Title: "x", Status: "open"})
	if err == nil || !strings.Contains(err.Error(), "empty string id") {
		t.Fatalf("got %v", err)
	}
}

func TestBuildCommandOrderedPreservesFieldOrder(t *testing.T) {
	doc := ojson.NewObject()
	doc.Set("title", ojson.NewString("a"))
	doc.Set("status", ojson.NewString("b"))
	line, err := datorium.BuildCommandOrdered("create", "Todos", "null", doc)
	if err != nil {
		t.Fatal(err)
	}
	want := `create Todos null {"title":"a","status":"b"}`
	if line != want {
		t.Fatalf("got %q want %q", line, want)
	}
}

func TestMustCollectionPanics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = datorium.MustCollection[todoDoc]("", 0)
}

package datorium_test

import (
	"context"
	"errors"
	"fmt"
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

func todoSchemaDoc() map[string]any {
	return map[string]any{
		"kind": "object",
		"children": []any{
			map[string]any{"name": "title", "kind": "string"},
			map[string]any{"name": "status", "kind": "string"},
		},
	}
}

func withTodosSchema(doc map[string]any) map[string]any {
	doc["schemas"] = map[string]any{
		"Todos": map[string]any{"version": 0, "schema": todoSchemaDoc()},
	}
	return doc
}

func TestEstablishCatalogMismatch(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		doc := establishDoc(r.Host)
		doc["schemas"] = map[string]any{
			"Todos": map[string]any{"version": 1, "schema": todoSchemaDoc()},
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
	var createdID string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, withTodosSchema(establishDoc(r.Host)))
	})
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		body := string(b)
		req := parseCommandBody(t, body)
		switch req.Command {
		case "create":
			createBody = body
			createdID = req.Parameter
			writeEnv(w, map[string]any{
				"ok": true, "command": "create", "collection": "Todos",
				"id": createdID, "$": "Todos:0", "#": "ver1", "operationId": "op1",
			})
		case "read":
			w.Header().Set("Content-Type", "application/json")
			_, _ = fmt.Fprintf(w, `{"ok":true,"command":"read","collection":"Todos","id":%q,"sot":{"!":%q,"$":"Todos:0","#":"ver1","title":"Buy milk","status":"open"}}`, createdID, createdID)
		case "delete":
			deleteBody = body
			writeEnv(w, map[string]any{
				"ok": true, "command": "delete", "collection": "Todos",
				"id": createdID, "$": "Todos:0", "#": "ver1", "operationId": "op2",
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
	todos, err := Todos.Bind(ctx, client)
	if err != nil {
		t.Fatal(err)
	}

	wr, err := todos.CreateDoc(ctx, nil, todoDoc{Title: "Buy milk", Status: "open"})
	if err != nil {
		t.Fatal(err)
	}
	if wr.ID == "" || wr.ID == "null" || wr.Version != "ver1" {
		t.Fatalf("create %#v", wr)
	}
	if req := parseCommandBody(t, createBody); req.Command != "create" || req.Target != "Todos" || req.Parameter != wr.ID {
		t.Fatalf("expected client-minted id in command, got %q", createBody)
	}
	if strings.Contains(createBody, `"parameter":"null"`) || strings.Contains(createBody, " null ") {
		t.Fatalf("server null parm must not be used: %q", createBody)
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

	item, err := todos.GetDoc(ctx, wr.ID)
	if err != nil {
		t.Fatal(err)
	}
	if item.Doc.Title != "Buy milk" || item.Doc.Status != "open" {
		t.Fatalf("doc %#v", item.Doc)
	}
	if item.OriginalDoc.Title != "Buy milk" || item.OriginalDoc.Status != "open" {
		t.Fatalf("original %#v", item.OriginalDoc)
	}
	if item.Meta.ID != wr.ID || item.Meta.Schema != "Todos:0" || item.Meta.Version != "ver1" {
		t.Fatalf("meta %#v", item.Meta)
	}

	if _, err := todos.DeleteDoc(ctx, item.Meta.ID, item.Meta.Version); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(deleteBody, `"$":"Todos:0"`) || !strings.Contains(deleteBody, `"#":"ver1"`) {
		t.Fatalf("delete detail %q", deleteBody)
	}
}

func TestCreateDocRejectsEmptyStringID(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, withTodosSchema(establishDoc(r.Host)))
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client, err := datorium.New(datorium.Config{EstablishmentURL: ts.URL, Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	Todos := datorium.MustCollection[todoDoc]("Todos", 0)
	todos, err := Todos.Bind(ctx, client)
	if err != nil {
		t.Fatal(err)
	}
	empty := ""
	_, err = todos.CreateDoc(ctx, &empty, todoDoc{Title: "x", Status: "open"})
	if err == nil || !strings.Contains(err.Error(), "empty string id") {
		t.Fatalf("got %v", err)
	}
}

func TestCreateDocExplicitID(t *testing.T) {
	var gotBody string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, withTodosSchema(establishDoc(r.Host)))
	})
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		writeEnv(w, map[string]any{
			"ok": true, "command": "create", "collection": "Todos",
			"id": "todo-42", "$": "Todos:0", "#": "ver1", "operationId": "op1",
		})
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
	id := "todo-42"
	wr, err := todos.CreateDoc(context.Background(), &id, todoDoc{Title: "x", Status: "open"})
	if err != nil {
		t.Fatal(err)
	}
	if wr.ID != "todo-42" {
		t.Fatalf("%#v", wr)
	}
	if !strings.HasPrefix(gotBody, `{"command":"create","target":"Todos","parameter":"todo-42"`) {
		t.Fatalf("%q", gotBody)
	}
}

func TestBindEstablishesLazily(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, withTodosSchema(establishDoc(r.Host)))
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client, err := datorium.New(datorium.Config{EstablishmentURL: ts.URL, Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	Todos := datorium.MustCollection[todoDoc]("Todos", 0)
	if client.CachedEstablishment() != nil {
		t.Fatal("expected empty cache before Bind")
	}
	todos, err := Todos.Bind(context.Background(), client)
	if err != nil {
		t.Fatal(err)
	}
	if client.CachedEstablishment() == nil {
		t.Fatal("expected Establish during Bind")
	}
	if todos.Collection().Name != "Todos" {
		t.Fatalf("%#v", todos.Collection())
	}
}

func TestBindCompilesDatoriumRefFormats(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		doc := establishDoc(r.Host)
		doc["schemas"] = map[string]any{
			"Todos": map[string]any{
				"version": 0,
				"schema": map[string]any{
					"kind": "object",
					"children": []any{
						map[string]any{"name": "title", "kind": "string"},
						map[string]any{"name": "status", "kind": "string"},
						map[string]any{"name": "list", "kind": "string", "format": "DatoriumDirectRef"},
						map[string]any{"name": "listSummary", "kind": "string", "format": "DatoriumCachedRef"},
					},
				},
			},
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
	if _, err := Todos.Bind(context.Background(), client); err != nil {
		t.Fatalf("Bind with Datorium ref formats: %v", err)
	}
}

func TestBuildCommandOrderedPreservesFieldOrder(t *testing.T) {
	doc := ojson.NewObject()
	doc.Set("title", ojson.NewString("a"))
	doc.Set("status", ojson.NewString("b"))
	body, err := datorium.BuildCommandOrdered("create", "Todos", "01TESTID000000000000000000", doc)
	if err != nil {
		t.Fatal(err)
	}
	want := `{"command":"create","target":"Todos","parameter":"01TESTID000000000000000000","detail":{"title":"a","status":"b"}}`
	if string(body) != want {
		t.Fatalf("got %q want %q", body, want)
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

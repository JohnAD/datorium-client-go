package datorium_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	datorium "github.com/JohnAD/datorium-client-go/v2"
	"github.com/JohnAD/datorium-client-go/v2/shard"
)

func TestHealthAndEstablishAndCRUD(t *testing.T) {
	var gotAuth string
	var gotCT string
	var gotBody string
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/health", func(w http.ResponseWriter, _ *http.Request) {
		writeEnv(w, map[string]any{"ok": true, "alive": true})
	})
	mux.HandleFunc("GET /datoriumdb/v1/ready", func(w http.ResponseWriter, _ *http.Request) {
		writeEnv(w, map[string]any{"ok": true, "ready": true, "generalVersion": 1})
	})
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		writeEnv(w, establishDoc(r.Host))
	})
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, r *http.Request) {
		gotCT = r.Header.Get("Content-Type")
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		req := parseCommandBody(t, gotBody)
		switch req.Command {
		case "create":
			writeEnv(w, map[string]any{
				"ok": true, "command": "create", "collection": "Todos",
				"id": "todo1", "$": "Todos:0", "#": "ver1", "operationId": "op1",
			})
		case "read":
			writeEnv(w, map[string]any{
				"ok": true, "command": "read", "collection": "Todos", "id": "todo1",
				"sot": map[string]any{"!": "todo1", "$": "Todos:0", "#": "ver1", "title": "Buy milk"},
			})
		default:
			writeEnv(w, map[string]any{"ok": false, "errors": []any{map[string]any{"code": "unknownCommand", "message": "nope"}}})
		}
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client, err := datorium.New(datorium.Config{
		EstablishmentURL: ts.URL,
		Token:            "test-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	h, err := client.Health(ctx)
	if err != nil || !h.OK {
		t.Fatalf("health: %#v err=%v", h, err)
	}
	if err := client.Establish(ctx); err != nil {
		t.Fatal(err)
	}
	if gotAuth != "Bearer test-token" {
		t.Fatalf("auth header %q", gotAuth)
	}
	wr, err := client.Create(ctx, "Todos", "todo1", map[string]any{"$": "Todos:0", "title": "Buy milk"})
	if err != nil {
		t.Fatal(err)
	}
	if wr.ID != "todo1" || wr.Version != "ver1" {
		t.Fatalf("create result %#v", wr)
	}
	if gotCT != "application/json" {
		t.Fatalf("content-type %q", gotCT)
	}
	if !strings.Contains(gotBody, `"title":"Buy milk"`) {
		t.Fatalf("body %q", gotBody)
	}
	rr, err := client.Read(ctx, "Todos", "todo1", nil)
	if err != nil {
		t.Fatal(err)
	}
	if rr.SOT.Get("title").ToStringOrEmpty() != "Buy milk" {
		t.Fatalf("read %#v", rr)
	}
}

func TestWrongMachineRetry(t *testing.T) {
	var hits atomic.Int32
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, establishDoc(r.Host))
	})
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, r *http.Request) {
		n := hits.Add(1)
		if n == 1 {
			writeEnv(w, map[string]any{
				"ok": false, "command": "read", "collection": "Todos", "id": "todo1",
				"configVersion": 1,
				"errors":        []any{map[string]any{"code": "wrongMachine", "message": "bounce"}},
			})
			return
		}
		writeEnv(w, map[string]any{
			"ok": true, "command": "read", "collection": "Todos", "id": "todo1",
			"sot": map[string]any{"!": "todo1", "title": "ok"},
		})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client, err := datorium.New(datorium.Config{
		EstablishmentURL: ts.URL,
		Token:            "t",
		BaseURLRewrite: map[string]string{
			"server1": ts.URL,
			"server2": ts.URL,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if err := client.Establish(ctx); err != nil {
		t.Fatal(err)
	}
	// Force an ID that routes to server1 initially; bounce sends to server2.
	id := findIDForSlotRange(t, 0x00, 0x7F)
	rr, err := client.Read(ctx, "Todos", id, nil)
	if err != nil {
		t.Fatal(err)
	}
	if rr.SOT.Get("title").ToStringOrEmpty() != "ok" {
		t.Fatalf("unexpected %#v", rr)
	}
	if hits.Load() < 2 {
		t.Fatalf("expected retry, hits=%d", hits.Load())
	}
}

func TestAppErrorHTTP200(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, establishDoc(r.Host))
	})
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, _ *http.Request) {
		writeEnv(w, map[string]any{
			"ok": false,
			"errors": []any{map[string]any{
				"code": "documentNotFound", "message": "missing",
			}},
		})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	client, err := datorium.New(datorium.Config{EstablishmentURL: ts.URL, Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	_, err = client.Read(ctx, "Todos", "missing", nil)
	if !datorium.IsAppCode(err, datorium.CodeDocumentNotFound) {
		t.Fatalf("want documentNotFound, got %v", err)
	}
}

func TestPatchUsesVersionsAfter(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, establishDoc(r.Host))
	})
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, _ *http.Request) {
		writeEnv(w, map[string]any{
			"ok": true, "command": "patch", "collection": "Todos", "id": "todo1",
			"$": "Todos:0", "operationId": "op1",
			"versions": map[string]any{"before": "v1", "after": "v2"},
		})
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)
	client, err := datorium.New(datorium.Config{EstablishmentURL: ts.URL, Token: "t"})
	if err != nil {
		t.Fatal(err)
	}
	wr, err := client.Patch(context.Background(), "Todos", "todo1", map[string]any{
		"$": "Todos:0", "#": "v1",
		"RFC6902": []any{map[string]any{"op": "replace", "path": "/status", "value": "done"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if wr.Version != "v2" || wr.VersionBefore != "v1" {
		t.Fatalf("got Version=%q VersionBefore=%q", wr.Version, wr.VersionBefore)
	}
}

func TestBuildCommand(t *testing.T) {
	body, err := datorium.BuildCommand("create", "Todos", "todo1", map[string]any{"title": "x"})
	if err != nil {
		t.Fatal(err)
	}
	want := `{"command":"create","target":"Todos","parameter":"todo1","detail":{"title":"x"}}`
	if string(body) != want {
		t.Fatalf("got %q want %q", body, want)
	}
}

func TestEnsureCollectionPostsAdminCommand(t *testing.T) {
	var gotBody string
	mux := http.NewServeMux()
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		writeEnv(w, map[string]any{
			"ok": true, "command": "collectionEnsure", "collection": "Todos",
			"schemaVersion": 0, "generalVersion": 2,
		})
	})
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		writeEnv(w, withTodosSchema(establishDoc(r.Host)))
	})
	ts := httptest.NewServer(mux)
	t.Cleanup(ts.Close)

	client, err := datorium.New(datorium.Config{
		EstablishmentURL: ts.URL,
		Token:            "admin-token",
	})
	if err != nil {
		t.Fatal(err)
	}
	res, err := client.EnsureCollection(context.Background(), "Todos", map[string]any{
		"kind": "object", "children": []any{},
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !res.OK {
		t.Fatalf("result %#v", res)
	}
	if client.CachedEstablishment() == nil {
		t.Fatal("expected establishment cache refresh after EnsureCollection")
	}
	req := parseCommandBody(t, gotBody)
	if req.Command != "collectionEnsure" || req.Target != "Todos" || req.Parameter != "" {
		t.Fatalf("unexpected request %#v body=%q", req, gotBody)
	}
}

type commandReq struct {
	Command   string          `json:"command"`
	Target    string          `json:"target"`
	Parameter string          `json:"parameter"`
	Detail    json.RawMessage `json:"detail"`
}

func parseCommandBody(t *testing.T, body string) commandReq {
	t.Helper()
	var req commandReq
	if err := json.Unmarshal([]byte(body), &req); err != nil {
		t.Fatalf("parse command JSON: %v body=%q", err, body)
	}
	return req
}

func writeEnv(w http.ResponseWriter, fields map[string]any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(fields)
}

func establishDoc(host string) map[string]any {
	base := "http://" + host
	return map[string]any{
		"ok": true,
		"general": map[string]any{
			"name": "test", "establishmentServer": "server1", "version": 1,
		},
		"servers": map[string]any{
			"server1": map[string]any{"baseURL": base},
			"server2": map[string]any{"baseURL": base},
		},
		"shardMap": map[string]any{
			"default": map[string]any{
				"00-7F": map[string]any{
					"SHARD_SOT_MEMBER": "server1", "SHARD_READ_MEMBER": []any{"server1"}, "PROXY_READ_MEMBER": []any{},
				},
				"80-FF": map[string]any{
					"SHARD_SOT_MEMBER": "server2", "SHARD_READ_MEMBER": []any{"server2"}, "PROXY_READ_MEMBER": []any{},
				},
			},
		},
		"schemas":  map[string]any{},
		"searches": map[string]any{},
		"auth":     map[string]any{},
	}
}

func findIDForSlotRange(t *testing.T, start, end byte) string {
	t.Helper()
	for i := 0; i < 100000; i++ {
		id := fmt.Sprintf("id%08d", i)
		s := shard.Slot(id)
		if s >= start && s <= end {
			return id
		}
	}
	t.Fatal("no id found")
	return ""
}

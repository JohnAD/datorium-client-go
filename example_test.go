package datorium_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"

	datorium "github.com/JohnAD/datorium-client-go"
)

func ExampleClient_Create() {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /datoriumdb/v1/establish", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, `{
			"ok":true,
			"general":{"name":"demo","establishmentServer":"server1","version":1},
			"servers":{"server1":{"baseURL":%q}},
			"shardMap":{"default":{"00-FF":{"SHARD_SOT_MEMBER":"server1","SHARD_READ_MEMBER":["server1"],"PROXY_READ_MEMBER":[]}}},
			"schemas":{},"searches":{},"auth":{}
		}`, "http://"+r.Host)
	})
	mux.HandleFunc("POST /datoriumdb/v1/command", func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{"ok":true,"command":"create","collection":"Todos","id":"t1","$":"Todos:0","#":"v1","operationId":"op1"}`)
	})
	ts := httptest.NewServer(mux)
	defer ts.Close()

	client, err := datorium.New(datorium.Config{
		EstablishmentURL: ts.URL,
		Token:            "demo-token",
	})
	if err != nil {
		panic(err)
	}
	wr, err := client.Create(context.Background(), "Todos", "t1", map[string]any{
		"$": "Todos:0", "title": "demo", "status": "open",
	})
	if err != nil {
		panic(err)
	}
	fmt.Println(wr.ID, wr.Version)
	// Output: t1 v1
}

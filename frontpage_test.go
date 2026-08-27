package datorium_test

import (
	"testing"

	datorium "github.com/JohnAD/datorium-client-go/v2"
	"github.com/JohnAD/datorium-client-go/v2/refs"
	"github.com/JohnAD/ojson"
)

func TestAppendCachedRefOp(t *testing.T) {
	op := datorium.AppendCachedRefOp("todoLists", "TodoLists", "list1")
	if op["op"] != "add" || op["path"] != "/todoLists/-" {
		t.Fatalf("%#v", op)
	}
	if op["value"] != refs.FormatCached("TodoLists", "list1") {
		t.Fatalf("value=%v", op["value"])
	}
}

func TestSummariesForArrayFieldOrder(t *testing.T) {
	sot, err := ojson.ReadStringNoSchema(`{
		"todoLists": [
			"` + refs.FormatCached("TodoLists", "b") + `",
			"` + refs.FormatCached("TodoLists", "a") + `"
		]
	}`)
	if err != nil {
		t.Fatal(err)
	}
	cache, err := ojson.ReadStringNoSchema(`{
		"TodoLists": {
			"a": {"!": "a", "#": "v1", "title": "A"},
			"b": {"!": "b", "#": "v1", "title": "B"}
		}
	}`)
	if err != nil {
		t.Fatal(err)
	}
	rr := datorium.ReadResult{
		SOT:            sot,
		CacheSummaries: cache,
	}
	sums, err := rr.SummariesForArrayField("todoLists")
	if err != nil {
		t.Fatal(err)
	}
	if len(sums) != 2 ||
		sums[0].Get("title").ToStringOrEmpty() != "B" ||
		sums[1].Get("title").ToStringOrEmpty() != "A" {
		t.Fatalf("%#v", sums)
	}
}

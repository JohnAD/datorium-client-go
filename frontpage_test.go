package datorium_test

import (
	"testing"

	datorium "github.com/JohnAD/datorium-client-go"
	"github.com/JohnAD/datorium-client-go/refs"
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
	rr := datorium.ReadResult{
		SOT: map[string]any{
			"todoLists": []any{
				refs.FormatCached("TodoLists", "b"),
				refs.FormatCached("TodoLists", "a"),
			},
		},
		CacheSummaries: map[string]any{
			"TodoLists": map[string]any{
				"a": map[string]any{"!": "a", "#": "v1", "title": "A"},
				"b": map[string]any{"!": "b", "#": "v1", "title": "B"},
			},
		},
	}
	sums, err := rr.SummariesForArrayField("todoLists")
	if err != nil {
		t.Fatal(err)
	}
	if len(sums) != 2 || sums[0]["title"] != "B" || sums[1]["title"] != "A" {
		t.Fatalf("%#v", sums)
	}
}

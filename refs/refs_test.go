package refs_test

import (
	"testing"

	"github.com/JohnAD/datorium-client-go/refs"
)

func TestParseDirectAndCached(t *testing.T) {
	r, ok, err := refs.Parse("@__Users__abc123")
	if err != nil || !ok {
		t.Fatalf("direct: ok=%v err=%v", ok, err)
	}
	if r.Kind != refs.Direct || r.Collection != "Users" || r.ID != "abc123" {
		t.Fatalf("unexpected %#v", r)
	}
	r, ok, err = refs.Parse("@@__TodoLists__xyz")
	if err != nil || !ok {
		t.Fatalf("cached: ok=%v err=%v", ok, err)
	}
	if r.Kind != refs.Cached || r.Collection != "TodoLists" || r.ID != "xyz" {
		t.Fatalf("unexpected %#v", r)
	}
	if _, ok, err := refs.Parse("not-a-ref"); err != nil || ok {
		t.Fatalf("expected non-ref, ok=%v err=%v", ok, err)
	}
	if refs.FormatDirect("Users", "a") != "@__Users__a" {
		t.Fatal("FormatDirect")
	}
	if refs.FormatCached("Users", "a") != "@@__Users__a" {
		t.Fatal("FormatCached")
	}
}

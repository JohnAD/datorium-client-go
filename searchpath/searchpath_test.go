package searchpath_test

import (
	"testing"

	"github.com/JohnAD/datorium-client-go/searchpath"
	"github.com/JohnAD/datorium-client-go/shard"
)

func TestEncodeStringValue(t *testing.T) {
	if got := searchpath.EncodeStringValue(""); got != "empty" {
		t.Fatalf("empty: %q", got)
	}
	if got := searchpath.EncodeStringValue("open"); got != "6F70656E" {
		t.Fatalf("open: %q", got)
	}
}

func TestShardSlotMatchesRaw(t *testing.T) {
	segs := searchpath.EqualsStringSegments("open")
	got := searchpath.ShardSlot(segs)
	want := shard.RawSlot("6F70656E")
	if got != want {
		t.Fatalf("got %02X want %02X", got, want)
	}
}

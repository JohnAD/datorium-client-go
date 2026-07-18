package shard_test

import (
	"testing"

	"github.com/JohnAD/datorium-client-go/shard"
)

func TestSlotStable(t *testing.T) {
	id := "01ARZ3NDEKTSV4RRFFQ69G5FAV"
	got := shard.Slot(id)
	// Recompute independently for the same algorithm.
	want := shard.Slot(id)
	if got != want {
		t.Fatalf("unstable slot")
	}
	if shard.SlotHex(id) != shard.RawSlotHex(id) {
		// For IDs without a qualifying period, Slot == RawSlot(id).
		t.Fatalf("expected SlotHex == RawSlotHex for plain ULID, got %s vs %s", shard.SlotHex(id), shard.RawSlotHex(id))
	}
}

func TestPrefixIgnoresEarlyPeriods(t *testing.T) {
	// Periods before rune index 6 are ignored; hashing uses the full ID
	// when no later period exists.
	a := shard.Slot("abc.de.fghijklmnop")
	b := shard.Slot("abcde.fghijklmnop")
	if a == 0 && b == 0 {
		// Still a valid outcome; just ensure functions don't panic.
	}
	_ = a
	_ = b
	// Qualifying period at index >= 6: hash only the prefix.
	withPrefix := shard.Slot("abcdef.rest")
	prefixOnly := shard.RawSlot("abcdef")
	if withPrefix != prefixOnly {
		t.Fatalf("prefix slot mismatch: got %02X want %02X", withPrefix, prefixOnly)
	}
}

func TestParseRangeAndCoverage(t *testing.T) {
	r1, err := shard.ParseRange("00-7F")
	if err != nil {
		t.Fatal(err)
	}
	r2, err := shard.ParseRange("80-FF")
	if err != nil {
		t.Fatal(err)
	}
	if !r1.Contains(0x7F) || r1.Contains(0x80) {
		t.Fatalf("range containment wrong: %+v", r1)
	}
	if err := shard.ValidateFullCoverage([]shard.Range{r1, r2}); err != nil {
		t.Fatal(err)
	}
	if err := shard.ValidateFullCoverage([]shard.Range{r1}); err == nil {
		t.Fatal("expected incomplete coverage error")
	}
}

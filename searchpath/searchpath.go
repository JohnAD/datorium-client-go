// Package searchpath encodes precompiled search result path segments and
// computes their shard slots, matching DatoriumDB SEARCHING.md.
package searchpath

import (
	"fmt"
	"strings"

	"github.com/JohnAD/datorium-client-go/v2/shard"
)

// EncodeStringValue encodes a string value as an uppercase-hex path component.
// An empty string encodes as the literal "empty".
func EncodeStringValue(s string) string {
	if s == "" {
		return "empty"
	}
	return strings.ToUpper(fmt.Sprintf("%x", []byte(s)))
}

// EncodeTruth encodes a boolean as "true" or "false".
func EncodeTruth(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

// EncodeNull is the literal path component for a null comparison value.
const EncodeNull = "null"

// ShardInput joins encoded path segments with "/" (no leading/trailing slash).
func ShardInput(segments []string) string {
	return strings.Join(segments, "/")
}

// ShardSlot computes the 8-bit shard slot for an encoded search bucket path.
func ShardSlot(segments []string) byte {
	return shard.RawSlot(ShardInput(segments))
}

// EqualsStringSegments is a helper for the common single equals-string clause
// search (e.g. byStatus with {"status":"open"}).
func EqualsStringSegments(values ...string) []string {
	out := make([]string, len(values))
	for i, v := range values {
		out[i] = EncodeStringValue(v)
	}
	return out
}

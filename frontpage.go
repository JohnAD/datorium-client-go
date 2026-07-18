package datorium

import (
	"fmt"

	"github.com/JohnAD/datorium-client-go/refs"
)

// AppendCachedRefOp returns an RFC6902 "add" operation that appends a
// @@__collection__id value to a document array field (e.g. Users.todoLists).
// Path uses JSON Pointer array-append ("/-").
func AppendCachedRefOp(arrayField, collection, id string) map[string]any {
	return map[string]any{
		"op":    "add",
		"path":  "/" + arrayField + "/-",
		"value": refs.FormatCached(collection, id),
	}
}

// PatchDetailAppendingCachedRef builds a patch detail object for appending one
// cached ref to an array field. schemaMarker and version come from a prior read.
func PatchDetailAppendingCachedRef(schemaMarker, version, arrayField, refCollection, refID string) map[string]any {
	return map[string]any{
		"$": schemaMarker,
		"#": version,
		"RFC6902": []any{
			AppendCachedRefOp(arrayField, refCollection, refID),
		},
	}
}

// SummariesForArrayField returns cache summary objects for @@ refs stored in
// sot[arrayField], in array order. Missing or unresolved summaries are skipped.
// This is the O(1) "front page" read pattern: one document read with
// cacheSummaries:true yields ordered summaries for an array of cached refs.
func (rr ReadResult) SummariesForArrayField(arrayField string) ([]map[string]any, error) {
	raw, ok := rr.SOT[arrayField]
	if !ok {
		return nil, nil
	}
	arr, ok := raw.([]any)
	if !ok {
		return nil, fmt.Errorf("datorium: sot field %q is not an array", arrayField)
	}
	var out []map[string]any
	for _, elem := range arr {
		s, ok := elem.(string)
		if !ok {
			continue
		}
		r, isRef, err := refs.Parse(s)
		if err != nil {
			return nil, err
		}
		if !isRef || r.Kind != refs.Cached {
			continue
		}
		coll, _ := rr.CacheSummaries[r.Collection].(map[string]any)
		if coll == nil {
			continue
		}
		sum, _ := coll[r.ID].(map[string]any)
		if sum == nil {
			continue
		}
		if sum["#"] == nil {
			continue
		}
		out = append(out, sum)
	}
	return out, nil
}

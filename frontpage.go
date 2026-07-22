package datorium

import (
	"fmt"

	"github.com/JohnAD/datorium-client-go/refs"
	"github.com/JohnAD/ojson"
)

// AppendCachedRefOp returns an RFC6902 "add" operation that appends a
// @@__collection__id value to a document array field (e.g. Users.todoLists).
// Path uses JSON Pointer array-append ("/-").
//
// This returns a map for use with the raw Patch escape hatch. Prefer
// CollectionClient.CreatePatch / PatchDoc for new code.
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
func (rr ReadResult) SummariesForArrayField(arrayField string) ([]ojson.JSONValue, error) {
	field := rr.SOT.Get(arrayField)
	if field.IsMissing() {
		return nil, nil
	}
	if !field.IsArray() {
		return nil, fmt.Errorf("datorium: sot field %q is not an array", arrayField)
	}
	var out []ojson.JSONValue
	for _, elem := range field.Items() {
		if !elem.IsString() {
			continue
		}
		r, isRef, err := refs.Parse(elem.ToStringOrEmpty())
		if err != nil {
			return nil, err
		}
		if !isRef || r.Kind != refs.Cached {
			continue
		}
		coll := rr.CacheSummaries.Get(r.Collection)
		if !coll.IsObject() {
			continue
		}
		sum := coll.Get(r.ID)
		if !sum.IsObject() {
			continue
		}
		if sum.Get("#").IsMissing() || sum.Get("#").IsNull() {
			continue
		}
		out = append(out, sum)
	}
	return out, nil
}

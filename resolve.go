package datorium

import (
	"context"
	"fmt"

	"github.com/JohnAD/datorium-client-go/v2/refs"
	"github.com/JohnAD/ojson"
)

// ResolveDirectRef reads the document targeted by a @__Collection__id string.
func (c *Client) ResolveDirectRef(ctx context.Context, ref string, opts *ReadOptions) (ReadResult, error) {
	r, ok, err := refs.Parse(ref)
	if err != nil {
		return ReadResult{}, err
	}
	if !ok || r.Kind != refs.Direct {
		return ReadResult{}, fmt.Errorf("datorium: not a direct reference: %q", ref)
	}
	return c.Read(ctx, r.Collection, r.ID, opts)
}

// ResolveRefsInSOT walks top-level SOT string fields and resolves direct refs
// up to maxDepth (1 = only the provided document's direct fields).
func (c *Client) ResolveRefsInSOT(ctx context.Context, sot ojson.JSONValue, maxDepth int) (map[string]ReadResult, error) {
	if maxDepth < 1 {
		maxDepth = 1
	}
	out := map[string]ReadResult{}
	seen := map[string]bool{}
	var walk func(doc ojson.JSONValue, depth int) error
	walk = func(doc ojson.JSONValue, depth int) error {
		if depth > maxDepth || !doc.IsObject() {
			return nil
		}
		for key := range doc.ToMap() {
			v := doc.Get(key)
			if !v.IsString() {
				continue
			}
			r, isRef, err := refs.Parse(v.ToStringOrEmpty())
			if err != nil {
				return err
			}
			if !isRef || r.Kind != refs.Direct {
				continue
			}
			refKey := r.Collection + "/" + r.ID
			if seen[refKey] {
				continue
			}
			seen[refKey] = true
			rr, err := c.Read(ctx, r.Collection, r.ID, nil)
			if err != nil {
				return err
			}
			out[refKey] = rr
			if err := walk(rr.SOT, depth+1); err != nil {
				return err
			}
		}
		return nil
	}
	if err := walk(sot, 1); err != nil {
		return out, err
	}
	return out, nil
}

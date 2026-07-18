package datorium

import (
	"context"
	"fmt"
)

// WriteResult is a successful create/patch/delete summary.
type WriteResult struct {
	Result        Result
	Collection    string
	ID            string
	Schema        string
	Version       string // create/delete: "#"; patch: versions.after
	VersionBefore string
	OperationID   string
}

// ReadResult is a successful read summary.
type ReadResult struct {
	Result         Result
	Collection     string
	ID             string
	SOT            map[string]any
	ExtraFields    map[string]any
	CacheSummaries map[string]any
}

// Create creates a document. Empty id means server-assigned ULID ("null" parm).
func (c *Client) Create(ctx context.Context, collection, id string, content map[string]any) (WriteResult, error) {
	if collection == "" {
		return WriteResult{}, fmt.Errorf("datorium: collection is required")
	}
	est, err := c.ensureEstablished(ctx)
	if err != nil {
		return WriteResult{}, err
	}
	parm := id
	if parm == "" {
		parm = "null"
	}
	detail := ensureOperationID(cloneMap(content))
	line, err := BuildCommand("create", collection, parm, detail)
	if err != nil {
		return WriteResult{}, err
	}
	route, err := c.routeDocument(est, id, RouteWrite)
	if err != nil {
		return WriteResult{}, err
	}
	res, err := c.executeRouted(ctx, route, line)
	if err != nil {
		return WriteResult{}, err
	}
	return writeResultFrom(res), nil
}

// ReadOptions controls optional read-scope fields.
type ReadOptions struct {
	ExtraFields    bool
	CacheSummaries bool
}

// Read reads a document by id.
func (c *Client) Read(ctx context.Context, collection, id string, opts *ReadOptions) (ReadResult, error) {
	if collection == "" || id == "" {
		return ReadResult{}, fmt.Errorf("datorium: collection and id are required")
	}
	est, err := c.ensureEstablished(ctx)
	if err != nil {
		return ReadResult{}, err
	}
	detail := map[string]any{}
	if opts != nil {
		if opts.ExtraFields {
			detail["extraFields"] = true
		}
		if opts.CacheSummaries {
			detail["cacheSummaries"] = true
		}
	}
	if len(detail) == 0 {
		detail = map[string]any{}
	}
	line, err := BuildCommand("read", collection, id, detail)
	if err != nil {
		return ReadResult{}, err
	}
	route, err := c.routeDocument(est, id, RouteRead)
	if err != nil {
		return ReadResult{}, err
	}
	res, err := c.executeRouted(ctx, route, line)
	if err != nil {
		return ReadResult{}, err
	}
	return readResultFrom(res), nil
}

// Patch applies RFC6902 operations to a document. detail must include "$", "#",
// and usually "RFC6902". operationId is filled if missing.
func (c *Client) Patch(ctx context.Context, collection, id string, detail map[string]any) (WriteResult, error) {
	if collection == "" || id == "" {
		return WriteResult{}, fmt.Errorf("datorium: collection and id are required")
	}
	est, err := c.ensureEstablished(ctx)
	if err != nil {
		return WriteResult{}, err
	}
	detail = ensureOperationID(cloneMap(detail))
	line, err := BuildCommand("patch", collection, id, detail)
	if err != nil {
		return WriteResult{}, err
	}
	route, err := c.routeDocument(est, id, RouteWrite)
	if err != nil {
		return WriteResult{}, err
	}
	res, err := c.executeRouted(ctx, route, line)
	if err != nil {
		return WriteResult{}, err
	}
	return writeResultFrom(res), nil
}

// Delete deletes a document. detail must include "#"; "$" is recommended.
func (c *Client) Delete(ctx context.Context, collection, id string, detail map[string]any) (WriteResult, error) {
	if collection == "" || id == "" {
		return WriteResult{}, fmt.Errorf("datorium: collection and id are required")
	}
	est, err := c.ensureEstablished(ctx)
	if err != nil {
		return WriteResult{}, err
	}
	detail = ensureOperationID(cloneMap(detail))
	line, err := BuildCommand("delete", collection, id, detail)
	if err != nil {
		return WriteResult{}, err
	}
	route, err := c.routeDocument(est, id, RouteWrite)
	if err != nil {
		return WriteResult{}, err
	}
	res, err := c.executeRouted(ctx, route, line)
	if err != nil {
		return WriteResult{}, err
	}
	return writeResultFrom(res), nil
}

// PatchWithVersionRetry re-reads on versionMismatch and retries patch once.
func (c *Client) PatchWithVersionRetry(ctx context.Context, collection, id string, build func(sot map[string]any) (map[string]any, error)) (WriteResult, error) {
	rr, err := c.Read(ctx, collection, id, nil)
	if err != nil {
		return WriteResult{}, err
	}
	detail, err := build(rr.SOT)
	if err != nil {
		return WriteResult{}, err
	}
	wr, err := c.Patch(ctx, collection, id, detail)
	if err == nil {
		return wr, nil
	}
	if !IsAppCode(err, CodeVersionMismatch) {
		return WriteResult{}, err
	}
	rr, err = c.Read(ctx, collection, id, nil)
	if err != nil {
		return WriteResult{}, err
	}
	detail, err = build(rr.SOT)
	if err != nil {
		return WriteResult{}, err
	}
	return c.Patch(ctx, collection, id, detail)
}

func writeResultFrom(res Result) WriteResult {
	wr := WriteResult{
		Result:      res,
		Collection:  res.StringField("collection"),
		ID:          res.StringField("id"),
		Schema:      res.StringField("$"),
		Version:     res.StringField("#"),
		OperationID: res.StringField("operationId"),
	}
	// Patch responses use versions.{before,after} instead of top-level "#".
	if versions := res.MapField("versions"); versions != nil {
		wr.VersionBefore = asString(versions["before"])
		if after := asString(versions["after"]); after != "" {
			wr.Version = after
		}
	}
	return wr
}

func readResultFrom(res Result) ReadResult {
	return ReadResult{
		Result:         res,
		Collection:     res.StringField("collection"),
		ID:             res.StringField("id"),
		SOT:            res.MapField("sot"),
		ExtraFields:    res.MapField("extraFields"),
		CacheSummaries: res.MapField("cacheSummaries"),
	}
}

func cloneMap(in map[string]any) map[string]any {
	if in == nil {
		return map[string]any{}
	}
	out := make(map[string]any, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

package datorium

import (
	"context"
	"fmt"

	"github.com/JohnAD/ojson"
)

// DocMeta holds DatoriumDB system fields from a document.
type DocMeta struct {
	ID      string // !
	Schema  string // $
	Version string // #
}

// TypedRead is a successful typed document read.
type TypedRead[T any] struct {
	Doc            T
	Meta           DocMeta
	ExtraFields    ojson.JSONValue
	CacheSummaries ojson.JSONValue
	Result         Result
}

// CreateDoc creates a document from a typed content struct. id must be
// ojson.NewVoid() (auto ULID, access-language parm null) or a non-empty
// string JSONValue. Empty string IDs are rejected. "$" is injected from col;
// if the struct already produced a disagreeing "$", CreateDoc returns an error.
//
// Document bodies are serialized via ojson with stable field order; they are
// never stored as map[string]any.
//
// Note: CreateDoc is a package function (not a Client method) because Go does
// not allow type parameters on methods.
func CreateDoc[T any](ctx context.Context, c *Client, col Collection[T], id ojson.JSONValue, doc T) (WriteResult, error) {
	if c == nil {
		return WriteResult{}, fmt.Errorf("datorium: nil client")
	}
	if col.Name == "" {
		return WriteResult{}, fmt.Errorf("datorium: collection is required")
	}
	parm, routeID, err := createParm(id)
	if err != nil {
		return WriteResult{}, err
	}
	detail, err := ojson.NewObjectFromStructTry(doc)
	if err != nil {
		return WriteResult{}, fmt.Errorf("datorium: marshal document: %w", err)
	}
	if !detail.IsObject() {
		return WriteResult{}, fmt.Errorf("datorium: document must marshal to a JSON object")
	}
	marker := col.SchemaMarker()
	if detail.HasField("$") {
		existing := detail.Get("$").ToStringOrEmpty()
		if existing != marker {
			return WriteResult{}, fmt.Errorf("datorium: document $ %q disagrees with collection marker %q", existing, marker)
		}
	} else {
		detail.Set("$", ojson.NewString(marker))
	}
	ensureOperationIDValue(detail)

	est, err := c.ensureEstablished(ctx)
	if err != nil {
		return WriteResult{}, err
	}
	line, err := BuildCommandOrdered("create", col.Name, parm, detail)
	if err != nil {
		return WriteResult{}, err
	}
	route, err := c.routeDocument(est, routeID, RouteWrite)
	if err != nil {
		return WriteResult{}, err
	}
	res, err := c.executeRouted(ctx, route, line)
	if err != nil {
		return WriteResult{}, err
	}
	return writeResultFrom(res), nil
}

// ReadDoc reads a document and unmarshals SOT content into T via ojson.
// System fields !/$/# are exposed on Meta and are not required on T.
func ReadDoc[T any](ctx context.Context, c *Client, col Collection[T], id string, opts *ReadOptions) (TypedRead[T], error) {
	var zero TypedRead[T]
	if c == nil {
		return zero, fmt.Errorf("datorium: nil client")
	}
	if col.Name == "" || id == "" {
		return zero, fmt.Errorf("datorium: collection and id are required")
	}
	est, err := c.ensureEstablished(ctx)
	if err != nil {
		return zero, err
	}
	detail := ojson.NewObject()
	if opts != nil {
		if opts.ExtraFields {
			detail.Set("extraFields", ojson.NewBoolean(true))
		}
		if opts.CacheSummaries {
			detail.Set("cacheSummaries", ojson.NewBoolean(true))
		}
	}
	line, err := BuildCommandOrdered("read", col.Name, id, detail)
	if err != nil {
		return zero, err
	}
	route, err := c.routeDocument(est, id, RouteRead)
	if err != nil {
		return zero, err
	}
	res, err := c.executeRouted(ctx, route, line)
	if err != nil {
		return zero, err
	}
	return typedReadFrom[T](res)
}

// DeleteDoc deletes a document using the collection schema marker and version.
func DeleteDoc[T any](ctx context.Context, c *Client, col Collection[T], id, version string) (WriteResult, error) {
	if c == nil {
		return WriteResult{}, fmt.Errorf("datorium: nil client")
	}
	if col.Name == "" || id == "" {
		return WriteResult{}, fmt.Errorf("datorium: collection and id are required")
	}
	if version == "" {
		return WriteResult{}, fmt.Errorf("datorium: version is required")
	}
	est, err := c.ensureEstablished(ctx)
	if err != nil {
		return WriteResult{}, err
	}
	detail := ojson.NewObject()
	detail.Set("$", ojson.NewString(col.SchemaMarker()))
	detail.Set("#", ojson.NewString(version))
	ensureOperationIDValue(detail)
	line, err := BuildCommandOrdered("delete", col.Name, id, detail)
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

func createParm(id ojson.JSONValue) (parm, routeID string, err error) {
	if id.IsMissing() {
		return "null", "", nil
	}
	if !id.IsString() {
		return "", "", fmt.Errorf("datorium: document id must be void or string, got %s", id.Kind())
	}
	s := id.ToStringOrEmpty()
	if s == "" {
		return "", "", fmt.Errorf("datorium: empty string id is not allowed; use ojson.NewVoid() for auto-id")
	}
	return s, s, nil
}

func typedReadFrom[T any](res Result) (TypedRead[T], error) {
	var out TypedRead[T]
	out.Result = res
	if len(res.Body) == 0 {
		return out, fmt.Errorf("datorium: empty response body")
	}
	env, err := ojson.ReadBytesNoSchema(res.Body)
	if err != nil {
		return out, fmt.Errorf("datorium: parse response: %w", err)
	}
	sot := env.Get("sot")
	if sot.IsMissing() || !sot.IsObject() {
		return out, fmt.Errorf("datorium: response missing sot object")
	}
	out.Meta = DocMeta{
		ID:      sot.Get("!").ToStringOrEmpty(),
		Schema:  sot.Get("$").ToStringOrEmpty(),
		Version: sot.Get("#").ToStringOrEmpty(),
	}
	var doc T
	if err := sot.ToStructTry(&doc); err != nil {
		return out, fmt.Errorf("datorium: unmarshal sot: %w", err)
	}
	out.Doc = doc
	out.ExtraFields = env.Get("extraFields")
	out.CacheSummaries = env.Get("cacheSummaries")
	return out, nil
}

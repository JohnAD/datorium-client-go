package datorium

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/JohnAD/ojson"
)

// collectionBinding is a private identity shared by a CollectionClient and the
// items/patches it creates. Identity equality prevents cross-collection use,
// even when two collections share the same Go document type T.
//
// A non-empty unique id is required: Go may coalesce distinct empty-struct
// addresses, which would make pointer equality unreliable.
type collectionBinding struct {
	id uint64
}

var nextCollectionBindingID atomic.Uint64

// CollectionClient is a typed, collection-scoped view of a Client.
// Obtain one with Collection[T].Bind after Establish.
type CollectionClient[T any] struct {
	client  *Client
	col     Collection[T]
	schema  ojson.JSONSchema
	binding *collectionBinding
}

// CollectionItem is a document snapshot owned by a CollectionClient.
// Mutate Doc, then CreatePatchFromChanges (or CreatePatch with hand-built ops).
// OriginalDoc is an independently decoded copy of the content at read time;
// the private original ojson baseline is what patch creation diffs against.
type CollectionItem[T any] struct {
	Doc            T
	OriginalDoc    T
	Meta           DocMeta
	ExtraFields    ojson.JSONValue
	CacheSummaries ojson.JSONValue
	Result         Result

	original ojson.JSONValue // content-only immutable baseline (no !/$/#)
	binding  *collectionBinding
}

// CollectionPatch is a schema-checked patch ready to send for one document.
// ID and Version come from the item used to create it.
type CollectionPatch[T any] struct {
	Patch ojson.Patch

	id      string
	version string
	marker  string
	binding *collectionBinding
}

// DocMeta holds DatoriumDB system fields from a document.
type DocMeta struct {
	ID      string // !
	Schema  string // $
	Version string // #
}

// Bind attaches this collection descriptor to an established client.
// It verifies the live schema name/version and compiles the ojson schema.
func (col Collection[T]) Bind(c *Client) (CollectionClient[T], error) {
	var zero CollectionClient[T]
	if c == nil {
		return zero, fmt.Errorf("datorium: nil client")
	}
	if col.Name == "" {
		return zero, fmt.Errorf("datorium: collection is required")
	}
	est := c.cache.get()
	if est == nil {
		return zero, fmt.Errorf("datorium: not established; call Establish before Bind")
	}
	entry, ok := est.Schemas[col.Name]
	if !ok {
		return zero, fmt.Errorf("datorium: schema for collection %q not in establishment", col.Name)
	}
	if entry.Version != col.Version {
		return zero, fmt.Errorf("datorium: schema version for %q is %d, collection declares %d", col.Name, entry.Version, col.Version)
	}
	if entry.Doc.IsMissing() || !entry.Doc.IsObject() {
		return zero, fmt.Errorf("datorium: schema document for %q is missing", col.Name)
	}
	schema, err := ojson.CompileSchemaBytes(entry.Doc.ToJSONBytes(), ojson.WithStringFormats(datoriumStringFormats()))
	if err != nil {
		return zero, fmt.Errorf("datorium: compile schema for %q: %w", col.Name, err)
	}
	return CollectionClient[T]{
		client:  c,
		col:     col,
		schema:  schema,
		binding: &collectionBinding{id: nextCollectionBindingID.Add(1)},
	}, nil
}

// Client returns the underlying smart client.
func (cc CollectionClient[T]) Client() *Client { return cc.client }

// Collection returns the catalog descriptor used to bind this client.
func (cc CollectionClient[T]) Collection() Collection[T] { return cc.col }

// CompiledSchema returns the ojson schema compiled at Bind time.
func (cc CollectionClient[T]) CompiledSchema() ojson.JSONSchema { return cc.schema }

// ID returns the document id the patch targets.
func (p CollectionPatch[T]) ID() string { return p.id }

// Version returns the optimistic concurrency version (#) the patch targets.
func (p CollectionPatch[T]) Version() string { return p.version }

// CreateDoc creates a document from the typed content struct doc.
// Pass id nil to mint a ULID locally, or a non-empty *string for an explicit
// id. Empty strings are rejected. The server never assigns create IDs.
//
// "$" is taken from the collection (error if doc already set a disagreeing "$").
// Content is serialized to ordered JSON once before any network attempt so
// retries keep the same id and bytes.
func (cc CollectionClient[T]) CreateDoc(ctx context.Context, id *string, doc T) (WriteResult, error) {
	if err := cc.requireBound(); err != nil {
		return WriteResult{}, err
	}
	docID, err := resolveCreateID(id)
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
	marker := cc.col.SchemaMarker()
	if detail.HasField("$") {
		existing := detail.Get("$").ToStringOrEmpty()
		if existing != marker {
			return WriteResult{}, fmt.Errorf("datorium: document $ %q disagrees with collection marker %q", existing, marker)
		}
	} else {
		detail.Set("$", ojson.NewString(marker))
	}
	ensureOperationIDValue(detail)

	line, err := BuildCommandOrdered("create", cc.col.Name, docID, detail)
	if err != nil {
		return WriteResult{}, err
	}
	return cc.client.executeCreate(ctx, cc.col.Name, docID, line)
}

// GetDoc reads a document by id (no extra fields / cache summaries).
func (cc CollectionClient[T]) GetDoc(ctx context.Context, id string) (CollectionItem[T], error) {
	return cc.GetDocOpts(ctx, id, nil)
}

// GetDocOpts reads a document by id with optional extra fields / cache summaries.
func (cc CollectionClient[T]) GetDocOpts(ctx context.Context, id string, opts *ReadOptions) (CollectionItem[T], error) {
	var zero CollectionItem[T]
	if err := cc.requireBound(); err != nil {
		return zero, err
	}
	if id == "" {
		return zero, fmt.Errorf("datorium: id is required")
	}
	est, err := cc.client.ensureEstablished(ctx)
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
	line, err := BuildCommandOrdered("read", cc.col.Name, id, detail)
	if err != nil {
		return zero, err
	}
	route, err := cc.client.routeDocument(est, id, RouteRead)
	if err != nil {
		return zero, err
	}
	res, err := cc.client.executeRouted(ctx, route, line, func(est *Establishment) (Route, error) {
		return cc.client.routeDocument(est, id, RouteRead)
	})
	if err != nil {
		return zero, err
	}
	return cc.itemFromResult(res)
}

// DeleteDoc deletes the document represented by item (optimistic concurrency).
func (cc CollectionClient[T]) DeleteDoc(ctx context.Context, item CollectionItem[T]) (WriteResult, error) {
	if err := cc.requireBound(); err != nil {
		return WriteResult{}, err
	}
	if err := cc.requireItem(item); err != nil {
		return WriteResult{}, err
	}
	if item.Meta.ID == "" {
		return WriteResult{}, fmt.Errorf("datorium: item id is required")
	}
	if item.Meta.Version == "" {
		return WriteResult{}, fmt.Errorf("datorium: item version is required")
	}

	est, err := cc.client.ensureEstablished(ctx)
	if err != nil {
		return WriteResult{}, err
	}
	detail := ojson.NewObject()
	detail.Set("$", ojson.NewString(cc.col.SchemaMarker()))
	detail.Set("#", ojson.NewString(item.Meta.Version))
	ensureOperationIDValue(detail)
	line, err := BuildCommandOrdered("delete", cc.col.Name, item.Meta.ID, detail)
	if err != nil {
		return WriteResult{}, err
	}
	route, err := cc.client.routeDocument(est, item.Meta.ID, RouteWrite)
	if err != nil {
		return WriteResult{}, err
	}
	docID := item.Meta.ID
	res, err := cc.client.executeRouted(ctx, route, line, func(est *Establishment) (Route, error) {
		return cc.client.routeDocument(est, docID, RouteWrite)
	})
	if err != nil {
		return WriteResult{}, err
	}
	return writeResultFrom(res), nil
}

// CreatePatchFromChanges diffs the item's immutable original content against
// the current Doc using the bound schema. The resulting patch carries the
// item's id and version.
func (cc CollectionClient[T]) CreatePatchFromChanges(item CollectionItem[T]) (CollectionPatch[T], error) {
	var zero CollectionPatch[T]
	if err := cc.requireBound(); err != nil {
		return zero, err
	}
	if err := cc.requireItem(item); err != nil {
		return zero, err
	}
	if err := cc.requireItemMeta(item); err != nil {
		return zero, err
	}
	after, err := contentObjectFromDoc(item.Doc)
	if err != nil {
		return zero, err
	}
	patch, err := ojson.Diff(item.original, after, ojson.WithPatchSchema(cc.schema))
	if err != nil {
		return zero, fmt.Errorf("datorium: diff changes: %w", err)
	}
	if patch.Len() == 0 {
		return zero, fmt.Errorf("datorium: patch has no operations")
	}
	return CollectionPatch[T]{
		Patch:   patch,
		id:      item.Meta.ID,
		version: item.Meta.Version,
		marker:  cc.col.SchemaMarker(),
		binding: cc.binding,
	}, nil
}

// CreatePatch validates a hand-built ojson.Patch against the item's original
// content and bound schema, then wraps it with the item's id and version.
func (cc CollectionClient[T]) CreatePatch(item CollectionItem[T], patch ojson.Patch) (CollectionPatch[T], error) {
	var zero CollectionPatch[T]
	if err := cc.requireBound(); err != nil {
		return zero, err
	}
	if err := cc.requireItem(item); err != nil {
		return zero, err
	}
	if err := cc.requireItemMeta(item); err != nil {
		return zero, err
	}
	if patch.Len() == 0 {
		return zero, fmt.Errorf("datorium: patch has no operations")
	}
	if err := ojson.ValidatePatch(item.original, patch, ojson.WithPatchSchema(cc.schema)); err != nil {
		return zero, fmt.Errorf("datorium: validate patch: %w", err)
	}
	return CollectionPatch[T]{
		Patch:   patch,
		id:      item.Meta.ID,
		version: item.Meta.Version,
		marker:  cc.col.SchemaMarker(),
		binding: cc.binding,
	}, nil
}

// PatchDoc applies a CollectionPatch created by this collection client.
func (cc CollectionClient[T]) PatchDoc(ctx context.Context, patch CollectionPatch[T]) (WriteResult, error) {
	if err := cc.requireBound(); err != nil {
		return WriteResult{}, err
	}
	if patch.binding == nil || cc.binding == nil || patch.binding.id != cc.binding.id {
		return WriteResult{}, fmt.Errorf("datorium: patch belongs to a different collection binding")
	}
	if patch.id == "" {
		return WriteResult{}, fmt.Errorf("datorium: patch id is required")
	}
	if patch.version == "" {
		return WriteResult{}, fmt.Errorf("datorium: patch version is required")
	}
	if patch.Patch.Len() == 0 {
		return WriteResult{}, fmt.Errorf("datorium: patch has no operations")
	}
	if patch.marker != "" && patch.marker != cc.col.SchemaMarker() {
		return WriteResult{}, fmt.Errorf("datorium: patch schema marker %q disagrees with collection %q", patch.marker, cc.col.SchemaMarker())
	}

	est, err := cc.client.ensureEstablished(ctx)
	if err != nil {
		return WriteResult{}, err
	}

	detail := ojson.NewObject()
	detail.Set("$", ojson.NewString(cc.col.SchemaMarker()))
	detail.Set("#", ojson.NewString(patch.version))
	detail.Set("RFC6902", patch.Patch.ToJSONValue())
	ensureOperationIDValue(detail)

	line, err := BuildCommandOrdered("patch", cc.col.Name, patch.id, detail)
	if err != nil {
		return WriteResult{}, err
	}
	route, err := cc.client.routeDocument(est, patch.id, RouteWrite)
	if err != nil {
		return WriteResult{}, err
	}
	docID := patch.id
	res, err := cc.client.executeRouted(ctx, route, line, func(est *Establishment) (Route, error) {
		return cc.client.routeDocument(est, docID, RouteWrite)
	})
	if err != nil {
		return WriteResult{}, err
	}
	return writeResultFrom(res), nil
}

func (cc CollectionClient[T]) requireBound() error {
	if cc.client == nil || cc.binding == nil || cc.col.Name == "" {
		return fmt.Errorf("datorium: collection client is not bound")
	}
	return nil
}

func (cc CollectionClient[T]) requireItem(item CollectionItem[T]) error {
	if item.binding == nil || cc.binding == nil || item.binding.id != cc.binding.id {
		return fmt.Errorf("datorium: item belongs to a different collection binding")
	}
	if item.original.IsMissing() || !item.original.IsObject() {
		return fmt.Errorf("datorium: item has no original content baseline")
	}
	return nil
}

func (cc CollectionClient[T]) requireItemMeta(item CollectionItem[T]) error {
	if item.Meta.ID == "" {
		return fmt.Errorf("datorium: item id is required")
	}
	if item.Meta.Version == "" {
		return fmt.Errorf("datorium: item version is required")
	}
	marker := cc.col.SchemaMarker()
	if item.Meta.Schema != "" && item.Meta.Schema != marker {
		return fmt.Errorf("datorium: item schema %q disagrees with collection marker %q", item.Meta.Schema, marker)
	}
	return nil
}

func (cc CollectionClient[T]) itemFromResult(res Result) (CollectionItem[T], error) {
	var out CollectionItem[T]
	out.Result = res
	out.binding = cc.binding
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

	content := sot.Clone()
	_ = content.Remove("!")
	_ = content.Remove("$")
	_ = content.Remove("#")
	out.original = content.Clone()

	var doc T
	if err := content.ToStructTry(&doc); err != nil {
		return out, fmt.Errorf("datorium: unmarshal sot: %w", err)
	}
	out.Doc = doc

	var original T
	if err := out.original.ToStructTry(&original); err != nil {
		return out, fmt.Errorf("datorium: unmarshal original sot: %w", err)
	}
	out.OriginalDoc = original

	out.ExtraFields = env.Get("extraFields")
	out.CacheSummaries = env.Get("cacheSummaries")
	return out, nil
}

func contentObjectFromDoc[T any](doc T) (ojson.JSONValue, error) {
	detail, err := ojson.NewObjectFromStructTry(doc)
	if err != nil {
		return ojson.NewVoid(), fmt.Errorf("datorium: marshal document: %w", err)
	}
	if !detail.IsObject() {
		return ojson.NewVoid(), fmt.Errorf("datorium: document must marshal to a JSON object")
	}
	_ = detail.Remove("!")
	_ = detail.Remove("$")
	_ = detail.Remove("#")
	return detail, nil
}

func resolveCreateID(id *string) (string, error) {
	if id == nil {
		return NewDocumentID(), nil
	}
	if *id == "" {
		return "", fmt.Errorf("datorium: empty string id is not allowed; pass nil for a client-minted ULID")
	}
	return *id, nil
}

package datorium

import (
	"fmt"
	"strings"
)

// CollectionRef identifies a collection and expected schema version for
// Establish catalog validation. Collection[T] implements this interface.
type CollectionRef interface {
	CollectionName() string
	SchemaVersion() int
}

// Collection is a compile-time binding of a document content type T to a
// collection name and schema version. Declare once in application code, then
// Bind to obtain a CollectionClient (lazy-establishes on first Bind):
//
//	var Todos = datorium.MustCollection[Todo]("Todos", 0)
//	todos, err := Todos.Bind(ctx, client)
type Collection[T any] struct {
	Name    string
	Version int
}

// MustCollection returns a Collection[T]. It panics if name is empty or
// schemaVersion is negative.
func MustCollection[T any](name string, schemaVersion int) Collection[T] {
	name = strings.TrimSpace(name)
	if name == "" {
		panic("datorium: collection name is required")
	}
	if schemaVersion < 0 {
		panic("datorium: schema version must be non-negative")
	}
	return Collection[T]{Name: name, Version: schemaVersion}
}

// CollectionName implements CollectionRef.
func (c Collection[T]) CollectionName() string { return c.Name }

// SchemaVersion implements CollectionRef.
func (c Collection[T]) SchemaVersion() int { return c.Version }

// SchemaMarker returns the "$" value "Name:version".
func (c Collection[T]) SchemaMarker() string {
	return fmt.Sprintf("%s:%d", c.Name, c.Version)
}

// CatalogMismatchCode identifies one catalog validation failure.
type CatalogMismatchCode string

const (
	CatalogCollectionNotFound    CatalogMismatchCode = "collectionNotFound"
	CatalogSchemaVersionMismatch CatalogMismatchCode = "schemaVersionMismatch"
)

// CatalogMismatch is one declared collection that does not match the live
// establishment document.
type CatalogMismatch struct {
	Collection string
	Code       CatalogMismatchCode
	Expected   int // declared schema version
	Actual     int // live schema version; -1 when collection is missing
}

// CatalogError is returned by Establish when the app catalog does not match
// the live establishment schemas.
type CatalogError struct {
	Mismatches []CatalogMismatch
}

func (e *CatalogError) Error() string {
	if e == nil || len(e.Mismatches) == 0 {
		return "datorium: catalog mismatch"
	}
	parts := make([]string, 0, len(e.Mismatches))
	for _, m := range e.Mismatches {
		switch m.Code {
		case CatalogCollectionNotFound:
			parts = append(parts, fmt.Sprintf("%s: collection not found (expected schema version %d)", m.Collection, m.Expected))
		case CatalogSchemaVersionMismatch:
			parts = append(parts, fmt.Sprintf("%s: schema version mismatch (expected %d, got %d)", m.Collection, m.Expected, m.Actual))
		default:
			parts = append(parts, fmt.Sprintf("%s: %s", m.Collection, m.Code))
		}
	}
	return "datorium: catalog mismatch: " + strings.Join(parts, "; ")
}

func validateCatalog(est *Establishment, cols []CollectionRef) error {
	if len(cols) == 0 {
		return nil
	}
	if est == nil {
		return fmt.Errorf("datorium: establishment is nil")
	}
	var mismatches []CatalogMismatch
	for _, col := range cols {
		if col == nil {
			continue
		}
		name := col.CollectionName()
		expected := col.SchemaVersion()
		entry, ok := est.Schemas[name]
		if !ok {
			mismatches = append(mismatches, CatalogMismatch{
				Collection: name,
				Code:       CatalogCollectionNotFound,
				Expected:   expected,
				Actual:     -1,
			})
			continue
		}
		if entry.Version != expected {
			mismatches = append(mismatches, CatalogMismatch{
				Collection: name,
				Code:       CatalogSchemaVersionMismatch,
				Expected:   expected,
				Actual:     entry.Version,
			})
		}
	}
	if len(mismatches) == 0 {
		return nil
	}
	return &CatalogError{Mismatches: mismatches}
}

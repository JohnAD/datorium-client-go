// Package refs parses DatoriumDB document reference strings.
package refs

import (
	"fmt"
	"strings"
)

// Kind distinguishes live direct refs from cached summary refs.
type Kind int

const (
	Direct Kind = iota + 1
	Cached
)

// Ref is a parsed @__Collection__id or @@__Collection__id value.
type Ref struct {
	Kind       Kind
	Collection string
	ID         string
	Raw        string
}

// Parse parses a reference string. Non-reference strings return ok=false.
func Parse(s string) (Ref, bool, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Ref{}, false, nil
	}
	kind := Direct
	rest := s
	switch {
	case strings.HasPrefix(s, "@@__"):
		kind = Cached
		rest = strings.TrimPrefix(s, "@@__")
	case strings.HasPrefix(s, "@__"):
		kind = Direct
		rest = strings.TrimPrefix(s, "@__")
	default:
		return Ref{}, false, nil
	}
	idx := strings.Index(rest, "__")
	if idx <= 0 || idx+2 >= len(rest) {
		return Ref{}, false, fmt.Errorf("invalid reference %q", s)
	}
	collection := rest[:idx]
	id := rest[idx+2:]
	if collection == "" || id == "" {
		return Ref{}, false, fmt.Errorf("invalid reference %q", s)
	}
	return Ref{Kind: kind, Collection: collection, ID: id, Raw: s}, true, nil
}

// FormatDirect builds @__Collection__id.
func FormatDirect(collection, id string) string {
	return "@__" + collection + "__" + id
}

// FormatCached builds @@__Collection__id.
func FormatCached(collection, id string) string {
	return "@@__" + collection + "__" + id
}

package datorium

import (
	"context"
	"fmt"

	"github.com/JohnAD/datorium-client-go/searchpath"
)

// SearchResult is a successful search response.
type SearchResult struct {
	Result     Result
	Collection string
	Search     string
	Matches    []string
}

// Search runs a precompiled search. For simple equals-string clauses, pass
// pathSegments via searchpath.EqualsStringSegments so the client can route
// to the correct search shard. If pathSegments is nil, the command is sent
// to PreferServer / establishment and relies on wrongMachine bounce.
func (c *Client) Search(ctx context.Context, collection, searchName string, vars map[string]any, pathSegments []string) (SearchResult, error) {
	if collection == "" || searchName == "" {
		return SearchResult{}, fmt.Errorf("datorium: collection and searchName are required")
	}
	est, err := c.ensureEstablished(ctx)
	if err != nil {
		return SearchResult{}, err
	}
	if vars == nil {
		vars = map[string]any{}
	}
	line, err := BuildCommand("search", collection, searchName, vars)
	if err != nil {
		return SearchResult{}, err
	}
	var route Route
	if pathSegments != nil {
		slot := searchpath.ShardSlot(pathSegments)
		route, err = c.routeSearch(est, slot)
		if err != nil {
			return SearchResult{}, err
		}
	} else {
		name := est.General.EstablishmentServer
		if c.cfg.PreferServer != "" {
			name = c.cfg.PreferServer
		}
		u := est.ServerBaseURL(name)
		if u == "" {
			u = c.cfg.EstablishmentURL
		}
		route = Route{ServerName: name, BaseURL: c.rewriteURL(name, u)}
	}
	resolve := func(est *Establishment) (Route, error) {
		if pathSegments != nil {
			return c.routeSearch(est, searchpath.ShardSlot(pathSegments))
		}
		name := est.General.EstablishmentServer
		if c.cfg.PreferServer != "" {
			name = c.cfg.PreferServer
		}
		u := est.ServerBaseURL(name)
		if u == "" {
			u = c.cfg.EstablishmentURL
		}
		return Route{ServerName: name, BaseURL: c.rewriteURL(name, u)}, nil
	}
	res, err := c.executeRouted(ctx, route, line, resolve)
	if err != nil {
		return SearchResult{}, err
	}
	return searchResultFrom(res), nil
}

func searchResultFrom(res Result) SearchResult {
	sr := SearchResult{
		Result:     res,
		Collection: res.StringField("collection"),
		Search:     res.StringField("search"),
	}
	matches := res.ValueField("matches")
	if matches.IsArray() {
		for _, v := range matches.Items() {
			sr.Matches = append(sr.Matches, jsonValueAsString(v))
		}
	}
	return sr
}

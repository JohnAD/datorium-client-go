package datorium

import (
	"context"
	"encoding/json"
	"fmt"
)

// EnsureCollection posts collectionEnsure to the establishment server.
// Use an admin JWT as the client's Token / TokenSource.
// Pass schema for create / schema-match; pass upgrade for one-step upgrade.
// Do not pass both when creating. Multi-version upgrades require looping.
// On success, refreshes the client's establishment cache so schemas are current.
func (c *Client) EnsureCollection(ctx context.Context, collection string, schema, upgrade map[string]any) (Result, error) {
	if collection == "" {
		return Result{}, fmt.Errorf("datorium: collection is required")
	}
	detail := map[string]any{}
	if schema != nil {
		detail["schema"] = schema
	}
	if upgrade != nil {
		detail["upgrade"] = upgrade
	}
	body, err := buildAdminCommand("collectionEnsure", collection, "", detail)
	if err != nil {
		return Result{}, err
	}
	return c.postAdminAndRefresh(ctx, body)
}

// EnsureSearch posts searchEnsure. definition is the search definition object
// (same shape as the former search-definition JSON file).
// On success, refreshes the client's establishment cache.
func (c *Client) EnsureSearch(ctx context.Context, collection, searchName string, definition map[string]any) (Result, error) {
	if collection == "" || searchName == "" {
		return Result{}, fmt.Errorf("datorium: collection and searchName are required")
	}
	if definition == nil {
		definition = map[string]any{}
	}
	body, err := buildAdminCommand("searchEnsure", collection, searchName, definition)
	if err != nil {
		return Result{}, err
	}
	return c.postAdminAndRefresh(ctx, body)
}

// DeleteSearch posts searchDelete.
// On success, refreshes the client's establishment cache.
func (c *Client) DeleteSearch(ctx context.Context, collection, searchName string) (Result, error) {
	if collection == "" || searchName == "" {
		return Result{}, fmt.Errorf("datorium: collection and searchName are required")
	}
	body, err := buildAdminCommand("searchDelete", collection, searchName, map[string]any{})
	if err != nil {
		return Result{}, err
	}
	return c.postAdminAndRefresh(ctx, body)
}

func (c *Client) postAdminAndRefresh(ctx context.Context, body []byte) (Result, error) {
	res, err := c.Command(ctx, c.cfg.EstablishmentURL, body)
	if err != nil {
		return Result{}, err
	}
	if err := c.Establish(ctx); err != nil {
		return res, fmt.Errorf("datorium: admin command succeeded but establish refresh failed: %w", err)
	}
	return res, nil
}

func buildAdminCommand(command, target, parameter string, detail any) ([]byte, error) {
	raw, err := json.Marshal(detail)
	if err != nil {
		return nil, fmt.Errorf("datorium: marshal detail: %w", err)
	}
	if len(raw) == 0 || raw[0] != '{' {
		return nil, fmt.Errorf("datorium: detail must be a JSON object")
	}
	return marshalCommandRequest(command, target, parameter, raw)
}

package datorium

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/oklog/ulid/v2"
)

// BuildCommand formats an access-language command with a strict-JSON detail object.
func BuildCommand(word, target, parm string, detail any) (string, error) {
	if word == "" || target == "" || parm == "" {
		return "", fmt.Errorf("datorium: word, target, and parm are required")
	}
	if detail == nil {
		detail = map[string]any{}
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return "", fmt.Errorf("datorium: marshal detail: %w", err)
	}
	return fmt.Sprintf("%s %s %s %s", word, target, parm, string(raw)), nil
}

// NewOperationID returns a new ULID string suitable for write operationId fields.
func NewOperationID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// Command posts a raw access-language command line to the given base URL
// (or the establishment URL when baseURL is empty), without smart routing.
func (c *Client) Command(ctx context.Context, baseURL, line string) (Result, error) {
	if baseURL == "" {
		baseURL = c.cfg.EstablishmentURL
	}
	return c.postCommand(ctx, baseURL, line)
}

func (c *Client) postCommand(ctx context.Context, baseURL, line string) (Result, error) {
	line = strings.TrimSpace(line)
	if line == "" {
		return Result{}, fmt.Errorf("datorium: empty command")
	}
	res, err := c.doJSON(ctx, http.MethodPost, baseURL, apiPrefix+"/command",
		bytes.NewReader([]byte(line)), "text/plain; charset=utf-8", true)
	if err != nil {
		return Result{}, err
	}
	return res, nil
}

// executeRouted posts a command with wrongMachine bounce handling.
func (c *Client) executeRouted(ctx context.Context, initial Route, line string) (Result, error) {
	base := initial.BaseURL
	if base == "" {
		base = c.cfg.EstablishmentURL
	}
	var last Result
	for attempt := 0; attempt <= c.wmRetries; attempt++ {
		res, err := c.postCommand(ctx, base, line)
		if err != nil {
			return Result{}, err
		}
		last = res
		if res.OK {
			return res, nil
		}
		ae := appErrorFromResult(res)
		if ae.Code != CodeWrongMachine {
			return res, ae
		}
		if attempt == c.wmRetries {
			return res, ae
		}
		// Refresh establish when the bounce advertises a config version we
		// don't have, or whenever we lack a cache.
		est := c.cache.get()
		if est == nil || (ae.ConfigVersion > 0 && ae.ConfigVersion > est.General.Version) {
			_ = c.Establish(ctx)
			est = c.cache.get()
		}
		next := ae.BaseURL
		server := ae.CorrectServer
		if est != nil && server != "" {
			if u := est.ServerBaseURL(server); u != "" {
				next = u
			}
			base = c.rewriteURL(server, next)
		} else if next != "" {
			base = c.rewriteURL(server, next)
		} else {
			return res, ae
		}
	}
	return last, appErrorFromResult(last)
}

func ensureOperationID(detail map[string]any) map[string]any {
	if detail == nil {
		detail = map[string]any{}
	}
	if _, ok := detail["operationId"]; !ok {
		detail["operationId"] = NewOperationID()
	}
	return detail
}

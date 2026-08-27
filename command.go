package datorium

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/oklog/ulid/v2"
)

// BuildCommand formats a four-field JSON command request with a strict-JSON detail object.
func BuildCommand(word, target, parm string, detail any) ([]byte, error) {
	if word == "" || target == "" || parm == "" {
		return nil, fmt.Errorf("datorium: word, target, and parm are required")
	}
	if detail == nil {
		detail = map[string]any{}
	}
	raw, err := json.Marshal(detail)
	if err != nil {
		return nil, fmt.Errorf("datorium: marshal detail: %w", err)
	}
	if len(raw) == 0 || raw[0] != '{' {
		return nil, fmt.Errorf("datorium: detail must be a JSON object")
	}
	return marshalCommandRequest(word, target, parm, raw)
}

func marshalCommandRequest(command, target, parameter string, detailJSON []byte) ([]byte, error) {
	cmd, err := json.Marshal(command)
	if err != nil {
		return nil, err
	}
	tgt, err := json.Marshal(target)
	if err != nil {
		return nil, err
	}
	parm, err := json.Marshal(parameter)
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	buf.WriteByte('{')
	buf.WriteString(`"command":`)
	buf.Write(cmd)
	buf.WriteString(`,"target":`)
	buf.Write(tgt)
	buf.WriteString(`,"parameter":`)
	buf.Write(parm)
	buf.WriteString(`,"detail":`)
	buf.Write(detailJSON)
	buf.WriteByte('}')
	return buf.Bytes(), nil
}

// NewOperationID returns a new ULID string suitable for write operationId fields.
func NewOperationID() string {
	return newULID()
}

// NewDocumentID returns a new ULID string suitable for client-supplied create IDs.
// The server never generates document IDs; callers should mint before create
// (Create / CollectionClient.CreateDoc do this when the id is empty / nil).
func NewDocumentID() string {
	return newULID()
}

func newULID() string {
	return ulid.MustNew(ulid.Timestamp(time.Now()), rand.Reader).String()
}

// Command posts a marshaled four-field JSON command to the given base URL
// (or the establishment URL when baseURL is empty), without smart routing.
func (c *Client) Command(ctx context.Context, baseURL string, body []byte) (Result, error) {
	if baseURL == "" {
		baseURL = c.cfg.EstablishmentURL
	}
	return c.postCommand(ctx, baseURL, body)
}

func (c *Client) postCommand(ctx context.Context, baseURL string, body []byte) (Result, error) {
	body = bytes.TrimSpace(body)
	if len(body) == 0 {
		return Result{}, fmt.Errorf("datorium: empty command")
	}
	res, err := c.doJSON(ctx, http.MethodPost, baseURL, apiPrefix+"/command",
		bytes.NewReader(body), "application/json", true)
	if err != nil {
		return Result{}, err
	}
	return res, nil
}

// routeResolver recomputes a command route from a fresh establishment document.
type routeResolver func(est *Establishment) (Route, error)

// executeRouted posts a command with wrongMachine bounce handling.
// On wrongMachine it always re-fetches establishment and recomputes the next
// hop via resolve. Bounce correctServer/baseURL/shardSlot are ignored;
// configVersion on the bounce is diagnostic-only (what that server thinks).
func (c *Client) executeRouted(ctx context.Context, initial Route, body []byte, resolve routeResolver) (Result, error) {
	base := initial.BaseURL
	if base == "" {
		base = c.cfg.EstablishmentURL
	}
	var last Result
	for attempt := 0; attempt <= c.wmRetries; attempt++ {
		res, err := c.postCommand(ctx, base, body)
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
		// Always refresh establishment. Bounce configVersion is not
		// authoritative and must not skip this refresh.
		if err := c.Establish(ctx); err != nil {
			return res, err
		}
		est := c.cache.get()
		if est == nil || resolve == nil {
			return res, ae
		}
		route, err := resolve(est)
		if err != nil {
			return res, err
		}
		if route.BaseURL == "" {
			return res, ae
		}
		base = route.BaseURL
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

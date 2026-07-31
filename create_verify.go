package datorium

import (
	"context"
	"errors"
	"time"
)

const defaultCreateAmbiguousVerifyDelay = 3 * time.Second

func (c *Client) createAmbiguousVerifyDelay() time.Duration {
	d := c.cfg.CreateAmbiguousVerifyDelay
	if d < 0 {
		return 0
	}
	if d == 0 {
		return defaultCreateAmbiguousVerifyDelay
	}
	return d
}

// executeCreate posts a pre-built create command line. The line must already
// include a client-supplied document id and a fully marshaled detail object so
// retries never re-stringify (map key order must not shift).
//
// On documentExists, a follow-up read treats an existing document as an
// idempotent success. On TransportError, waits CreateAmbiguousVerifyDelay then
// reads; if the document exists, returns success.
func (c *Client) executeCreate(ctx context.Context, collection, id, line string) (WriteResult, error) {
	est, err := c.ensureEstablished(ctx)
	if err != nil {
		return WriteResult{}, err
	}
	route, err := c.routeDocument(est, id, RouteWrite)
	if err != nil {
		return WriteResult{}, err
	}
	res, err := c.executeRouted(ctx, route, line, func(est *Establishment) (Route, error) {
		return c.routeDocument(est, id, RouteWrite)
	})
	if err == nil {
		return writeResultFrom(res), nil
	}
	if IsAppCode(err, CodeDocumentExists) {
		if wr, ok := c.writeResultFromExisting(ctx, collection, id); ok {
			return wr, nil
		}
		return WriteResult{}, err
	}
	var te *TransportError
	if errors.As(err, &te) {
		if wr, ok := c.verifyCreateAfterDelay(ctx, collection, id); ok {
			return wr, nil
		}
	}
	return WriteResult{}, err
}

func (c *Client) verifyCreateAfterDelay(ctx context.Context, collection, id string) (WriteResult, bool) {
	if delay := c.createAmbiguousVerifyDelay(); delay > 0 {
		timer := time.NewTimer(delay)
		defer timer.Stop()
		select {
		case <-ctx.Done():
			return WriteResult{}, false
		case <-timer.C:
		}
	}
	return c.writeResultFromExisting(ctx, collection, id)
}

func (c *Client) writeResultFromExisting(ctx context.Context, collection, id string) (WriteResult, bool) {
	rr, err := c.Read(ctx, collection, id, nil)
	if err != nil {
		return WriteResult{}, false
	}
	return WriteResult{
		Result:      rr.Result,
		Collection:  collection,
		ID:          id,
		Schema:      rr.SOT.Get("$").ToStringOrEmpty(),
		Version:     rr.SOT.Get("#").ToStringOrEmpty(),
		OperationID: rr.Result.StringField("operationId"),
	}, true
}

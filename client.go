package datorium

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	defaultTimeout         = 30 * time.Second
	maxResponseBodyBytes   = 8 << 20
	defaultWrongMachineMax = 3
	apiPrefix              = "/datoriumdb/v1"
)

// TokenSource supplies a bearer token (without the "Bearer " prefix).
type TokenSource interface {
	Token(ctx context.Context) (string, error)
}

// StaticToken is a TokenSource that always returns the same token.
type StaticToken string

func (s StaticToken) Token(context.Context) (string, error) { return string(s), nil }

// Config configures a Client.
type Config struct {
	// EstablishmentURL is the base URL of the establishment server
	// (scheme + host[:port], no path). Required.
	EstablishmentURL string

	// Token is a static bearer token. Ignored if TokenSource is set.
	Token string

	// TokenSource provides tokens dynamically.
	TokenSource TokenSource

	// HTTPClient overrides the default HTTP client.
	HTTPClient *http.Client

	// BaseURLRewrite maps establishment server baseURLs (or server names)
	// to host-reachable base URLs. Useful for Docker Compose from the host.
	// Keys may be Docker URLs ("http://server1:8080") or server names ("server1").
	BaseURLRewrite map[string]string

	// PreferServer, when dual-role eligible, is preferred for local routing.
	PreferServer string

	// WrongMachineRetries bounds wrongMachine bounce loops (default 3).
	WrongMachineRetries int

	// TransportRetries bounds retries on transport failures (default 0).
	TransportRetries int

	// UserAgent sets the User-Agent header.
	UserAgent string
}

// Client is a smart DatoriumDB client.
type Client struct {
	cfg       Config
	http      *http.Client
	tokens    TokenSource
	cache     establishmentCache
	userAgent string
	wmRetries int
	trRetries int
}

// New constructs a Client.
func New(cfg Config) (*Client, error) {
	if strings.TrimSpace(cfg.EstablishmentURL) == "" {
		return nil, fmt.Errorf("datorium: EstablishmentURL is required")
	}
	cfg.EstablishmentURL = strings.TrimRight(cfg.EstablishmentURL, "/")
	var tokens TokenSource
	switch {
	case cfg.TokenSource != nil:
		tokens = cfg.TokenSource
	case cfg.Token != "":
		tokens = StaticToken(cfg.Token)
	default:
		return nil, fmt.Errorf("datorium: Token or TokenSource is required")
	}
	hc := cfg.HTTPClient
	if hc == nil {
		hc = &http.Client{Timeout: defaultTimeout}
	}
	wm := cfg.WrongMachineRetries
	if wm <= 0 {
		wm = defaultWrongMachineMax
	}
	ua := cfg.UserAgent
	if ua == "" {
		ua = "datorium-client-go"
	}
	return &Client{
		cfg:       cfg,
		http:      hc,
		tokens:    tokens,
		userAgent: ua,
		wmRetries: wm,
		trRetries: cfg.TransportRetries,
	}, nil
}

// Close releases resources. Safe to call multiple times.
func (c *Client) Close() error { return nil }

// CachedEstablishment returns the last fetched establishment document, if any.
func (c *Client) CachedEstablishment() *Establishment {
	return c.cache.get()
}

func (c *Client) bearer(ctx context.Context) (string, error) {
	tok, err := c.tokens.Token(ctx)
	if err != nil {
		return "", err
	}
	tok = strings.TrimSpace(tok)
	tok = strings.TrimPrefix(tok, "Bearer ")
	if tok == "" {
		return "", fmt.Errorf("datorium: empty bearer token")
	}
	return tok, nil
}

func (c *Client) rewriteURL(serverName, baseURL string) string {
	if c.cfg.BaseURLRewrite != nil {
		if u, ok := c.cfg.BaseURLRewrite[serverName]; ok && u != "" {
			return strings.TrimRight(u, "/")
		}
		if u, ok := c.cfg.BaseURLRewrite[baseURL]; ok && u != "" {
			return strings.TrimRight(u, "/")
		}
	}
	return strings.TrimRight(baseURL, "/")
}

func (c *Client) doJSON(ctx context.Context, method, baseURL, path string, body io.Reader, contentType string, auth bool) (Result, error) {
	var lastErr error
	attempts := c.trRetries + 1
	for i := 0; i < attempts; i++ {
		res, err := c.doJSONOnce(ctx, method, baseURL, path, body, contentType, auth)
		if err == nil {
			return res, nil
		}
		lastErr = err
		var te *TransportError
		if !asTransport(err, &te) {
			return Result{}, err
		}
		if i+1 < attempts {
			select {
			case <-ctx.Done():
				return Result{}, ctx.Err()
			case <-time.After(time.Duration(i+1) * 100 * time.Millisecond):
			}
		}
	}
	return Result{}, lastErr
}

func asTransport(err error, target **TransportError) bool {
	if err == nil {
		return false
	}
	te, ok := err.(*TransportError)
	if ok {
		*target = te
		return true
	}
	return false
}

func (c *Client) doJSONOnce(ctx context.Context, method, baseURL, path string, body io.Reader, contentType string, auth bool) (Result, error) {
	url := strings.TrimRight(baseURL, "/") + path
	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return Result{}, &TransportError{Err: err}
	}
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	if auth {
		tok, err := c.bearer(ctx)
		if err != nil {
			return Result{}, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return Result{}, &TransportError{Err: err}
	}
	defer resp.Body.Close()
	limited := io.LimitReader(resp.Body, maxResponseBodyBytes+1)
	data, err := io.ReadAll(limited)
	if err != nil {
		return Result{}, &TransportError{StatusCode: resp.StatusCode, Err: err}
	}
	if len(data) > maxResponseBodyBytes {
		return Result{}, &TransportError{StatusCode: resp.StatusCode, Err: fmt.Errorf("response body exceeds %d bytes", maxResponseBodyBytes)}
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return Result{}, &TransportError{StatusCode: resp.StatusCode, Body: string(data)}
	}
	res, err := DecodeResult(data)
	if err != nil {
		return Result{}, &TransportError{StatusCode: resp.StatusCode, Body: string(data), Err: err}
	}
	return res, nil
}

// Health calls GET /datoriumdb/v1/health on the establishment URL (or override).
func (c *Client) Health(ctx context.Context) (Result, error) {
	return c.doJSON(ctx, http.MethodGet, c.cfg.EstablishmentURL, apiPrefix+"/health", nil, "", false)
}

// Ready calls GET /datoriumdb/v1/ready.
func (c *Client) Ready(ctx context.Context) (Result, error) {
	return c.doJSON(ctx, http.MethodGet, c.cfg.EstablishmentURL, apiPrefix+"/ready", nil, "", false)
}

// Establish fetches and caches establishment config from the establishment server.
// When cols are provided, each declared collection name and schema version is
// validated against the live establishment schemas (CatalogError on mismatch).
// Extra server collections not listed in cols are ignored.
func (c *Client) Establish(ctx context.Context, cols ...CollectionRef) error {
	res, err := c.doJSON(ctx, http.MethodGet, c.cfg.EstablishmentURL, apiPrefix+"/establish", nil, "", true)
	if err != nil {
		return err
	}
	est, err := parseEstablishment(res)
	if err != nil {
		return err
	}
	if err := validateCatalog(est, cols); err != nil {
		return err
	}
	c.cache.set(est)
	return nil
}

// Schema fetches a historic schema document.
func (c *Client) Schema(ctx context.Context, collection string, version int) (Result, error) {
	base, err := c.establishmentBase(ctx)
	if err != nil {
		return Result{}, err
	}
	path := fmt.Sprintf("%s/schema/%s/%d", apiPrefix, collection, version)
	res, err := c.doJSON(ctx, http.MethodGet, base, path, nil, "", true)
	if err != nil {
		return Result{}, err
	}
	if !res.OK {
		return res, appErrorFromResult(res)
	}
	return res, nil
}

func (c *Client) establishmentBase(ctx context.Context) (string, error) {
	est := c.cache.get()
	if est == nil {
		if err := c.Establish(ctx); err != nil {
			return "", err
		}
		est = c.cache.get()
	}
	name := est.General.EstablishmentServer
	u := est.ServerBaseURL(name)
	if u == "" {
		return c.cfg.EstablishmentURL, nil
	}
	return c.rewriteURL(name, u), nil
}

func (c *Client) ensureEstablished(ctx context.Context) (*Establishment, error) {
	if est := c.cache.get(); est != nil {
		return est, nil
	}
	if err := c.Establish(ctx); err != nil {
		return nil, err
	}
	est := c.cache.get()
	if est == nil {
		return nil, fmt.Errorf("datorium: establishment cache empty after fetch")
	}
	return est, nil
}

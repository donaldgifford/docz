package confluence

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

// gatewayBase is Atlassian's API gateway. Every request goes through it,
// because a scoped API token is accepted there and nowhere else, while an
// unscoped one works there too (DESIGN-0020 §5).
const gatewayBase = "https://api.atlassian.com/ex/confluence/"

// maxRetries is how many times a rate-limited request is retried.
const maxRetries = 3

// statusCurrent is the page status Export reads and writes: published, not
// draft, trashed, or archived.
const statusCurrent = "current"

// HTTPClient is the Client over Confluence Cloud's REST API v2.
type HTTPClient struct {
	site  string
	email string
	token string
	http  *http.Client
	sleep func(context.Context, time.Duration) error

	once    sync.Once
	base    string // gateway base for the site, resolved once
	baseErr error
}

// HTTPOption configures an HTTPClient.
type HTTPOption func(*HTTPClient)

// WithHTTPClient sets the *http.Client requests go through.
func WithHTTPClient(c *http.Client) HTTPOption {
	return func(h *HTTPClient) { h.http = c }
}

// NewHTTPClient speaks to site through Atlassian's gateway: it resolves the
// cloud id from <site>/_edge/tenant_info on first use and sends every request
// to api.atlassian.com/ex/confluence/<cloudId>, which accepts scoped and
// unscoped tokens alike. email and token are sent as basic auth and never
// appear in an error.
func NewHTTPClient(site, email, token string, opts ...HTTPOption) *HTTPClient {
	h := &HTTPClient{
		site:  strings.TrimSuffix(site, "/"),
		email: email,
		token: token,
		http:  &http.Client{Timeout: 60 * time.Second},
		sleep: sleepCtx,
	}

	for _, o := range opts {
		o(h)
	}

	return h
}

var _ Client = (*HTTPClient)(nil)

// sleepCtx waits for d or until ctx is done.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// gateway returns the site's gateway base, resolving the cloud id the first
// time. A failure is remembered, so a bad site fails every call the same way.
func (h *HTTPClient) gateway(ctx context.Context) (string, error) {
	h.once.Do(func() {
		var info struct {
			CloudID string `json:"cloudId"`
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, h.site+"/_edge/tenant_info", http.NoBody)
		if err != nil {
			h.baseErr = &RequestError{Op: "resolve cloud id", Err: err}

			return
		}

		if err := h.decode(req, "resolve cloud id", &info); err != nil {
			h.baseErr = err

			return
		}

		if info.CloudID == "" {
			h.baseErr = &RequestError{Op: "resolve cloud id", Err: errors.New("no cloudId in tenant_info")}

			return
		}

		h.base = gatewayBase + info.CloudID
	})

	return h.base, h.baseErr
}

// do sends one API request to path under the gateway, with body encoded as
// JSON when non-nil, and decodes the response into out when non-nil.
func (h *HTTPClient) do(ctx context.Context, op, method, path string, body, out any) error {
	base, err := h.gateway(ctx)
	if err != nil {
		return err
	}

	var payload []byte

	if body != nil {
		if payload, err = json.Marshal(body); err != nil {
			return &RequestError{Op: op, Err: err}
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, base+path, bytes.NewReader(payload))
	if err != nil {
		return &RequestError{Op: op, Err: err}
	}

	req.SetBasicAuth(h.email, h.token)
	req.Header.Set("Accept", "application/json")

	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return h.decode(req, op, out)
}

// decode sends req, retrying a 429, and decodes a 2xx body into out.
func (h *HTTPClient) decode(req *http.Request, op string, out any) error {
	backoff := 2 * time.Second

	for attempt := 0; ; attempt++ {
		if attempt > 0 && req.GetBody != nil {
			b, err := req.GetBody()
			if err != nil {
				return &RequestError{Op: op, Err: err}
			}

			req.Body = b
		}

		resp, err := h.http.Do(req)
		if err != nil {
			if ctxErr := req.Context().Err(); ctxErr != nil {
				return ctxErr
			}

			return &RequestError{Op: op, Err: err}
		}

		fireRequest(req.Context(), req.Method, req.URL.Path, resp.StatusCode)

		if resp.StatusCode == http.StatusTooManyRequests && attempt < maxRetries {
			wait := retryAfter(resp.Header.Get("Retry-After"), backoff)
			drain(resp)
			backoff *= 2

			if err := h.sleep(req.Context(), wait); err != nil {
				return err
			}

			continue
		}

		return readResponse(resp, op, out)
	}
}

// readResponse turns a response into out or a typed error.
func readResponse(resp *http.Response, op string, out any) error {
	defer drain(resp)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		if out == nil || resp.StatusCode == http.StatusNoContent {
			return nil
		}

		if err := json.NewDecoder(resp.Body).Decode(out); err != nil {
			return &RequestError{Op: op, Status: resp.StatusCode, Err: fmt.Errorf("decoding response: %w", err)}
		}

		return nil
	}

	// A failed read leaves whatever arrived; the status is the error.
	snippet, err := io.ReadAll(io.LimitReader(resp.Body, maxErrorBody))
	if err != nil && len(snippet) == 0 {
		snippet = []byte(http.StatusText(resp.StatusCode))
	}

	text := strings.TrimSpace(string(snippet))

	switch resp.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return &AuthError{Op: op, Status: resp.StatusCode, Body: text}
	case http.StatusConflict:
		return &ConflictError{}
	default:
		return &RequestError{Op: op, Status: resp.StatusCode, Body: text}
	}
}

// drain discards and closes a response body so the connection is reused.
func drain(resp *http.Response) {
	//nolint:errcheck,gosec // best effort: draining only lets the connection be reused
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
	//nolint:gosec // nothing to do with a close error on a read body
	resp.Body.Close()
}

// retryAfter reads a Retry-After header as seconds or an HTTP date, falling
// back to fallback when it is absent or unreadable.
func retryAfter(header string, fallback time.Duration) time.Duration {
	if header == "" {
		return fallback
	}

	if secs, err := strconv.Atoi(strings.TrimSpace(header)); err == nil && secs >= 0 {
		return time.Duration(secs) * time.Second
	}

	if at, err := http.ParseTime(header); err == nil {
		if d := time.Until(at); d > 0 {
			return d
		}

		return 0
	}

	return fallback
}

// The v2 wire shapes.
type (
	apiVersion struct {
		Number  int    `json:"number"`
		Message string `json:"message,omitempty"`
	}

	apiBody struct {
		Representation string `json:"representation"`
		Value          string `json:"value"`
	}

	apiPage struct {
		ID       string     `json:"id"`
		Title    string     `json:"title"`
		ParentID string     `json:"parentId"`
		SpaceID  string     `json:"spaceId"`
		Version  apiVersion `json:"version"`
		Body     struct {
			Storage struct {
				Value string `json:"value"`
			} `json:"storage"`
		} `json:"body"`
		Links struct {
			WebUI string `json:"webui"`
		} `json:"_links"`
	}

	apiPageWrite struct {
		ID       string      `json:"id,omitempty"`
		SpaceID  string      `json:"spaceId,omitempty"`
		Status   string      `json:"status"`
		Title    string      `json:"title"`
		ParentID string      `json:"parentId,omitempty"`
		Body     apiBody     `json:"body"`
		Version  *apiVersion `json:"version,omitempty"`
	}

	apiProperty struct {
		ID      string          `json:"id,omitempty"`
		Key     string          `json:"key"`
		Value   json.RawMessage `json:"value"`
		Version *apiVersion     `json:"version,omitempty"`
	}

	apiList[T any] struct {
		Results []T `json:"results"`
		Links   struct {
			Next string `json:"next"`
		} `json:"_links"`
	}
)

// page converts the wire page.
func (h *HTTPClient) page(p *apiPage) *Page {
	out := &Page{ID: p.ID, Title: p.Title, ParentID: p.ParentID, SpaceID: p.SpaceID, Version: p.Version.Number}
	if p.Links.WebUI != "" {
		out.WebURL = h.site + "/wiki" + p.Links.WebUI
	}

	return out
}

// SpaceID implements Client.
func (h *HTTPClient) SpaceID(ctx context.Context, key string) (string, error) {
	var list apiList[struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}]

	if err := h.do(ctx, "get space", http.MethodGet, "/wiki/api/v2/spaces?keys="+url.QueryEscape(key), nil, &list); err != nil {
		return "", err
	}

	for _, s := range list.Results {
		if s.Key == key {
			return s.ID, nil
		}
	}

	return "", &RequestError{Op: "get space", Status: http.StatusNotFound, Body: fmt.Sprintf("no space with key %q", key)}
}

// FindPage implements Client.
func (h *HTTPClient) FindPage(ctx context.Context, spaceID, title string) (*Page, error) {
	q := url.Values{"space-id": {spaceID}, "title": {title}, "status": {statusCurrent}}

	var list apiList[apiPage]
	if err := h.do(ctx, "find page", http.MethodGet, "/wiki/api/v2/pages?"+q.Encode(), nil, &list); err != nil {
		return nil, err
	}

	for i := range list.Results {
		if list.Results[i].Title == title {
			return h.page(&list.Results[i]), nil
		}
	}

	return nil, nil //nolint:nilnil // a missing page is an answer, not an error (Client contract)
}

// CreatePage implements Client.
func (h *HTTPClient) CreatePage(ctx context.Context, p *NewPage) (*Page, error) {
	req := apiPageWrite{
		SpaceID:  p.SpaceID,
		Status:   statusCurrent,
		Title:    p.Title,
		ParentID: p.ParentID,
		Body:     apiBody{Representation: "storage", Value: string(p.Body)},
	}

	var out apiPage
	if err := h.do(ctx, "create page", http.MethodPost, "/wiki/api/v2/pages", req, &out); err != nil {
		return nil, err
	}

	return h.page(&out), nil
}

// UpdatePage implements Client.
func (h *HTTPClient) UpdatePage(ctx context.Context, id string, p *PageUpdate) (*Page, error) {
	if p.Body == nil {
		var err error
		if p, err = h.currentBody(ctx, id, p); err != nil {
			return nil, err
		}
	}

	req := apiPageWrite{
		ID:       id,
		Status:   statusCurrent,
		Title:    p.Title,
		ParentID: p.ParentID,
		Body:     apiBody{Representation: "storage", Value: string(p.Body)},
		Version:  &apiVersion{Number: p.Version, Message: p.Message},
	}

	var out apiPage

	err := h.do(ctx, "update page", http.MethodPut, "/wiki/api/v2/pages/"+url.PathEscape(id), req, &out)

	var conflict *ConflictError
	if errors.As(err, &conflict) {
		conflict.Title, conflict.Want = p.Title, p.Version-1
	}

	if err != nil {
		return nil, err
	}

	return h.page(&out), nil
}

// currentBody fills a move's body, version, and title from the page as it
// is, since the v2 update needs all three.
func (h *HTTPClient) currentBody(ctx context.Context, id string, p *PageUpdate) (*PageUpdate, error) {
	var cur apiPage
	if err := h.do(ctx, "get page", http.MethodGet, "/wiki/api/v2/pages/"+url.PathEscape(id)+"?body-format=storage", nil, &cur); err != nil {
		return nil, err
	}

	out := *p
	out.Body = []byte(cur.Body.Storage.Value)

	if out.Version == 0 {
		out.Version = cur.Version.Number + 1
	}

	if out.Title == "" {
		out.Title = cur.Title
	}

	return &out, nil
}

// Property implements Client.
func (h *HTTPClient) Property(ctx context.Context, pageID, key string) (*Property, error) {
	path := "/wiki/api/v2/pages/" + url.PathEscape(pageID) + "/properties?key=" + url.QueryEscape(key)

	var list apiList[apiProperty]
	if err := h.do(ctx, "get property", http.MethodGet, path, nil, &list); err != nil {
		return nil, err
	}

	for _, p := range list.Results {
		if p.Key == key {
			out := &Property{ID: p.ID, Key: p.Key, Value: p.Value}
			if p.Version != nil {
				out.Version = p.Version.Number
			}

			return out, nil
		}
	}

	return nil, nil //nolint:nilnil // a missing property is an answer, not an error (Client contract)
}

// SetProperty implements Client.
func (h *HTTPClient) SetProperty(ctx context.Context, pageID string, p *Property) error {
	path := "/wiki/api/v2/pages/" + url.PathEscape(pageID) + "/properties"

	if p.ID == "" {
		return h.do(ctx, "create property", http.MethodPost, path, apiProperty{Key: p.Key, Value: p.Value}, nil)
	}

	body := apiProperty{Key: p.Key, Value: p.Value, Version: &apiVersion{Number: p.Version + 1}}

	return h.do(ctx, "update property", http.MethodPut, path+"/"+url.PathEscape(p.ID), body, nil)
}

// Children implements Client.
func (h *HTTPClient) Children(ctx context.Context, parentID string) ([]Page, error) {
	var out []Page

	next := "/wiki/api/v2/pages/" + url.PathEscape(parentID) + "/children?limit=250"

	for next != "" {
		var list apiList[apiPage]
		if err := h.do(ctx, "list children", http.MethodGet, next, nil, &list); err != nil {
			return nil, err
		}

		for i := range list.Results {
			p := h.page(&list.Results[i])
			p.ParentID = parentID
			out = append(out, *p)
		}

		// The cursor link is relative to the gateway base, like every path.
		next = list.Links.Next
	}

	return out, nil
}

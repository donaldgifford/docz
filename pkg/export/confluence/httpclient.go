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

// statusArchived is a page a person archived, which still holds its title.
const statusArchived = "archived"

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
		Type     string     `json:"type"`
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

	apiFolderWrite struct {
		SpaceID  string `json:"spaceId"`
		Title    string `json:"title"`
		ParentID string `json:"parentId,omitempty"`
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

// Spaces looks up several space keys in one request and returns the id of
// each one found; a key missing from the map does not exist or is not
// visible to the credential. It is not part of Client: docz-api's startup
// check is its one caller.
func (h *HTTPClient) Spaces(ctx context.Context, keys []string) (map[string]string, error) {
	var list apiList[struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}]

	q := url.Values{"keys": {strings.Join(keys, ",")}, "limit": {"250"}}
	if err := h.do(ctx, "get spaces", http.MethodGet, "/wiki/api/v2/spaces?"+q.Encode(), nil, &list); err != nil {
		return nil, err
	}

	out := make(map[string]string, len(list.Results))
	for _, s := range list.Results {
		out[s.Key] = s.ID
	}

	return out, nil
}

// SpaceHome implements Client.
func (h *HTTPClient) SpaceHome(ctx context.Context, spaceID string) (string, error) {
	var space struct {
		HomepageID string `json:"homepageId"`
	}

	if err := h.do(ctx, "get space", http.MethodGet, "/wiki/api/v2/spaces/"+url.PathEscape(spaceID), nil, &space); err != nil {
		return "", err
	}

	return space.HomepageID, nil
}

// FindPage implements Client.
func (h *HTTPClient) FindPage(ctx context.Context, spaceID, title string) (*Page, error) {
	return h.findPage(ctx, spaceID, title, statusCurrent)
}

// findPage returns the page titled title in the space with the given
// status, or nil.
func (h *HTTPClient) findPage(ctx context.Context, spaceID, title, status string) (*Page, error) {
	q := url.Values{"space-id": {spaceID}, "title": {title}, "status": {status}}

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

// Page implements Client.
func (h *HTTPClient) Page(ctx context.Context, id string) (*Page, error) {
	var out apiPage

	err := h.do(ctx, "get page", http.MethodGet, "/wiki/api/v2/pages/"+url.PathEscape(id), nil, &out)
	if notFound(err) {
		return nil, nil //nolint:nilnil // a missing page is an answer, not an error (Client contract)
	}

	if err != nil {
		return nil, err
	}

	return h.page(&out), nil
}

// Body implements Client.
func (h *HTTPClient) Body(ctx context.Context, id string) ([]byte, error) {
	var out apiPage

	err := h.do(ctx, "get page body", http.MethodGet, "/wiki/api/v2/pages/"+url.PathEscape(id)+"?body-format=storage", nil, &out)
	if notFound(err) {
		return nil, nil
	}

	if err != nil {
		return nil, err
	}

	return []byte(out.Body.Storage.Value), nil
}

// Folder implements Client.
func (h *HTTPClient) Folder(ctx context.Context, id string) (*Folder, error) {
	var out apiPage

	err := h.do(ctx, "get folder", http.MethodGet, "/wiki/api/v2/folders/"+url.PathEscape(id), nil, &out)
	if notFound(err) {
		return nil, nil //nolint:nilnil // a missing folder is an answer, not an error (Client contract)
	}

	if err != nil {
		return nil, err
	}

	return h.folder(&out), nil
}

// CreateFolder implements Client.
func (h *HTTPClient) CreateFolder(ctx context.Context, f *NewFolder) (*Folder, error) {
	req := apiFolderWrite{SpaceID: f.SpaceID, Title: f.Title, ParentID: f.ParentID}

	var out apiPage

	err := h.do(ctx, "create folder", http.MethodPost, "/wiki/api/v2/folders", req, &out)
	if titleTaken(err) {
		return nil, &TitleError{Title: f.Title}
	}

	if err != nil {
		return nil, err
	}

	return h.folder(&out), nil
}

// folder converts the wire folder, which has a page's shape.
func (h *HTTPClient) folder(p *apiPage) *Folder {
	pg := h.page(p)

	return &Folder{ID: pg.ID, Title: pg.Title, ParentID: pg.ParentID, SpaceID: pg.SpaceID, WebURL: pg.WebURL}
}

// notFound reports a 404.
func notFound(err error) bool {
	var re *RequestError

	return errors.As(err, &re) && re.Status == http.StatusNotFound
}

// titleTaken reports Confluence's 400 for a title already in the space:
// "A page already exists with the same TITLE in this space", and "A folder
// exists with the same title in this space" (INV-0020 Observation 9).
func titleTaken(err error) bool {
	var re *RequestError

	return errors.As(err, &re) && re.Status == http.StatusBadRequest &&
		strings.Contains(strings.ToLower(re.Body), "same title")
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

	err := h.do(ctx, "create page", http.MethodPost, "/wiki/api/v2/pages", req, &out)
	if titleTaken(err) {
		return nil, h.titleError(ctx, p.SpaceID, p.Title)
	}

	if err != nil {
		return nil, err
	}

	return h.page(&out), nil
}

// titleError names the archived page holding a title when there is one: a
// current holder is one FindPage would have found, so an archived one is
// the likely cause, and naming it makes the fix obvious.
func (h *HTTPClient) titleError(ctx context.Context, spaceID, title string) error {
	te := &TitleError{Title: title}

	if archived, err := h.findPage(ctx, spaceID, title, statusArchived); err == nil && archived != nil {
		te.ArchivedID = archived.ID
	}

	return te
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

// propertiesPath is the content-properties collection of a page or folder.
func propertiesPath(t Target) string {
	kind := "pages"
	if t.Type == TypeFolder {
		kind = "folders"
	}

	return "/wiki/api/v2/" + kind + "/" + url.PathEscape(t.ID) + "/properties"
}

// Property implements Client.
func (h *HTTPClient) Property(ctx context.Context, t Target, key string) (*Property, error) {
	var list apiList[apiProperty]
	if err := h.do(ctx, "get property", http.MethodGet, propertiesPath(t)+"?key="+url.QueryEscape(key), nil, &list); err != nil {
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
func (h *HTTPClient) SetProperty(ctx context.Context, t Target, p *Property) error {
	path := propertiesPath(t)

	if p.ID == "" {
		return h.do(ctx, "create property", http.MethodPost, path, apiProperty{Key: p.Key, Value: p.Value}, nil)
	}

	body := apiProperty{Key: p.Key, Value: p.Value, Version: &apiVersion{Number: p.Version + 1}}

	return h.do(ctx, "update property", http.MethodPut, path+"/"+url.PathEscape(p.ID), body, nil)
}

// Children implements Client, through direct-children, which lists folders
// as well as pages and works under either.
func (h *HTTPClient) Children(ctx context.Context, parent Target) ([]Node, error) {
	kind := "pages"
	if parent.Type == TypeFolder {
		kind = "folders"
	}

	var out []Node

	next := "/wiki/api/v2/" + kind + "/" + url.PathEscape(parent.ID) + "/direct-children?limit=250"

	for next != "" {
		var list apiList[apiPage]

		err := h.do(ctx, "list children", http.MethodGet, next, nil, &list)
		if notFound(err) {
			return nil, nil
		}

		if err != nil {
			return nil, err
		}

		for i := range list.Results {
			p := h.page(&list.Results[i])
			p.ParentID = parent.ID
			out = append(out, Node{Page: *p, Type: list.Results[i].Type})
		}

		// The cursor link is relative to the gateway base, like every path.
		next = list.Links.Next
	}

	return out, nil
}

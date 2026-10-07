package confluencetest

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/export/confluence"
)

// CloudID is the cloud id Handler's tenant_info reports.
const CloudID = "confluencetest-cloud"

// The v2 wire shapes Handler reads and writes: the subset HTTPClient uses.
type (
	wireVersion struct {
		Number  int    `json:"number"`
		Message string `json:"message,omitempty"`
	}

	wireBody struct {
		Value string `json:"value"`
	}

	wireStorage struct {
		Storage wireBody `json:"storage"`
	}

	wireLinks struct {
		WebUI string `json:"webui"`
	}

	wirePage struct {
		ID       string       `json:"id"`
		Type     string       `json:"type"`
		Status   string       `json:"status"`
		Title    string       `json:"title"`
		ParentID string       `json:"parentId"`
		SpaceID  string       `json:"spaceId"`
		Version  wireVersion  `json:"version"`
		Body     *wireStorage `json:"body,omitempty"`
		Links    wireLinks    `json:"_links"`
	}

	wireWrite struct {
		SpaceID  string       `json:"spaceId"`
		Title    string       `json:"title"`
		ParentID string       `json:"parentId"`
		Body     wireBody     `json:"body"`
		Version  *wireVersion `json:"version"`
	}

	wireProperty struct {
		ID      string          `json:"id,omitempty"`
		Key     string          `json:"key"`
		Value   json.RawMessage `json:"value"`
		Version *wireVersion    `json:"version,omitempty"`
	}
)

// Handler serves the site over the endpoints confluence.HTTPClient calls:
// <site>/_edge/tenant_info, and under /ex/confluence/<CloudID>/wiki/api/v2
// the spaces, pages, folders, properties, and direct-children endpoints.
// Errors come back with Confluence's statuses and, for a taken title, its
// wording, so HTTPClient's typed errors are exercised as they are live.
func (s *Site) Handler() http.Handler {
	return http.HandlerFunc(s.serve)
}

// HTTPClient starts an httptest server over Handler, closed when the test
// ends, and returns a confluence.HTTPClient whose every request, the
// gateway's included, goes to it. The client reports pages under s.URL.
func (s *Site) HTTPClient(tb testing.TB, email, token string) *confluence.HTTPClient {
	tb.Helper()

	srv := httptest.NewServer(s.Handler())
	tb.Cleanup(srv.Close)

	hc := &http.Client{Transport: rewrite{srv: srv}}

	return confluence.NewHTTPClient(s.URL, email, token, confluence.WithHTTPClient(hc))
}

// rewrite sends every request to srv, whatever host it names.
type rewrite struct{ srv *httptest.Server }

func (r rewrite) RoundTrip(req *http.Request) (*http.Response, error) {
	out := req.Clone(req.Context())
	out.URL.Scheme = "http"
	out.URL.Host = strings.TrimPrefix(r.srv.URL, "http://")
	out.Host = out.URL.Host

	return r.srv.Client().Transport.RoundTrip(out)
}

// The two content collections.
const (
	pages   = "pages"
	folders = "folders"
)

const apiPrefix = "/ex/confluence/" + CloudID + "/wiki/api/v2/"

func (s *Site) serve(w http.ResponseWriter, r *http.Request) {
	if r.URL.Path == "/_edge/tenant_info" {
		writeJSON(w, map[string]string{"cloudId": CloudID})

		return
	}

	if !s.authorized(r) {
		http.Error(w, "unauthorized", http.StatusUnauthorized)

		return
	}

	rest, ok := strings.CutPrefix(r.URL.Path, apiPrefix)
	if !ok {
		http.NotFound(w, r)

		return
	}

	seg := strings.Split(rest, "/")
	content := seg[0] == pages || seg[0] == folders

	switch {
	case seg[0] == "spaces":
		s.serveSpaces(w, r, seg[1:])
	case content && len(seg) >= 3 && seg[2] == "properties":
		s.serveProperties(w, r, seg)
	case content && len(seg) == 3 && seg[2] == "direct-children":
		s.serveChildren(w, r, seg)
	case seg[0] == pages:
		s.servePages(w, r, seg[1:])
	case seg[0] == folders:
		s.serveFolders(w, r, seg[1:])
	default:
		http.NotFound(w, r)
	}
}

func (s *Site) authorized(r *http.Request) bool {
	s.mu.Lock()
	email, token := s.email, s.token
	s.mu.Unlock()

	if token == "" {
		return true
	}

	u, p, ok := r.BasicAuth()

	return ok && u == email && p == token
}

func (s *Site) serveSpaces(w http.ResponseWriter, r *http.Request, seg []string) {
	if len(seg) == 1 && seg[0] != "" {
		writeJSON(w, map[string]string{"id": seg[0], "homepageId": HomeID(seg[0])})

		return
	}

	type space struct {
		ID  string `json:"id"`
		Key string `json:"key"`
	}

	var out []space

	s.mu.Lock()
	for _, k := range strings.Split(r.URL.Query().Get("keys"), ",") {
		if id, ok := s.spaces[k]; ok {
			out = append(out, space{ID: id, Key: k})
		}
	}
	s.mu.Unlock()

	writeList(w, out)
}

func (s *Site) servePages(w http.ResponseWriter, r *http.Request, seg []string) {
	ctx := r.Context()
	collection := len(seg) == 0 || seg[0] == ""

	switch {
	case r.Method == http.MethodGet && collection:
		q := r.URL.Query()

		var out []wirePage
		if p := s.findPage(q.Get("space-id"), q.Get("title"), q.Get("status")); p != nil {
			wp, _ := s.wire(p.ID, "", false)
			out = append(out, wp)
		}

		writeList(w, out)
	case r.Method == http.MethodPost && collection:
		var in wireWrite
		if !readJSON(w, r, &in) {
			return
		}

		p, err := s.CreatePage(ctx, &confluence.NewPage{
			SpaceID: in.SpaceID, ParentID: in.ParentID, Title: in.Title, Body: []byte(in.Body.Value),
		})
		s.reply(w, p, err)
	case r.Method == http.MethodGet:
		wp, ok := s.wire(seg[0], confluence.TypePage, r.URL.Query().Get("body-format") == "storage")
		if !ok {
			http.NotFound(w, r)

			return
		}

		writeJSON(w, wp)
	case r.Method == http.MethodPut:
		var in wireWrite
		if !readJSON(w, r, &in) {
			return
		}

		u := &confluence.PageUpdate{ParentID: in.ParentID, Title: in.Title, Body: []byte(in.Body.Value)}
		if in.Version != nil {
			u.Version, u.Message = in.Version.Number, in.Version.Message
		}

		p, err := s.UpdatePage(ctx, seg[0], u)
		s.reply(w, p, err)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Site) serveFolders(w http.ResponseWriter, r *http.Request, seg []string) {
	collection := len(seg) == 0 || seg[0] == ""

	switch {
	case r.Method == http.MethodPost && collection:
		var in wireWrite
		if !readJSON(w, r, &in) {
			return
		}

		f, err := s.CreateFolder(r.Context(), &confluence.NewFolder{
			SpaceID: in.SpaceID, ParentID: in.ParentID, Title: in.Title,
		})
		if err != nil {
			writeError(w, err)

			return
		}

		wp, _ := s.wire(f.ID, "", false)
		writeJSON(w, wp)
	case r.Method == http.MethodGet && !collection:
		wp, ok := s.wire(seg[0], confluence.TypeFolder, false)
		if !ok {
			http.NotFound(w, r)

			return
		}

		writeJSON(w, wp)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Site) serveProperties(w http.ResponseWriter, r *http.Request, seg []string) {
	t := confluence.Target{ID: seg[1], Type: confluence.TypePage}
	if seg[0] == folders {
		t.Type = confluence.TypeFolder
	}

	ctx := r.Context()

	switch r.Method {
	case http.MethodGet:
		p, err := s.Property(ctx, t, r.URL.Query().Get("key"))
		if err != nil {
			writeError(w, err)

			return
		}

		var out []wireProperty
		if p != nil {
			out = append(out, wireProperty{ID: p.ID, Key: p.Key, Value: p.Value, Version: &wireVersion{Number: p.Version}})
		}

		writeList(w, out)
	case http.MethodPost, http.MethodPut:
		var in wireProperty
		if !readJSON(w, r, &in) {
			return
		}

		p := &confluence.Property{Key: in.Key, Value: in.Value}
		if r.Method == http.MethodPut && len(seg) == 4 {
			p.ID = seg[3]
			if in.Version != nil {
				p.Version = in.Version.Number - 1
			}
		}

		if err := s.SetProperty(ctx, t, p); err != nil {
			writeError(w, err)

			return
		}

		writeJSON(w, in)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (s *Site) serveChildren(w http.ResponseWriter, r *http.Request, seg []string) {
	kind := confluence.TypePage
	if seg[0] == folders {
		kind = confluence.TypeFolder
	}

	_, ok := s.wire(seg[1], kind, false)
	if !ok && !strings.HasPrefix(seg[1], "home-") {
		http.NotFound(w, r)

		return
	}

	nodes, err := s.Children(r.Context(), confluence.Target{ID: seg[1], Type: kind})
	if err != nil {
		writeError(w, err)

		return
	}

	out := make([]wirePage, 0, len(nodes))
	for i := range nodes {
		wp, _ := s.wire(nodes[i].ID, "", false)
		wp.Version = wireVersion{}
		out = append(out, wp)
	}

	writeList(w, out)
}

// wire renders the current node with id, with its body when withBody,
// reporting false when there is no current node of kind (any kind when
// kind is empty).
func (s *Site) wire(id, kind string, withBody bool) (wirePage, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, ok := s.nodes[id]
	if !ok || (kind != "" && n.kind != kind) || n.status != statusCurrent {
		return wirePage{}, false
	}

	wp := wirePage{
		ID: n.ID, Type: n.kind, Status: n.status, Title: n.Title, ParentID: n.ParentID,
		SpaceID: n.SpaceID, Version: wireVersion{Number: n.Version},
		Links: wireLinks{WebUI: webUI(n.kind, n.SpaceID, n.ID)},
	}

	if withBody {
		wp.Body = &wireStorage{Storage: wireBody{Value: string(n.body)}}
	}

	return wp, true
}

// reply writes a created or updated page, or the error.
func (s *Site) reply(w http.ResponseWriter, p *confluence.Page, err error) {
	if err != nil {
		writeError(w, err)

		return
	}

	wp, _ := s.wire(p.ID, "", false)
	writeJSON(w, wp)
}

// writeError maps a Client error to the status and body Confluence answers
// with.
func writeError(w http.ResponseWriter, err error) {
	var (
		te   *confluence.TitleError
		ce   *confluence.ConflictError
		ae   *confluence.AuthError
		re   *confluence.RequestError
		body = err.Error()
		code = http.StatusInternalServerError
	)

	switch {
	case errors.As(err, &te):
		code, body = http.StatusBadRequest, "A page already exists with the same TITLE in this space"
	case errors.As(err, &ce):
		code = http.StatusConflict
	case errors.As(err, &ae):
		code = ae.Status
	case errors.As(err, &re) && re.Status != 0:
		code, body = re.Status, re.Body
	}

	http.Error(w, body, code)
}

func readJSON(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(r.Body).Decode(v); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)

		return false
	}

	return true
}

func writeList[T any](w http.ResponseWriter, results []T) {
	if results == nil {
		results = []T{}
	}

	writeJSON(w, map[string]any{"results": results, "_links": map[string]string{}})
}

// writeJSON writes v as a 200. A failed write surfaces in the client.
func writeJSON(w http.ResponseWriter, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)

		return
	}

	w.Header().Set("Content-Type", "application/json")
	//nolint:errcheck,gosec // a test server's failed write surfaces in the client
	w.Write(b)
}

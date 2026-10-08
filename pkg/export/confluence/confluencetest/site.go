// Package confluencetest is an in-memory Confluence Cloud site for tests of
// code built on pkg/export/confluence (IMPL-0024 Open Question 4).
//
// A Site keeps pages and folders with their titles, parents, versions,
// bodies, statuses, and content properties, and enforces the rules Export
// depends on: page titles are unique in a space and so are folder titles,
// the two do not collide, and an archived page keeps its title. It is a
// confluence.Client itself, and Handler serves the same state over the v2
// REST endpoints HTTPClient calls, so one fake covers both the library's
// callers and code that drives the real HTTP client.
//
// EXPERIMENTAL: like its parent, this package may change between
// v2.0.0-beta.N tags (ADR-0002 Decision 7).
package confluencetest

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"sync"

	"github.com/donaldgifford/docz/v2/pkg/export/confluence"
)

// DefaultURL is the site URL a Site reports pages under unless URL is set.
const DefaultURL = "https://example.atlassian.net"

const (
	statusCurrent  = "current"
	statusArchived = "archived"
)

// node is one page or folder.
type node struct {
	confluence.Page

	kind   string // confluence.TypePage or confluence.TypeFolder
	status string
	body   []byte
	props  map[string]*confluence.Property
}

// Site is an in-memory Confluence site. The zero value is not usable; call
// New. A Site is safe for concurrent use.
type Site struct {
	// URL is the site's base URL, used for pages' web URLs.
	URL string

	mu     sync.Mutex
	spaces map[string]string // key -> id
	nodes  map[string]*node
	nextID int
	writes []string
	fail   map[string]int
	email  string
	token  string
}

var _ confluence.Client = (*Site)(nil)

// New returns a Site holding the given spaces. Each space's id is
// "space-<KEY>" and its homepage id "home-<space id>"; content filed with no
// parent lists under the homepage, as Confluence's does.
func New(spaceKeys ...string) *Site {
	s := &Site{
		URL:    DefaultURL,
		spaces: make(map[string]string, len(spaceKeys)),
		nodes:  make(map[string]*node),
		fail:   make(map[string]int),
	}

	for _, k := range spaceKeys {
		s.spaces[k] = SpaceID(k)
	}

	return s
}

// SpaceID is the id New gives the space with key.
func SpaceID(key string) string { return "space-" + key }

// HomeID is the homepage id of a space.
func HomeID(spaceID string) string { return "home-" + spaceID }

// RequireAuth makes Handler answer 401 to a request whose basic auth is not
// email and token. The in-memory Client methods are not checked.
func (s *Site) RequireAuth(email, token string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.email, s.token = email, token
}

// FailOn makes the operation op on the page or folder titled title fail
// with status until Clear is called. op is "create", "update", or
// "property". A 401 or 403 is a *confluence.AuthError, anything else a
// *confluence.RequestError.
func (s *Site) FailOn(op, title string, status int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.fail[op+" "+title] = status
}

// Clear removes every failure FailOn set.
func (s *Site) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()

	clear(s.fail)
}

// failing returns the injected error for op on title. The lock is held.
func (s *Site) failing(op, title string) error {
	status, ok := s.fail[op+" "+title]
	if !ok {
		return nil
	}

	if status == 401 || status == 403 {
		return &confluence.AuthError{Op: op, Status: status, Body: "injected"}
	}

	return &confluence.RequestError{Op: op, Status: status, Body: "injected"}
}

// Writes returns every write since the last call, in order, and clears the
// log: "create <title>", "create folder <title>", "update <title> v<n>",
// and "property <title>".
func (s *Site) Writes() []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := s.writes
	s.writes = nil

	return out
}

// PageByTitle returns the current page titled title in any space, or nil.
func (s *Site) PageByTitle(title string) *confluence.Page {
	return s.byTitle(confluence.TypePage, title)
}

// FolderByTitle returns the folder titled title in any space, or nil.
func (s *Site) FolderByTitle(title string) *confluence.Page {
	return s.byTitle(confluence.TypeFolder, title)
}

func (s *Site) byTitle(kind, title string) *confluence.Page {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, n := range s.nodes {
		if n.kind == kind && n.Title == title && n.status == statusCurrent {
			cp := n.Page

			return &cp
		}
	}

	return nil
}

// Pages returns every current page and folder, ordered by id.
func (s *Site) Pages() []confluence.Node {
	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]confluence.Node, 0, len(s.nodes))

	for _, id := range slices.SortedFunc(maps.Keys(s.nodes), compareIDs) {
		if n := s.nodes[id]; n.status == statusCurrent {
			out = append(out, confluence.Node{Page: n.Page, Type: n.kind})
		}
	}

	return out
}

// Edit bumps the version of the page titled title, as an edit in Confluence
// does.
func (s *Site) Edit(title string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, n := range s.nodes {
		if n.kind == confluence.TypePage && n.Title == title {
			n.Version++
		}
	}
}

// Archive archives the page with id. It keeps its title.
func (s *Site) Archive(id string) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if n, ok := s.nodes[id]; ok {
		n.status = statusArchived
	}
}

// add stores a new node. The lock is held.
func (s *Site) add(kind, spaceID, parentID, title string, body []byte) *node {
	s.nextID++
	id := strconv.Itoa(100 + s.nextID)
	n := &node{
		Page: confluence.Page{
			ID: id, Title: title, ParentID: parentID, SpaceID: spaceID, Version: 1,
			WebURL: s.URL + "/wiki" + webUI(kind, spaceID, id),
		},
		kind:   kind,
		status: statusCurrent,
		body:   body,
		props:  make(map[string]*confluence.Property),
	}
	s.nodes[id] = n

	return n
}

// webUI is the path a node's _links.webui carries.
func webUI(kind, spaceID, id string) string {
	return "/spaces/" + spaceID + "/" + kind + "s/" + id
}

// holder returns the node of kind holding title in the space, archived
// included, or nil. The lock is held.
func (s *Site) holder(kind, spaceID, title string) *node {
	for _, n := range s.nodes {
		if n.kind == kind && n.SpaceID == spaceID && n.Title == title {
			return n
		}
	}

	return nil
}

// SpaceID implements confluence.Client.
func (s *Site) SpaceID(_ context.Context, key string) (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if id, ok := s.spaces[key]; ok {
		return id, nil
	}

	return "", &confluence.RequestError{Op: "get space", Status: 404, Body: fmt.Sprintf("no space with key %q", key)}
}

// SpaceHome implements confluence.Client.
func (*Site) SpaceHome(_ context.Context, spaceID string) (string, error) {
	return HomeID(spaceID), nil
}

// FindPage implements confluence.Client.
func (s *Site) FindPage(_ context.Context, spaceID, title string) (*confluence.Page, error) {
	return s.findPage(spaceID, title, statusCurrent), nil
}

func (s *Site) findPage(spaceID, title, status string) *confluence.Page {
	s.mu.Lock()
	defer s.mu.Unlock()

	if n := s.holder(confluence.TypePage, spaceID, title); n != nil && n.status == status {
		cp := n.Page

		return &cp
	}

	return nil
}

// Page implements confluence.Client.
func (s *Site) Page(_ context.Context, id string) (*confluence.Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if n, ok := s.nodes[id]; ok && n.kind == confluence.TypePage && n.status == statusCurrent {
		cp := n.Page

		return &cp, nil
	}

	return nil, nil //nolint:nilnil // a missing page is an answer (Client contract)
}

// Body implements confluence.Client.
func (s *Site) Body(_ context.Context, id string) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if n, ok := s.nodes[id]; ok && n.kind == confluence.TypePage {
		return slices.Clone(n.body), nil
	}

	return nil, nil
}

// Folder implements confluence.Client.
func (s *Site) Folder(_ context.Context, id string) (*confluence.Folder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if n, ok := s.nodes[id]; ok && n.kind == confluence.TypeFolder {
		return folderOf(n), nil
	}

	return nil, nil //nolint:nilnil // a missing folder is an answer (Client contract)
}

func folderOf(n *node) *confluence.Folder {
	return &confluence.Folder{ID: n.ID, Title: n.Title, ParentID: n.ParentID, SpaceID: n.SpaceID, WebURL: n.WebURL}
}

// CreatePage implements confluence.Client.
func (s *Site) CreatePage(_ context.Context, p *confluence.NewPage) (*confluence.Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.failing("create", p.Title); err != nil {
		return nil, err
	}

	if h := s.holder(confluence.TypePage, p.SpaceID, p.Title); h != nil {
		te := &confluence.TitleError{Title: p.Title}
		if h.status == statusArchived {
			te.ArchivedID = h.ID
		}

		return nil, te
	}

	parentID := p.ParentID
	if parentID == "" {
		parentID = HomeID(p.SpaceID)
	}

	n := s.add(confluence.TypePage, p.SpaceID, parentID, p.Title, slices.Clone(p.Body))
	s.writes = append(s.writes, "create "+p.Title)
	cp := n.Page

	return &cp, nil
}

// CreateFolder implements confluence.Client.
func (s *Site) CreateFolder(_ context.Context, nf *confluence.NewFolder) (*confluence.Folder, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.failing("create", nf.Title); err != nil {
		return nil, err
	}

	if s.holder(confluence.TypeFolder, nf.SpaceID, nf.Title) != nil {
		return nil, &confluence.TitleError{Title: nf.Title}
	}

	parentID := nf.ParentID
	if parentID == "" {
		parentID = HomeID(nf.SpaceID)
	}

	n := s.add(confluence.TypeFolder, nf.SpaceID, parentID, nf.Title, nil)
	n.Version = 0
	s.writes = append(s.writes, "create folder "+nf.Title)

	return folderOf(n), nil
}

// UpdatePage implements confluence.Client.
func (s *Site) UpdatePage(_ context.Context, id string, p *confluence.PageUpdate) (*confluence.Page, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	n, ok := s.nodes[id]
	if !ok || n.kind != confluence.TypePage {
		return nil, &confluence.RequestError{Op: "update page", Status: 404}
	}

	if err := s.failing("update", n.Title); err != nil {
		return nil, err
	}

	want := p.Version
	if p.Body == nil && want == 0 {
		want = n.Version + 1
	}

	if want != n.Version+1 {
		return nil, &confluence.ConflictError{Title: n.Title, Want: want - 1}
	}

	if p.Title != "" && p.Title != n.Title && s.holder(confluence.TypePage, n.SpaceID, p.Title) != nil {
		return nil, &confluence.RequestError{
			Op: "update page", Status: 400, Body: "A page already exists with the same TITLE in this space",
		}
	}

	n.Version = want
	n.status = statusCurrent

	if p.ParentID != "" {
		n.ParentID = p.ParentID
	}

	if p.Title != "" {
		n.Title = p.Title
	}

	if p.Body != nil {
		n.body = slices.Clone(p.Body)
	}

	s.writes = append(s.writes, fmt.Sprintf("update %s v%d", n.Title, n.Version))
	cp := n.Page

	return &cp, nil
}

// target returns the node a target names, or nil. The lock is held.
func (s *Site) target(t confluence.Target) *node {
	if n, ok := s.nodes[t.ID]; ok && n.kind == t.Type {
		return n
	}

	return nil
}

// Property implements confluence.Client.
func (s *Site) Property(_ context.Context, t confluence.Target, key string) (*confluence.Property, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	n := s.target(t)
	if n == nil {
		return nil, &confluence.RequestError{Op: "get property", Status: 404}
	}

	if p, ok := n.props[key]; ok {
		cp := *p
		cp.Value = slices.Clone(p.Value)

		return &cp, nil
	}

	return nil, nil //nolint:nilnil // a missing property is an answer (Client contract)
}

// SetProperty implements confluence.Client. An empty ID creates the
// property; otherwise ID and Version must be the current ones, and the
// stored version becomes Version+1.
func (s *Site) SetProperty(_ context.Context, t confluence.Target, p *confluence.Property) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	n := s.target(t)
	if n == nil {
		return &confluence.RequestError{Op: "set property", Status: 404}
	}

	if err := s.failing("property", n.Title); err != nil {
		return err
	}

	cur, exists := n.props[p.Key]

	switch {
	case p.ID == "" && exists:
		return &confluence.ConflictError{}
	case p.ID != "" && (!exists || cur.ID != p.ID || cur.Version != p.Version):
		return &confluence.ConflictError{}
	}

	n.props[p.Key] = &confluence.Property{
		ID: "prop-" + n.ID, Key: p.Key, Value: slices.Clone(p.Value), Version: p.Version + 1,
	}
	s.writes = append(s.writes, "property "+n.Title)

	return nil
}

// Children implements confluence.Client. Like direct-children it lists
// pages and folders without their versions.
func (s *Site) Children(_ context.Context, parent confluence.Target) ([]confluence.Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var out []confluence.Node

	for _, id := range slices.SortedFunc(maps.Keys(s.nodes), compareIDs) {
		n := s.nodes[id]
		if n.ParentID == parent.ID && n.status == statusCurrent {
			c := confluence.Node{Page: n.Page, Type: n.kind}
			c.Version = 0
			out = append(out, c)
		}
	}

	return out, nil
}

// compareIDs orders numeric ids numerically.
func compareIDs(a, b string) int {
	if len(a) != len(b) {
		return len(a) - len(b)
	}

	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}

	return 0
}

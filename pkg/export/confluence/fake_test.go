package confluence

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"sync"
)

// fakePage is one page or folder in the fake site.
type fakePage struct {
	Page
	// kind is TypePage or TypeFolder.
	kind string
	// status is "current" or "archived"; an archived page keeps its title
	// reserved, as Confluence's does.
	status string
	body   []byte
	props  map[string]*Property
}

// fakeClient is an in-memory Confluence site: pages and folders by id with
// titles, parents, versions, bodies, and properties, plus a log of every
// write. It enforces Confluence's title rules: page titles are unique in a
// space, folder titles too, and the two do not collide with each other.
type fakeClient struct {
	mu     sync.Mutex
	pages  map[string]*fakePage
	nextID int
	writes []string
	// fail names one operation, "<op> <title>", that returns an error.
	fail string
}

var _ Client = (*fakeClient)(nil)

func newFakeClient() *fakeClient {
	return &fakeClient{pages: make(map[string]*fakePage)}
}

var errFake = errors.New("fake: injected failure")

func (f *fakeClient) failing(op, title string) error {
	if f.fail == op+" "+title {
		return &RequestError{Op: op, Status: 500, Err: errFake}
	}

	return nil
}

func (*fakeClient) SpaceID(_ context.Context, key string) (string, error) {
	return "space-" + key, nil
}

// SpaceHome is a homepage id with no page behind it: content filed under
// it lists as its children, which is all Export asks of it.
func (*fakeClient) SpaceHome(_ context.Context, spaceID string) (string, error) {
	return "home-" + spaceID, nil
}

// holder returns the node of kind holding title in the space, archived
// included, or nil. The lock is held.
func (f *fakeClient) holder(kind, spaceID, title string) *fakePage {
	for _, p := range f.pages {
		if p.kind == kind && p.SpaceID == spaceID && p.Title == title {
			return p
		}
	}

	return nil
}

func (f *fakeClient) FindPage(_ context.Context, spaceID, title string) (*Page, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if p := f.holder(TypePage, spaceID, title); p != nil && p.status == statusCurrent {
		cp := p.Page

		return &cp, nil
	}

	return nil, nil
}

func (f *fakeClient) Page(_ context.Context, id string) (*Page, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if p, ok := f.pages[id]; ok && p.kind == TypePage && p.status == statusCurrent {
		cp := p.Page

		return &cp, nil
	}

	return nil, nil
}

func (f *fakeClient) Body(_ context.Context, id string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if p, ok := f.pages[id]; ok && p.kind == TypePage {
		return slices.Clone(p.body), nil
	}

	return nil, nil
}

func (f *fakeClient) Folder(_ context.Context, id string) (*Folder, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if p, ok := f.pages[id]; ok && p.kind == TypeFolder {
		return &Folder{ID: p.ID, Title: p.Title, ParentID: p.ParentID, SpaceID: p.SpaceID, WebURL: p.WebURL}, nil
	}

	return nil, nil
}

// add stores a new node. The lock is held.
func (f *fakeClient) add(kind, spaceID, parentID, title string, body []byte) *fakePage {
	f.nextID++
	id := strconv.Itoa(100 + f.nextID)
	p := &fakePage{
		Page: Page{
			ID: id, Title: title, ParentID: parentID, SpaceID: spaceID, Version: 1,
			WebURL: "https://example.atlassian.net/wiki/" + kind + "s/" + id,
		},
		kind:   kind,
		status: statusCurrent,
		body:   body,
		props:  make(map[string]*Property),
	}
	f.pages[id] = p

	return p
}

func (f *fakeClient) CreatePage(_ context.Context, p *NewPage) (*Page, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.failing("create", p.Title); err != nil {
		return nil, err
	}

	if h := f.holder(TypePage, p.SpaceID, p.Title); h != nil {
		te := &TitleError{Title: p.Title}
		if h.status == statusArchived {
			te.ArchivedID = h.ID
		}

		return nil, te
	}

	pg := f.add(TypePage, p.SpaceID, p.ParentID, p.Title, p.Body)
	f.writes = append(f.writes, "create "+p.Title)
	cp := pg.Page

	return &cp, nil
}

func (f *fakeClient) CreateFolder(_ context.Context, nf *NewFolder) (*Folder, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if f.holder(TypeFolder, nf.SpaceID, nf.Title) != nil {
		return nil, &TitleError{Title: nf.Title}
	}

	parentID := nf.ParentID
	if parentID == "" {
		parentID = "home-" + nf.SpaceID
	}

	p := f.add(TypeFolder, nf.SpaceID, parentID, nf.Title, nil)
	p.Version = 0
	f.writes = append(f.writes, "create folder "+nf.Title)

	return &Folder{ID: p.ID, Title: p.Title, ParentID: p.ParentID, SpaceID: p.SpaceID, WebURL: p.WebURL}, nil
}

func (f *fakeClient) UpdatePage(_ context.Context, id string, p *PageUpdate) (*Page, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	pg, ok := f.pages[id]
	if !ok || pg.kind != TypePage {
		return nil, &RequestError{Op: "update page", Status: 404}
	}

	if err := f.failing("update", pg.Title); err != nil {
		return nil, err
	}

	want := p.Version
	if p.Body == nil && want == 0 {
		want = pg.Version + 1
	}

	if want != pg.Version+1 {
		return nil, &ConflictError{Title: pg.Title, Want: want - 1}
	}

	if p.Title != "" && p.Title != pg.Title && f.holder(TypePage, pg.SpaceID, p.Title) != nil {
		return nil, &RequestError{Op: "update page", Status: 400, Body: "A page already exists with the same TITLE in this space"}
	}

	pg.Version = want
	pg.ParentID = p.ParentID

	if p.Title != "" {
		pg.Title = p.Title
	}

	if p.Body != nil {
		pg.body = p.Body
	}

	f.writes = append(f.writes, fmt.Sprintf("update %s v%d", pg.Title, pg.Version))
	cp := pg.Page

	return &cp, nil
}

// node returns the node a target names, or nil. The lock is held.
func (f *fakeClient) node(t Target) *fakePage {
	if p, ok := f.pages[t.ID]; ok && p.kind == t.Type {
		return p
	}

	return nil
}

func (f *fakeClient) Property(_ context.Context, t Target, key string) (*Property, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	pg := f.node(t)
	if pg == nil {
		return nil, &RequestError{Op: "get property", Status: 404}
	}

	if p, ok := pg.props[key]; ok {
		cp := *p

		return &cp, nil
	}

	return nil, nil
}

func (f *fakeClient) SetProperty(_ context.Context, t Target, p *Property) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	pg := f.node(t)
	if pg == nil {
		return &RequestError{Op: "set property", Status: 404}
	}

	cur, exists := pg.props[p.Key]

	switch {
	case p.ID == "" && exists:
		return &RequestError{Op: "create property", Status: 409}
	case p.ID != "" && (!exists || cur.ID != p.ID || cur.Version != p.Version):
		return &RequestError{Op: "update property", Status: 409}
	}

	next := &Property{ID: "prop-" + pg.ID, Key: p.Key, Value: p.Value, Version: p.Version + 1}
	pg.props[p.Key] = next
	f.writes = append(f.writes, "property "+pg.Title)

	return nil
}

func (f *fakeClient) Children(_ context.Context, parent Target) ([]Node, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var out []Node

	for _, p := range f.pages {
		if p.ParentID == parent.ID && p.status == statusCurrent {
			n := Node{Page: p.Page, Type: p.kind}
			n.Version = 0
			out = append(out, n)
		}
	}

	slices.SortFunc(out, func(a, b Node) int { return compareIDs(a.ID, b.ID) })

	return out, nil
}

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

// byTitle returns the page (not folder) with title, or nil.
func (f *fakeClient) byTitle(title string) *fakePage {
	return f.find(TypePage, title)
}

// folderByTitle returns the folder with title, or nil.
func (f *fakeClient) folderByTitle(title string) *fakePage {
	return f.find(TypeFolder, title)
}

func (f *fakeClient) find(kind, title string) *fakePage {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, p := range f.pages {
		if p.kind == kind && p.Title == title {
			return p
		}
	}

	return nil
}

// takeWrites returns the write log and clears it.
func (f *fakeClient) takeWrites() []string {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := f.writes
	f.writes = nil

	return out
}

// seed stores a page or folder as if someone else had made it, with a docz
// property holding prop when prop is not empty.
func (f *fakeClient) seed(kind, parentID, title, prop string) *fakePage {
	f.mu.Lock()
	defer f.mu.Unlock()

	p := f.add(kind, "space-DOCZ", parentID, title, []byte("<p>seeded</p>"))
	if prop != "" {
		p.props[propertyKey] = &Property{ID: "prop-" + p.ID, Key: propertyKey, Value: []byte(prop), Version: 1}
	}

	return p
}

// edit bumps a page's version, as an edit in Confluence does.
func (f *fakeClient) edit(title string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, p := range f.pages {
		if p.kind == TypePage && p.Title == title {
			p.Version++
		}
	}
}

package confluence

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strconv"
	"sync"
)

// fakePage is one page in the fake space.
type fakePage struct {
	Page
	body  []byte
	props map[string]*Property
}

// fakeClient is an in-memory Confluence space: pages by id with titles,
// parents, versions, bodies, and properties, plus a log of every write.
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

func (f *fakeClient) FindPage(_ context.Context, spaceID, title string) (*Page, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, p := range f.pages {
		if p.Title == title && p.SpaceID == spaceID {
			cp := p.Page

			return &cp, nil
		}
	}

	return nil, nil
}

func (f *fakeClient) CreatePage(_ context.Context, p *NewPage) (*Page, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if err := f.failing("create", p.Title); err != nil {
		return nil, err
	}

	f.nextID++
	id := strconv.Itoa(100 + f.nextID)
	f.pages[id] = &fakePage{
		Page: Page{
			ID: id, Title: p.Title, ParentID: p.ParentID, SpaceID: p.SpaceID, Version: 1,
			WebURL: "https://example.atlassian.net/wiki/pages/" + id,
		},
		body:  p.Body,
		props: make(map[string]*Property),
	}
	f.writes = append(f.writes, "create "+p.Title)
	cp := f.pages[id].Page

	return &cp, nil
}

func (f *fakeClient) UpdatePage(_ context.Context, id string, p *PageUpdate) (*Page, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	pg, ok := f.pages[id]
	if !ok {
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

func (f *fakeClient) Property(_ context.Context, pageID, key string) (*Property, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	pg, ok := f.pages[pageID]
	if !ok {
		return nil, &RequestError{Op: "get property", Status: 404}
	}

	if p, ok := pg.props[key]; ok {
		cp := *p

		return &cp, nil
	}

	return nil, nil
}

func (f *fakeClient) SetProperty(_ context.Context, pageID string, p *Property) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	pg, ok := f.pages[pageID]
	if !ok {
		return &RequestError{Op: "set property", Status: 404}
	}

	cur, exists := pg.props[p.Key]

	switch {
	case p.ID == "" && exists:
		return &RequestError{Op: "create property", Status: 409}
	case p.ID != "" && (!exists || cur.ID != p.ID || cur.Version != p.Version):
		return &RequestError{Op: "update property", Status: 409}
	}

	next := &Property{ID: "prop-" + pageID, Key: p.Key, Value: p.Value, Version: p.Version + 1}
	pg.props[p.Key] = next
	f.writes = append(f.writes, "property "+pg.Title)

	return nil
}

func (f *fakeClient) Children(_ context.Context, parentID string) ([]Page, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	var out []Page

	for _, p := range f.pages {
		if p.ParentID == parentID {
			out = append(out, p.Page)
		}
	}

	slices.SortFunc(out, func(a, b Page) int { return compareIDs(a.ID, b.ID) })

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

// byTitle returns the page with title, or nil.
func (f *fakeClient) byTitle(title string) *fakePage {
	f.mu.Lock()
	defer f.mu.Unlock()

	for _, p := range f.pages {
		if p.Title == title {
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

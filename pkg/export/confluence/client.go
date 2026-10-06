package confluence

import (
	"context"
	"encoding/json"
)

// Client is the Confluence Cloud surface Export needs. *HTTPClient
// implements it over net/http; a test supplies a fake.
//
// A page or property that does not exist is (nil, nil), not an error: not
// finding one is an ordinary answer to the question Export asks.
type Client interface {
	// SpaceID returns the id of the space with the given key.
	SpaceID(ctx context.Context, key string) (string, error)
	// FindPage returns the current page titled title in the space, or nil.
	FindPage(ctx context.Context, spaceID, title string) (*Page, error)
	// CreatePage creates a page and returns it.
	CreatePage(ctx context.Context, p *NewPage) (*Page, error)
	// UpdatePage writes a new version of a page, moving it under
	// p.ParentID, and returns it. A nil p.Body is a move: the page keeps
	// its current body, and a zero p.Version or empty p.Title means the
	// current one plus one and the current title.
	UpdatePage(ctx context.Context, id string, p *PageUpdate) (*Page, error)
	// Property returns the page's content property named key, or nil.
	Property(ctx context.Context, pageID, key string) (*Property, error)
	// SetProperty creates the property when p.ID is empty and otherwise
	// writes version p.Version+1 of it.
	SetProperty(ctx context.Context, pageID string, p *Property) error
	// Children returns the direct child pages of a page, every page of
	// results followed.
	Children(ctx context.Context, parentID string) ([]Page, error)
}

// Page is a Confluence page as Export sees it.
type Page struct {
	ID       string
	Title    string
	ParentID string
	SpaceID  string
	// Version is the page's version number. Children leaves it zero, since
	// the endpoint does not report it.
	Version int
	// WebURL is the page's address in a browser.
	WebURL string
}

// NewPage is a page to create.
type NewPage struct {
	SpaceID  string
	ParentID string
	Title    string
	// Body is storage format.
	Body []byte
}

// PageUpdate is a new version of a page.
type PageUpdate struct {
	Title    string
	ParentID string
	// Body is storage format.
	Body []byte
	// Version is the new version number, one more than the page's current.
	Version int
	// Message is the version comment Confluence shows in page history.
	Message string
}

// Property is a page's content property.
type Property struct {
	// ID is the property's id; empty for one not yet created.
	ID    string
	Key   string
	Value json.RawMessage
	// Version is the property's current version number.
	Version int
}

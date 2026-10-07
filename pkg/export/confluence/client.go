package confluence

import (
	"context"
	"encoding/json"
)

// Client is the Confluence Cloud surface Export needs. *HTTPClient
// implements it over net/http; a test supplies a fake. The package is
// EXPERIMENTAL, and a caller's own implementation must grow with it.
//
// A page, folder, or property that does not exist is (nil, nil), not an
// error: not finding one is an ordinary answer to the question Export asks.
type Client interface {
	// SpaceID returns the id of the space with the given key.
	SpaceID(ctx context.Context, key string) (string, error)
	// SpaceHome returns the id of the space's homepage, where Confluence
	// files content created with no parent.
	SpaceHome(ctx context.Context, spaceID string) (string, error)
	// FindPage returns the current page titled title in the space, or nil.
	FindPage(ctx context.Context, spaceID, title string) (*Page, error)
	// Page returns the page with the given id, or nil when there is none.
	Page(ctx context.Context, id string) (*Page, error)
	// Body returns a page's current storage-format body, or nil when there
	// is no such page.
	Body(ctx context.Context, id string) ([]byte, error)
	// CreatePage creates a page and returns it. A title another page in the
	// space holds is a *TitleError.
	CreatePage(ctx context.Context, p *NewPage) (*Page, error)
	// UpdatePage writes a new version of a page, moving it under
	// p.ParentID, and returns it. A nil p.Body is a move: the page keeps
	// its current body, and a zero p.Version or empty p.Title means the
	// current one plus one and the current title.
	UpdatePage(ctx context.Context, id string, p *PageUpdate) (*Page, error)
	// Folder returns the folder with the given id, or nil.
	Folder(ctx context.Context, id string) (*Folder, error)
	// CreateFolder creates a folder and returns it. A title another folder
	// in the space holds is a *TitleError.
	CreateFolder(ctx context.Context, f *NewFolder) (*Folder, error)
	// Property returns the content property named key on a page or a
	// folder, or nil.
	Property(ctx context.Context, t Target, key string) (*Property, error)
	// SetProperty creates the property when p.ID is empty and otherwise
	// writes version p.Version+1 of it.
	SetProperty(ctx context.Context, t Target, p *Property) error
	// Children returns the direct children of a page or a folder, pages and
	// folders both, every page of results followed. A missing parent is an
	// empty list.
	Children(ctx context.Context, parent Target) ([]Node, error)
}

// The content types a Node or a Target names.
const (
	TypePage   = "page"
	TypeFolder = "folder"
)

// Target names a page or a folder: the two kinds of content that carry
// properties and children.
type Target struct {
	// Type is TypePage or TypeFolder.
	Type string
	ID   string
}

// PageTarget is the Target for a page.
func PageTarget(id string) Target { return Target{Type: TypePage, ID: id} }

// FolderTarget is the Target for a folder.
func FolderTarget(id string) Target { return Target{Type: TypeFolder, ID: id} }

// Node is a child of a page or a folder: a page or a folder, told apart by
// Type. Version is zero, since the listing does not report it.
type Node struct {
	Page
	// Type is TypePage or TypeFolder.
	Type string `json:"type"`
}

// Folder is a Confluence folder as Export sees it.
type Folder struct {
	ID       string
	Title    string
	ParentID string
	SpaceID  string
	// WebURL is the folder's address in a browser.
	WebURL string
}

// Node returns the folder as a Node.
func (f *Folder) Node() *Node {
	return &Node{
		Page: Page{ID: f.ID, Title: f.Title, ParentID: f.ParentID, SpaceID: f.SpaceID, WebURL: f.WebURL},
		Type: TypeFolder,
	}
}

// NewFolder is a folder to create. An empty ParentID files it under the
// space's homepage.
type NewFolder struct {
	SpaceID  string
	ParentID string
	Title    string
}

// Page is a Confluence page as Export sees it.
type Page struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	ParentID string `json:"parent_id,omitempty"`
	SpaceID  string `json:"space_id,omitempty"`
	// Version is the page's version number. Children leaves it zero, since
	// the endpoint does not report it.
	Version int `json:"version,omitempty"`
	// WebURL is the page's address in a browser.
	WebURL string `json:"url,omitempty"`
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

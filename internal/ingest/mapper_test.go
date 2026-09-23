package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"testing"
	"time"

	doczcfg "github.com/donaldgifford/docz/pkg/doczcore/config"
	doczdoc "github.com/donaldgifford/docz/pkg/doczcore/document"
)

func TestMapDocType(t *testing.T) {
	tc := doczcfg.TypeConfig{
		Dir:         "frameworks",
		IDPrefix:    "FW",
		PluralLabel: "Frameworks",
		Statuses:    []string{"Draft", "Adopted"},
		Aliases:     []string{"fw"},
	}
	got, err := mapDocType("frameworks", &tc)
	if err != nil {
		t.Fatalf("mapDocType: %v", err)
	}
	if got.Name != "frameworks" || got.Dir != "frameworks" || got.IDPrefix != "FW" || got.PluralLabel != "Frameworks" {
		t.Errorf("scalar fields = %+v, want frameworks/frameworks/FW/Frameworks", got)
	}
	if string(got.Statuses) != `["Draft","Adopted"]` {
		t.Errorf("Statuses = %s, want the JSON array", got.Statuses)
	}
	if string(got.Aliases) != `["fw"]` {
		t.Errorf("Aliases = %s, want [\"fw\"]", got.Aliases)
	}
}

func TestMapDocument(t *testing.T) {
	content := []byte("---\nid: FW-0001\n---\n\n# Intro\n")
	blob := BlobEntry{Path: "docs/frameworks/0001-intro.md", GitSHA: "abc123", Content: content}
	fm := doczdoc.Frontmatter{
		ID:      "FW-0001",
		Title:   "Example Framework",
		Status:  "Draft",
		Author:  "Test Author",
		Created: "2026-07-01",
	}

	got, err := mapDocument("frameworks", &blob, &fm)
	if err != nil {
		t.Fatalf("mapDocument: %v", err)
	}

	sum := sha256.Sum256(content)
	wantHash := hex.EncodeToString(sum[:])
	if got.ContentHash != wantHash {
		t.Errorf("ContentHash = %q, want %q", got.ContentHash, wantHash)
	}
	if got.Type != "frameworks" || got.DocID != "FW-0001" || got.Title != "Example Framework" {
		t.Errorf("type/id/title = %q/%q/%q", got.Type, got.DocID, got.Title)
	}
	if got.Status != "Draft" || got.Author != "Test Author" {
		t.Errorf("status/author = %q/%q", got.Status, got.Author)
	}
	if got.Path != blob.Path || got.GitSHA != "abc123" || got.RawMD != string(content) {
		t.Errorf("path/sha/raw mismatch: %+v", got)
	}
	want := time.Date(2026, time.July, 1, 0, 0, 0, 0, time.UTC)
	if !got.Created.Equal(want) {
		t.Errorf("Created = %v, want %v", got.Created, want)
	}
}

func TestMapDocumentEmptyCreatedIsZero(t *testing.T) {
	blob := BlobEntry{Path: "docs/frameworks/0002-x.md", Content: []byte("body")}
	fm := doczdoc.Frontmatter{ID: "FW-0002", Created: ""}
	got, err := mapDocument("frameworks", &blob, &fm)
	if err != nil {
		t.Fatalf("mapDocument: %v", err)
	}
	if !got.Created.IsZero() {
		t.Errorf("Created = %v, want zero time (→ SQL NULL)", got.Created)
	}
}

func TestMapDocumentBadDate(t *testing.T) {
	blob := BlobEntry{Path: "docs/frameworks/0003-x.md", Content: []byte("body")}
	fm := doczdoc.Frontmatter{ID: "FW-0003", Created: "July 1st"}
	if _, err := mapDocument("frameworks", &blob, &fm); err == nil {
		t.Fatal("mapDocument with a malformed created date = nil error, want error")
	}
}

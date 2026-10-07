//go:build live

package confluence

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"
)

// TestLive round trips one page and its docz property against a real
// Confluence site (IMPL-0023 Open Question 9). It runs only under the live
// tag, through `just export-live`, and leaves what it wrote in place: the
// page is updated in place on the next run.
func TestLive(t *testing.T) {
	env := func(k string) string {
		v := os.Getenv(k)
		if v == "" {
			t.Skipf("%s is not set", k)
		}

		return v
	}

	h := NewHTTPClient(env("ATLASSIAN_SITE"), env("ATLASSIAN_EMAIL"), env("ATLASSIAN_API_TOKEN"))
	space := env("DOCZ_LIVE_SPACE")

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()

	spaceID, err := h.SpaceID(ctx, space)
	if err != nil {
		t.Fatalf("space %s: %v", space, err)
	}

	parent := livePage(ctx, t, h, &NewPage{SpaceID: spaceID, Title: "docz live", Body: []byte("<p>docz live tests.</p>")})

	stamp := time.Now().UTC().Format(time.RFC3339)
	body := []byte("<p>Round trip at " + stamp + ".</p>")

	page := livePage(ctx, t, h, &NewPage{SpaceID: spaceID, ParentID: parent.ID, Title: "docz live: round trip", Body: body})
	if page.Version > 1 || page.ParentID != parent.ID {
		page, err = h.UpdatePage(ctx, page.ID, &PageUpdate{
			Title: page.Title, ParentID: parent.ID, Body: body, Version: page.Version + 1, Message: "docz live",
		})
		if err != nil {
			t.Fatalf("update: %v", err)
		}
	}

	if page.ParentID != parent.ID || page.WebURL == "" {
		t.Errorf("page %+v; want it under %s with a web URL", page, parent.ID)
	}

	prop, err := h.Property(ctx, PageTarget(page.ID), "docz")
	if err != nil {
		t.Fatalf("property: %v", err)
	}

	value := json.RawMessage(fmt.Sprintf(`{"stamp":%q}`, stamp))
	if prop == nil {
		prop = &Property{Key: "docz"}
	}

	prop.Value = value
	if err := h.SetProperty(ctx, PageTarget(page.ID), prop); err != nil {
		t.Fatalf("set property: %v", err)
	}

	got, err := h.Property(ctx, PageTarget(page.ID), "docz")
	if err != nil || got == nil {
		t.Fatalf("read back property: %v %v", got, err)
	}

	var decoded struct{ Stamp string }
	if err := json.Unmarshal(got.Value, &decoded); err != nil || decoded.Stamp != stamp {
		t.Errorf("property %s; want stamp %s", got.Value, stamp)
	}

	children, err := h.Children(ctx, PageTarget(parent.ID))
	if err != nil {
		t.Fatalf("children: %v", err)
	}

	found := false

	for _, c := range children {
		found = found || c.ID == page.ID
	}

	if !found {
		t.Errorf("children of %s do not include %s", parent.ID, page.ID)
	}

	t.Logf("round trip ok: %s", page.WebURL)
}

// TestLiveFolder round trips a folder (IMPL-0024 Phase 3): create it,
// set and read its property, create a page in it, list it through
// direct-children, archive the page under an Archive page inside it, and
// delete everything it made, so a run leaves the space as it found it.
func TestLiveFolder(t *testing.T) {
	h, spaceID := liveClient(t)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()

	stamp := time.Now().UTC().Format("20060102T150405")

	folder, err := h.CreateFolder(ctx, &NewFolder{SpaceID: spaceID, Title: "docz live folder " + stamp})
	if err != nil {
		t.Fatalf("create folder: %v", err)
	}

	t.Cleanup(func() { liveDelete(t, h, "folders", folder.ID) })

	if got, err := h.Folder(ctx, folder.ID); err != nil || got == nil || got.Title != folder.Title {
		t.Fatalf("read folder back: %+v %v", got, err)
	}

	if _, err := h.CreateFolder(ctx, &NewFolder{SpaceID: spaceID, Title: folder.Title}); !isTitleError(err) {
		t.Errorf("second folder with the title: %v; want a TitleError", err)
	}

	value := json.RawMessage(`{"repo":"donaldgifford/docz","docz":"live"}`)
	if err := h.SetProperty(ctx, FolderTarget(folder.ID), &Property{Key: "docz", Value: value}); err != nil {
		t.Fatalf("set folder property: %v", err)
	}

	prop, err := h.Property(ctx, FolderTarget(folder.ID), "docz")
	if err != nil || prop == nil || !strings.Contains(string(prop.Value), "donaldgifford/docz") {
		t.Fatalf("folder property: %+v %v", prop, err)
	}

	page, err := h.CreatePage(ctx, &NewPage{
		SpaceID: spaceID, ParentID: folder.ID, Title: "docz live page " + stamp, Body: []byte("<p>kept</p>"),
	})
	if err != nil {
		t.Fatalf("create page in folder: %v", err)
	}

	t.Cleanup(func() { liveDelete(t, h, "pages", page.ID) })

	archive, err := h.CreatePage(ctx, &NewPage{
		SpaceID: spaceID, ParentID: folder.ID, Title: "docz live archive " + stamp, Body: []byte("<p>archive</p>"),
	})
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}

	t.Cleanup(func() { liveDelete(t, h, "pages", archive.ID) })

	children, err := h.Children(ctx, FolderTarget(folder.ID))
	if err != nil {
		t.Fatalf("direct-children: %v", err)
	}

	if len(children) != 2 || children[0].Type != TypePage {
		t.Errorf("folder children %+v; want the two pages", children)
	}

	moved, err := h.UpdatePage(ctx, page.ID, &PageUpdate{ParentID: archive.ID, Message: "docz live: archived"})
	if err != nil || moved.ParentID != archive.ID {
		t.Fatalf("archive move: %+v %v", moved, err)
	}

	if body, err := h.Body(ctx, page.ID); err != nil || !strings.Contains(string(body), "kept") {
		t.Errorf("body after the move %q %v; want it kept", body, err)
	}

	if got, err := h.Page(ctx, "1"); err != nil || got != nil {
		t.Errorf("Page of a missing id: %+v %v; want nil, nil", got, err)
	}

	t.Logf("folder round trip ok: %s", folder.WebURL)
}

// liveClient builds the client and resolves the space, skipping without
// credentials.
func liveClient(t *testing.T) (*HTTPClient, string) {
	t.Helper()

	env := func(k string) string {
		v := os.Getenv(k)
		if v == "" {
			t.Skipf("%s is not set", k)
		}

		return v
	}

	h := NewHTTPClient(env("ATLASSIAN_SITE"), env("ATLASSIAN_EMAIL"), env("ATLASSIAN_API_TOKEN"))

	spaceID, err := h.SpaceID(t.Context(), env("DOCZ_LIVE_SPACE"))
	if err != nil {
		t.Fatalf("space: %v", err)
	}

	return h, spaceID
}

// liveDelete deletes and purges what a live test made. The Client has no
// delete, since docz never deletes; the test reaches the transport itself.
func liveDelete(t *testing.T, h *HTTPClient, kind, id string) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	path := "/wiki/api/v2/" + kind + "/" + id

	// Confluence answers a delete with an occasional 500 that a retry
	// clears; a 404 means an earlier attempt went through.
	var err error

	for attempt := range 4 {
		if attempt > 0 {
			time.Sleep(time.Duration(attempt) * 2 * time.Second)
		}

		if err = h.do(ctx, "delete", http.MethodDelete, path, nil, nil); err == nil || notFound(err) {
			break
		}
	}

	if err != nil && !notFound(err) {
		t.Errorf("delete %s %s: %v", kind, id, err)

		return
	}

	if kind == "pages" {
		if err := h.do(ctx, "purge", http.MethodDelete, path+"?purge=true", nil, nil); err != nil {
			t.Logf("purge page %s: %v", id, err)
		}
	}
}

func isTitleError(err error) bool {
	var te *TitleError

	return errors.As(err, &te)
}

// livePage finds the page titled p.Title or creates it.
func livePage(ctx context.Context, t *testing.T, h *HTTPClient, p *NewPage) *Page {
	t.Helper()

	page, err := h.FindPage(ctx, p.SpaceID, p.Title)
	if err != nil {
		t.Fatalf("find %q: %v", p.Title, err)
	}

	if page != nil {
		return page
	}

	page, err = h.CreatePage(ctx, p)
	if err != nil {
		t.Fatalf("create %q: %v", p.Title, err)
	}

	return page
}

// TestLiveInlineComment proves #158 on a real site (IMPL-0024 Phase 5): an
// inline comment made through the API leaves the page's version alone
// (DESIGN-0021 §4 amendment), and an update that carries its marker keeps
// it anchored rather than dangling.
func TestLiveInlineComment(t *testing.T) {
	h, spaceID := liveClient(t)

	ctx, cancel := context.WithTimeout(t.Context(), 2*time.Minute)
	defer cancel()

	stamp := time.Now().UTC().Format("20060102T150405")

	page, err := h.CreatePage(ctx, &NewPage{
		SpaceID: spaceID, Title: "docz live comment " + stamp,
		Body: []byte("<p>Before. We chose Postgres for the store.</p>"),
	})
	if err != nil {
		t.Fatalf("create page: %v", err)
	}

	t.Cleanup(func() { liveDelete(t, h, "pages", page.ID) })

	var comment struct {
		ID               string `json:"id"`
		ResolutionStatus string `json:"resolutionStatus"`
		Properties       struct {
			Ref string `json:"inlineMarkerRef"`
		} `json:"properties"`
	}

	req := map[string]any{
		"pageId": page.ID,
		"body":   map[string]string{"representation": "storage", "value": "<p>docz live comment</p>"},
		"inlineCommentProperties": map[string]any{
			"textSelection": "Postgres", "textSelectionMatchCount": 1, "textSelectionMatchIndex": 0,
		},
	}
	if err := h.do(ctx, "create inline comment", http.MethodPost, "/wiki/api/v2/inline-comments", req, &comment); err != nil {
		t.Fatalf("create inline comment: %v", err)
	}

	after, err := h.Page(ctx, page.ID)
	if err != nil || after == nil || after.Version != page.Version {
		t.Fatalf("page after the comment: %+v %v; want version %d still", after, err, page.Version)
	}

	current, err := h.Body(ctx, page.ID)
	if err != nil {
		t.Fatal(err)
	}

	markers := collectMarkers(current)
	if len(markers) != 1 || markers[0].text != "Postgres" || markers[0].ref != comment.Properties.Ref {
		t.Fatalf("markers %+v; want Postgres under ref %s", markers, comment.Properties.Ref)
	}

	out, kept, lost := carryMarkers([]byte("<p>Changed intro. We chose Postgres for the store, still.</p>"), markers)
	if kept != 1 || len(lost) != 0 {
		t.Fatalf("kept %d lost %v", kept, lost)
	}

	if _, err := h.UpdatePage(ctx, page.ID, &PageUpdate{
		Title: page.Title, Body: out, Version: after.Version + 1, Message: "docz live: comment carried",
	}); err != nil {
		t.Fatalf("update: %v", err)
	}

	if body, err := h.Body(ctx, page.ID); err != nil || !strings.Contains(string(body), comment.Properties.Ref) {
		t.Errorf("body after the update lost the marker: %s %v", body, err)
	}

	if err := h.do(ctx, "get inline comment", http.MethodGet, "/wiki/api/v2/inline-comments/"+comment.ID, nil, &comment); err != nil {
		t.Fatalf("read the comment back: %v", err)
	}

	if comment.ResolutionStatus == "dangling" {
		t.Errorf("the comment is dangling after the update")
	}

	t.Logf("comment %s on page %s: %s after the update", comment.ID, page.ID, comment.ResolutionStatus)
}

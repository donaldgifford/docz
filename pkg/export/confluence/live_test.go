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
	if err := h.do(ctx, "delete", http.MethodDelete, path, nil, nil); err != nil {
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

//go:build live

package confluence

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
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

	prop, err := h.Property(ctx, page.ID, "docz")
	if err != nil {
		t.Fatalf("property: %v", err)
	}

	value := json.RawMessage(fmt.Sprintf(`{"stamp":%q}`, stamp))
	if prop == nil {
		prop = &Property{Key: "docz"}
	}

	prop.Value = value
	if err := h.SetProperty(ctx, page.ID, prop); err != nil {
		t.Fatalf("set property: %v", err)
	}

	got, err := h.Property(ctx, page.ID, "docz")
	if err != nil || got == nil {
		t.Fatalf("read back property: %v %v", got, err)
	}

	var decoded struct{ Stamp string }
	if err := json.Unmarshal(got.Value, &decoded); err != nil || decoded.Stamp != stamp {
		t.Errorf("property %s; want stamp %s", got.Value, stamp)
	}

	children, err := h.Children(ctx, parent.ID)
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

package confluence

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
)

// The folder layout (DESIGN-0021 §2, IMPL-0024 Phase 3), over the fake.

const (
	widgets = "acme/widgets"
	home    = "home-space-DOCZ"
)

// folderRepo is exportRepo in the folder layout, under parent (empty for
// the top of the space).
func folderRepo(t *testing.T, parent string) *repo.Repo {
	t.Helper()

	rp := exportRepo(t)
	rp.Cfg.Sync.Confluence.Layout = config.LayoutFolder
	rp.Cfg.Sync.Confluence.Parent = parent

	return rp
}

// exportAs exports as the named repository.
func exportAs(t *testing.T, rp *repo.Repo, c *fakeClient, repository string, opts ExportOptions) Report { //nolint:gocritic // mirrors Export
	t.Helper()

	opts.Repository = repository

	return export(t, rp, c, opts)
}

// w prefixes a title with the widgets folder.
func w(title string) string { return "widgets: " + title }

func TestFolder_TreeAndTitles(t *testing.T) {
	t.Parallel()

	rp, c := folderRepo(t, ""), newFakeClient()
	rep := exportAs(t, rp, c, widgets, ExportOptions{})

	wantActions(t, &rep,
		"widgets=created", w("RFCs")+"=created", w(rfc1)+"=created", w(rfc2)+"=created",
		w("ADRs")+"=created", w(adr1)+"=created", w("Design")+"=created",
		w("Implementation Plans")+"=created", w("Investigations")+"=created")

	folder := c.folderByTitle("widgets")
	if folder == nil || folder.ParentID != home {
		t.Fatalf("folder %+v; want widgets under the space homepage", folder)
	}

	if rep.Folder == nil || rep.Folder.ID != folder.ID || rep.Folder.Type != TypeFolder {
		t.Errorf("Report.Folder %+v; want the folder", rep.Folder)
	}

	if got := string(folder.props[propertyKey].Value); !strings.Contains(got, `"repo":"acme/widgets"`) {
		t.Errorf("folder property %s lacks the repository", got)
	}

	for title, under := range map[string]string{
		"widgets": folder.ID, w("RFCs"): folder.ID, w("ADRs"): folder.ID,
		w(rfc1): c.byTitle(w("RFCs")).ID, w(adr1): c.byTitle(w("ADRs")).ID,
	} {
		if got := c.byTitle(title); got == nil || got.ParentID != under {
			t.Errorf("%s not under %s", title, under)
		}
	}

	page := c.byTitle(w(rfc1))
	if prop := string(page.props[propertyKey].Value); !strings.Contains(prop, `"repo":"acme/widgets"`) {
		t.Errorf("page property %s lacks the repository", prop)
	}

	if !strings.Contains(string(c.byTitle(w("RFCs")).body), `ri:content-title="`+w(rfc1)+`"`) {
		t.Error("the RFC index does not link to the prefixed title")
	}

	if res := rep.Pages[2]; res.Key != "RFC-0001" || !strings.HasPrefix(res.Hash, "sha256:") {
		t.Errorf("result %+v; want key and hash", res)
	}

	// A second run finds the folder among the homepage's children.
	c.takeWrites()

	rep = exportAs(t, rp, c, widgets, ExportOptions{})
	if n := rep.Count(Unchanged); n != len(rep.Pages) {
		t.Errorf("second run:\n%s", actions(&rep))
	}

	for _, wr := range c.takeWrites() {
		if strings.HasPrefix(wr, "create") {
			t.Errorf("second run wrote %q", wr)
		}
	}
}

func TestFolder_HomePageNamesTheRepository(t *testing.T) {
	t.Parallel()

	rp, c := folderRepo(t, ""), newFakeClient()
	exportAs(t, rp, c, widgets, ExportOptions{})

	if body := string(c.byTitle("widgets").body); !strings.Contains(body, "acme/widgets") {
		t.Errorf("home page body does not name the repository:\n%s", body)
	}
}

func TestFolder_UnderTheParentPage(t *testing.T) {
	t.Parallel()

	rp, c := folderRepo(t, "Engineering"), newFakeClient()
	parent := c.seed(TypePage, "", "Engineering", "")

	exportAs(t, rp, c, widgets, ExportOptions{})

	if f := c.folderByTitle("widgets"); f == nil || f.ParentID != parent.ID {
		t.Fatalf("folder %+v; want it under Engineering", f)
	}

	c.takeWrites()
	exportAs(t, rp, c, widgets, ExportOptions{})

	if wr := c.takeWrites(); len(wr) != 0 {
		t.Errorf("second run wrote %v", wr)
	}
}

func TestFolder_ParentPageMissingIsConfigError(t *testing.T) {
	t.Parallel()

	rp, c := folderRepo(t, "Engineering"), newFakeClient()

	_, err := Export(t.Context(), rp, ExportOptions{Client: c, Repository: widgets})
	if !errors.Is(err, ErrConfig) || !strings.Contains(err.Error(), "Engineering") {
		t.Errorf("err %v; want a ConfigError naming the parent", err)
	}
}

func TestFolder_NotThisRepositorys(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		prop string
		want string
	}{
		{"another repository's", `{"repo":"other/widgets","docz":"test"}`, "belongs to other/widgets"},
		{"no property", "", "belongs to no docz export"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rp, c := folderRepo(t, ""), newFakeClient()
			c.seed(TypeFolder, home, "widgets", tt.prop)

			for _, force := range []bool{false, true} {
				_, err := Export(t.Context(), rp, ExportOptions{Client: c, Repository: widgets, Force: force})
				if !errors.Is(err, ErrConfig) || !strings.Contains(err.Error(), tt.want) ||
					!strings.Contains(err.Error(), "sync.confluence.folder") {
					t.Errorf("force=%v: err %v; want a ConfigError: %s", force, err, tt.want)
				}
			}

			if wr := c.takeWrites(); len(wr) != 0 {
				t.Errorf("wrote %v", wr)
			}
		})
	}
}

func TestFolder_TitleTakenElsewhere(t *testing.T) {
	t.Parallel()

	rp, c := folderRepo(t, ""), newFakeClient()
	other := c.seed(TypePage, home, "Elsewhere", "")
	c.seed(TypeFolder, other.ID, "widgets", `{"repo":"other/widgets"}`)

	_, err := Export(t.Context(), rp, ExportOptions{Client: c, Repository: widgets})
	if !errors.Is(err, ErrConfig) || !strings.Contains(err.Error(), "taken elsewhere") {
		t.Errorf("err %v; want a ConfigError for the taken title", err)
	}
}

func TestFolder_RecordedFolderID(t *testing.T) {
	t.Parallel()

	rp, c := folderRepo(t, ""), newFakeClient()
	rep := exportAs(t, rp, c, widgets, ExportOptions{})

	// Moved by hand somewhere the title search does not look: the recorded
	// id still finds it.
	moved := c.seed(TypePage, home, "Moved here", "")
	c.folderByTitle("widgets").ParentID = moved.ID

	rep = exportAs(t, rp, c, widgets, ExportOptions{Folder: rep.Folder.ID})
	if rep.Count(Unchanged) != len(rep.Pages) {
		t.Errorf("run with the recorded folder:\n%s", actions(&rep))
	}
}

func TestFolder_RecordedPageIDs(t *testing.T) {
	t.Parallel()

	rp, c := folderRepo(t, ""), newFakeClient()
	rep := exportAs(t, rp, c, widgets, ExportOptions{})

	pages := make(map[string]string)
	for _, p := range rep.Pages {
		pages[p.Key] = p.PageID
	}

	id := pages["RFC-0001"]

	src := "docs/rfc/0001-first-proposal.md"

	b, err := os.ReadFile(rp.Path(src))
	if err != nil {
		t.Fatal(err)
	}

	writeDoc(t, rp, src, strings.ReplaceAll(string(b), "First proposal", "Renamed proposal"))

	c.takeWrites()

	rep = exportAs(t, rp, c, widgets, ExportOptions{Pages: pages, IDs: []string{"RFC-0001"}})
	if res := rep.Pages[2]; res.Action != Updated || res.PageID != id || res.Title != w("RFC-0001: Renamed proposal") {
		t.Fatalf("renamed: %+v; want page %s updated in place", res, id)
	}

	for _, wr := range c.takeWrites() {
		if strings.HasPrefix(wr, "create") {
			t.Errorf("renaming created %q", wr)
		}
	}

	// A recorded id that is gone falls back to the title.
	pages["RFC-0002"] = "999"

	rep = exportAs(t, rp, c, widgets, ExportOptions{Pages: pages, IDs: []string{"RFC-0002"}})
	if res := rep.Pages[2]; res.Action != Unchanged {
		t.Errorf("gone id: %+v; want unchanged by title", res)
	}
}

func TestFolder_OverwriteForceAndNeither(t *testing.T) {
	t.Parallel()

	type mode struct {
		name             string
		overwrite, force bool
	}

	modes := []mode{{"neither", false, false}, {"overwrite", true, false}, {"force", false, true}}

	tests := []struct {
		name  string
		setup func(c *fakeClient)
		want  map[string]Action
		// edited reports whether the result carries Edited.
		edited bool
		reason string
	}{
		{
			name:   "edited page",
			setup:  func(c *fakeClient) { c.edit(w(rfc1)) },
			want:   map[string]Action{"neither": Skipped, "overwrite": Updated, "force": Updated},
			edited: true,
		},
		{
			name: "page with no property",
			setup: func(c *fakeClient) {
				delete(c.byTitle(w(rfc1)).props, propertyKey)
			},
			want:   map[string]Action{"neither": Skipped, "overwrite": Skipped, "force": Updated},
			reason: "not docz's page",
		},
		{
			name: "another repository's page",
			setup: func(c *fakeClient) {
				p := c.byTitle(w(rfc1))
				p.props[propertyKey].Value = []byte(fmt.Sprintf(`{"id":"RFC-0001","version":%d,"repo":"other/widgets"}`, p.Version))
			},
			want:   map[string]Action{"neither": Skipped, "overwrite": Skipped, "force": Skipped},
			reason: "belongs to other/widgets",
		},
	}

	for _, tt := range tests {
		for _, m := range modes {
			t.Run(tt.name+"/"+m.name, func(t *testing.T) {
				t.Parallel()

				rp, c := folderRepo(t, ""), newFakeClient()
				exportAs(t, rp, c, widgets, ExportOptions{})
				tt.setup(c)

				rep := exportAs(t, rp, c, widgets, ExportOptions{IDs: []string{"RFC-0001"}, Overwrite: m.overwrite, Force: m.force})
				res := rep.Pages[2]

				if res.Action != tt.want[m.name] {
					t.Errorf("action %s (%s); want %s", res.Action, res.Reason, tt.want[m.name])
				}

				if tt.edited && (res.Edited == nil || res.Edited.Version != 2 || res.Edited.Expected != 1) {
					t.Errorf("Edited %+v; want v2 expected v1", res.Edited)
				}

				if res.Action == Skipped && tt.reason != "" && res.Reason != tt.reason {
					t.Errorf("reason %q; want %q", res.Reason, tt.reason)
				}
			})
		}
	}
}

func TestFolder_OrphansMoveToTheFoldersArchive(t *testing.T) {
	t.Parallel()

	rp, c := folderRepo(t, ""), newFakeClient()
	exportAs(t, rp, c, widgets, ExportOptions{})

	folder := c.folderByTitle("widgets")
	// Another repository's page that wandered into the folder stays put.
	stray := c.seed(TypePage, folder.ID, "Someone else's", `{"id":"X-1","version":1,"repo":"other/widgets"}`)

	if err := os.Remove(rp.Path("docs/rfc/0002-second-proposal.md")); err != nil {
		t.Fatal(err)
	}

	rep := exportAs(t, rp, c, widgets, ExportOptions{})
	if rep.Count(Archived) != 1 {
		t.Fatalf("archived %d; want 1:\n%s", rep.Count(Archived), actions(&rep))
	}

	archive := c.byTitle(w("Archive"))
	if archive == nil || archive.ParentID != folder.ID || c.byTitle(w(rfc2)).ParentID != archive.ID {
		t.Fatal("RFC-0002 is not under widgets: Archive in the folder")
	}

	if c.byTitle(stray.Title).ParentID != folder.ID {
		t.Error("another repository's page was moved")
	}
}

func TestFolder_TwoRepositoriesShareASpace(t *testing.T) {
	t.Parallel()

	c := newFakeClient()
	a, b := folderRepo(t, ""), folderRepo(t, "")

	for _, run := range []struct {
		rp   *repo.Repo
		name string
	}{{a, widgets}, {b, "acme/gadgets"}, {a, widgets}, {b, "acme/gadgets"}} {
		rep, err := Export(t.Context(), run.rp, ExportOptions{Client: c, Repository: run.name, Version: "test"})
		if err != nil || rep.Count(Failed) != 0 || rep.Count(Skipped) != 0 {
			t.Fatalf("%s: %v\n%s", run.name, err, actions(&rep))
		}
	}

	if c.byTitle(w(rfc1)) == nil || c.byTitle("gadgets: "+rfc1) == nil {
		t.Fatal("each repository should have its own RFC-0001")
	}

	// Neither may take the other's folder, under Force included.
	b.Cfg.Sync.Confluence.Folder = "widgets"

	_, err := Export(t.Context(), b, ExportOptions{Client: c, Repository: "acme/gadgets", Force: true})
	if !errors.Is(err, ErrConfig) {
		t.Errorf("gadgets into the widgets folder: %v; want a ConfigError", err)
	}
}

func TestFolder_TitleHeldByAnArchivedPage(t *testing.T) {
	t.Parallel()

	rp, c := folderRepo(t, ""), newFakeClient()
	held := c.seed(TypePage, home, w(rfc1), "")
	held.status = statusArchived

	rep, err := Export(t.Context(), rp, ExportOptions{Client: c, Repository: widgets})
	if err == nil {
		t.Fatal("want the page's failure")
	}

	res := rep.Pages[2]
	if res.Action != Failed || !strings.Contains(res.Reason, "held by archived page "+held.ID) {
		t.Errorf("result %+v; want failed naming archived page %s", res, held.ID)
	}

	if rep.Count(Failed) != 1 {
		t.Errorf("failed %d; want only the one page", rep.Count(Failed))
	}
}

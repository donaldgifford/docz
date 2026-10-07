package confluence

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
)

// exportRepo builds a repository with sync enabled and two RFCs and one
// ADR in it.
func exportRepo(t *testing.T) *repo.Repo {
	t.Helper()

	cfg := config.DefaultConfig()
	cfg.Sync.Confluence = config.ConfluenceSyncConfig{
		Enabled: true, Site: "https://example.atlassian.net", Space: "DOCZ", Parent: "docz",
		Mermaid: config.MermaidSyncConfig{Viewer: config.MermaidViewerAuto},
	}

	rp := &repo.Repo{Root: t.TempDir(), Cfg: &cfg}
	ctx := t.Context()

	if _, err := rp.Init(ctx, repo.InitOptions{}); err != nil {
		t.Fatal(err)
	}

	now := time.Date(2026, 3, 4, 0, 0, 0, 0, time.UTC)

	for _, d := range []struct{ typ, title string }{
		{"rfc", "First proposal"}, {"rfc", "Second proposal"}, {"adr", "A decision"},
	} {
		if _, err := rp.Create(ctx, repo.CreateOptions{Type: d.typ, Title: d.title, Author: "T", Now: now, Update: true}); err != nil {
			t.Fatal(err)
		}
	}

	return rp
}

func export(t *testing.T, rp *repo.Repo, c *fakeClient, opts ExportOptions) Report { //nolint:gocritic // mirrors Export
	t.Helper()

	opts.Client = c
	opts.Version = "test"

	rep, err := Export(t.Context(), rp, opts)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}

	return rep
}

// actions renders a report as "title=action" lines.
func actions(rep *Report) string {
	lines := make([]string, 0, len(rep.Pages))

	for i := range rep.Pages {
		lines = append(lines, rep.Pages[i].Title+"="+rep.Pages[i].Action.String())
	}

	return strings.Join(lines, "\n")
}

func wantActions(t *testing.T, rep *Report, want ...string) {
	t.Helper()

	if got := actions(rep); got != strings.Join(want, "\n") {
		t.Errorf("actions:\n%s\nwant:\n%s", got, strings.Join(want, "\n"))
	}
}

func writeDoc(t *testing.T, rp *repo.Repo, rel, content string) {
	t.Helper()

	if err := os.WriteFile(rp.Path(rel), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func appendDoc(t *testing.T, rp *repo.Repo, rel, extra string) {
	t.Helper()

	b, err := os.ReadFile(rp.Path(rel))
	if err != nil {
		t.Fatal(err)
	}

	writeDoc(t, rp, rel, string(b)+extra)
}

const (
	rfc1 = "RFC-0001: First proposal"
	rfc2 = "RFC-0002: Second proposal"
	adr1 = "ADR-0001: A decision"
)

func TestExport_FirstRunCreatesTheTree(t *testing.T) {
	t.Parallel()

	rp, c := exportRepo(t), newFakeClient()
	rep := export(t, rp, c, ExportOptions{})

	wantActions(t, &rep,
		"docz=created", "RFCs=created", rfc1+"=created", rfc2+"=created",
		"ADRs=created", adr1+"=created", "Design=created", "Implementation Plans=created", "Investigations=created")

	parent := c.byTitle("docz")
	if parent.ParentID != "" {
		t.Errorf("parent under %q; want the space root", parent.ParentID)
	}

	rfcs := c.byTitle("RFCs")
	for title, under := range map[string]*fakePage{"RFCs": parent, rfc1: rfcs, rfc2: rfcs, adr1: c.byTitle("ADRs")} {
		if got := c.byTitle(title); got == nil || got.ParentID != under.ID {
			t.Errorf("%s not under %s", title, under.Title)
		}
	}

	if !strings.Contains(string(rfcs.body), `<ri:page ri:content-title="`+rfc1+`"`) {
		t.Errorf("the RFC index does not link to its documents:\n%s", rfcs.body)
	}

	prop := c.byTitle(rfc1).props[propertyKey]
	for _, want := range []string{`"id":"RFC-0001"`, `"version":1`, `"docz":"test"`, `"hash":"sha256:`, `"source":"docs/rfc/0001-first-proposal.md"`} {
		if !strings.Contains(string(prop.Value), want) {
			t.Errorf("property %s lacks %s", prop.Value, want)
		}
	}
}

func TestExport_SecondRunWritesNothing(t *testing.T) {
	t.Parallel()

	rp, c := exportRepo(t), newFakeClient()
	export(t, rp, c, ExportOptions{})
	c.takeWrites()

	rep := export(t, rp, c, ExportOptions{})

	if n := rep.Count(Unchanged); n != len(rep.Pages) {
		t.Errorf("%d of %d unchanged:\n%s", n, len(rep.Pages), actions(&rep))
	}

	if w := c.takeWrites(); len(w) != 0 {
		t.Errorf("second run wrote %v", w)
	}
}

func TestExport_ChangedDocumentIsUpdated(t *testing.T) {
	t.Parallel()

	rp, c := exportRepo(t), newFakeClient()
	export(t, rp, c, ExportOptions{})
	c.takeWrites()

	appendDoc(t, rp, "docs/rfc/0001-first-proposal.md", "\nA new paragraph.\n")
	rep := export(t, rp, c, ExportOptions{})

	if got := rep.Count(Updated); got != 1 {
		t.Fatalf("%d updated, want 1:\n%s", got, actions(&rep))
	}

	if w := c.takeWrites(); strings.Join(w, ",") != "update "+rfc1+" v2,property "+rfc1 {
		t.Errorf("writes %v", w)
	}

	if !strings.Contains(string(c.byTitle(rfc1).props[propertyKey].Value), `"version":2`) {
		t.Error("the property does not record version 2")
	}
}

func TestExport_EditedInConfluence(t *testing.T) {
	t.Parallel()

	rp, c := exportRepo(t), newFakeClient()
	export(t, rp, c, ExportOptions{})

	pg := c.byTitle(rfc1)
	edit := &PageUpdate{Title: rfc1, ParentID: pg.ParentID, Body: []byte("<p>hand edit</p>"), Version: 2}
	if _, err := c.UpdatePage(t.Context(), pg.ID, edit); err != nil {
		t.Fatal(err)
	}

	appendDoc(t, rp, "docs/rfc/0001-first-proposal.md", "\nMore.\n")

	rep := export(t, rp, c, ExportOptions{})
	res := rep.Pages[2]

	if res.Action != Skipped || res.Reason != "edited in Confluence (v2, expected v1)" {
		t.Errorf("got %s %q", res.Action, res.Reason)
	}

	rep = export(t, rp, c, ExportOptions{Force: true})
	if res := rep.Pages[2]; res.Action != Updated || res.Version != 3 {
		t.Errorf("under force: %s v%d; want updated v3", res.Action, res.Version)
	}
}

func TestExport_PageWithoutPropertyIsAdoptedOnlyUnderForce(t *testing.T) {
	t.Parallel()

	rp, c := exportRepo(t), newFakeClient()

	if _, err := c.CreatePage(t.Context(), &NewPage{SpaceID: "space-DOCZ", Title: adr1, Body: []byte("<p>mine</p>")}); err != nil {
		t.Fatal(err)
	}

	rep := export(t, rp, c, ExportOptions{})
	if res := rep.Pages[5]; res.Title != adr1 || res.Action != Skipped || res.Reason != "not docz's page" {
		t.Errorf("got %+v", res)
	}

	rep = export(t, rp, c, ExportOptions{Force: true})
	if res := rep.Pages[5]; res.Action != Updated {
		t.Errorf("under force: %s", res.Action)
	}

	pg := c.byTitle(adr1)
	if pg.ParentID != c.byTitle("ADRs").ID || pg.props[propertyKey] == nil {
		t.Error("the adopted page was not moved under its type page with a property")
	}
}

func TestExport_RemovedDocumentIsArchivedAndRestored(t *testing.T) {
	t.Parallel()

	rp, c := exportRepo(t), newFakeClient()
	export(t, rp, c, ExportOptions{})

	src := rp.Path("docs/rfc/0002-second-proposal.md")

	saved, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Remove(src); err != nil {
		t.Fatal(err)
	}

	rep := export(t, rp, c, ExportOptions{})
	last := rep.Pages[len(rep.Pages)-1]

	if last.Title != rfc2 || last.Action != Archived || last.ID != "RFC-0002" {
		t.Fatalf("last result %+v; want RFC-0002 archived", last)
	}

	archive := c.byTitle(archiveTitle)
	if archive == nil || archive.ParentID != c.byTitle("docz").ID || c.byTitle(rfc2).ParentID != archive.ID {
		t.Fatal("RFC-0002 is not under Archive under the parent")
	}

	if rep := export(t, rp, c, ExportOptions{}); rep.Count(Archived) != 0 {
		t.Errorf("a second run archived again:\n%s", actions(&rep))
	}

	writeDoc(t, rp, "docs/rfc/0002-second-proposal.md", string(saved))

	rep = export(t, rp, c, ExportOptions{})
	if res := rep.Pages[3]; res.Title != rfc2 || res.Action != Updated {
		t.Errorf("restored: %+v; want updated", res)
	}

	if c.byTitle(rfc2).ParentID != c.byTitle("RFCs").ID {
		t.Error("the restored page is not back under its type page")
	}
}

func TestExport_NarrowedRunArchivesNothing(t *testing.T) {
	t.Parallel()

	rp, c := exportRepo(t), newFakeClient()
	export(t, rp, c, ExportOptions{})

	if err := os.Remove(rp.Path("docs/rfc/0002-second-proposal.md")); err != nil {
		t.Fatal(err)
	}

	rep := export(t, rp, c, ExportOptions{Types: []string{"adr"}})
	wantActions(t, &rep, "docz=unchanged", "ADRs=unchanged", adr1+"=unchanged")

	// The RFC index still names RFC-0002, whose link now has no page to go
	// to, so the index page itself changes.
	rep = export(t, rp, c, ExportOptions{IDs: []string{"RFC-0001"}})
	wantActions(t, &rep, "docz=unchanged", "RFCs=updated", rfc1+"=unchanged")

	if c.byTitle(archiveTitle) != nil {
		t.Error("a narrowed run created Archive")
	}
}

func TestExport_ReadsOnlyThroughFS(t *testing.T) {
	t.Parallel()

	built := exportRepo(t)
	rp := &repo.Repo{Root: filepath.Join(t.TempDir(), "nowhere"), Cfg: built.Cfg}
	c := newFakeClient()

	rep := export(t, rp, c, ExportOptions{FS: mapFS(t, built.Root)})

	wantActions(t, &rep,
		"docz=created", "RFCs=created", rfc1+"=created", rfc2+"=created",
		"ADRs=created", adr1+"=created", "Design=created", "Implementation Plans=created", "Investigations=created")
}

func TestExport_IDWithoutPrefixIsUnknownType(t *testing.T) {
	t.Parallel()

	rp, c := exportRepo(t), newFakeClient()

	_, err := Export(t.Context(), rp, ExportOptions{Client: c, IDs: []string{"RFC0001"}})

	var ut *repo.UnknownTypeError
	if !errors.As(err, &ut) || ut.Token != "RFC0001" {
		t.Errorf("err %v; want repo.UnknownTypeError for RFC0001", err)
	}
}

func TestExport_UnknownIDIsNotFound(t *testing.T) {
	t.Parallel()

	rp, c := exportRepo(t), newFakeClient()

	_, err := Export(t.Context(), rp, ExportOptions{Client: c, IDs: []string{"RFC-0099"}})

	var nf *repo.NotFoundError
	if !errors.As(err, &nf) {
		t.Errorf("err %v; want repo.NotFoundError", err)
	}

	if w := c.takeWrites(); len(w) != 0 {
		t.Errorf("wrote %v", w)
	}
}

func TestExport_Exclude(t *testing.T) {
	t.Parallel()

	rp, c := exportRepo(t), newFakeClient()
	rp.Cfg.Sync.Confluence.Exclude = []string{"rfc"}
	rp.Cfg.Sync.Confluence.Types = []string{"rfc", "adr"}

	rep := export(t, rp, c, ExportOptions{})
	wantActions(t, &rep, "docz=created", "ADRs=created", adr1+"=created")
}

func TestExport_APIPages(t *testing.T) {
	t.Parallel()

	for _, on := range []bool{false, true} {
		t.Run(map[bool]string{false: "off", true: "on"}[on], func(t *testing.T) {
			t.Parallel()

			rp, c := exportRepo(t), newFakeClient()
			rp.Cfg.Sync.Confluence.Types = []string{"adr"}
			rp.Cfg.Sync.Confluence.APIPages = on
			rp.Cfg.API = config.APIConfig{
				Enabled:        true,
				LandingPage:    "docs/index.md",
				AdditionalDocs: []string{"DEVELOPMENT.md"},
			}
			writeDoc(t, rp, "docs/index.md", "# Home\n\nSee the [decisions](adr/).\n")
			writeDoc(t, rp, "DEVELOPMENT.md", "# Development\n\nRead [the ADR](docs/adr/0001-a-decision.md).\n")

			rep := export(t, rp, c, ExportOptions{})

			if !on {
				wantActions(t, &rep, "docz=created", "ADRs=created", adr1+"=created")

				return
			}

			wantActions(t, &rep, "docz=created", "ADRs=created", adr1+"=created", "Development=created")

			if body := string(c.byTitle("docz").body); !strings.Contains(body, `ri:content-title="ADRs"`) {
				t.Errorf("the landing page does not link to the type page:\n%s", body)
			}

			if body := string(c.byTitle("Development").body); !strings.Contains(body, `ri:content-title="`+adr1+`"`) {
				t.Errorf("the additional doc does not link to the ADR page:\n%s", body)
			}
		})
	}
}

func TestExport_DryRunWritesNothing(t *testing.T) {
	t.Parallel()

	rp, c := exportRepo(t), newFakeClient()

	rep := export(t, rp, c, ExportOptions{DryRun: true})
	if !rep.DryRun || rep.Count(Created) != len(rep.Pages) {
		t.Errorf("dry run:\n%s", actions(&rep))
	}

	export(t, rp, c, ExportOptions{})
	c.takeWrites()
	appendDoc(t, rp, "docs/adr/0001-a-decision.md", "\nChanged.\n")

	rep = export(t, rp, c, ExportOptions{DryRun: true})
	if rep.Count(Updated) != 1 {
		t.Errorf("dry run over a change:\n%s", actions(&rep))
	}

	if w := c.takeWrites(); len(w) != 0 {
		t.Errorf("dry run wrote %v", w)
	}
}

func TestExport_CancelledAfterFirstPage(t *testing.T) {
	t.Parallel()

	rp, c := exportRepo(t), newFakeClient()

	ctx, cancel := context.WithCancel(t.Context())
	ctx = WithHooks(ctx, &Hooks{PageDone: func(PageResult) { cancel() }})

	rep, err := Export(ctx, rp, ExportOptions{Client: c})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err %v; want context.Canceled", err)
	}

	if len(rep.Pages) != 1 || c.byTitle("docz") == nil {
		t.Errorf("report %s; want the first page written and kept", actions(&rep))
	}
}

func TestExport_FailedPageDoesNotStopTheRun(t *testing.T) {
	t.Parallel()

	rp, c := exportRepo(t), newFakeClient()
	export(t, rp, c, ExportOptions{})

	for _, rel := range []string{"docs/rfc/0001-first-proposal.md", "docs/adr/0001-a-decision.md"} {
		appendDoc(t, rp, rel, "\nChanged.\n")
	}

	c.fail = "update " + rfc1

	rep, err := Export(t.Context(), rp, ExportOptions{Client: c})

	var pe *PageError
	if !errors.As(err, &pe) || pe.Title != rfc1 || !errors.Is(err, errFake) {
		t.Errorf("err %v; want the RFC-0001 failure", err)
	}

	if rep.Pages[2].Action != Failed || rep.Pages[5].Action != Updated {
		t.Errorf("report:\n%s", actions(&rep))
	}
}

func TestExport_Config(t *testing.T) {
	t.Parallel()

	rp := exportRepo(t)
	rp.Cfg.Sync.Confluence.Enabled = false

	_, err := Export(t.Context(), rp, ExportOptions{Client: newFakeClient()})
	if !errors.Is(err, ErrConfig) {
		t.Errorf("err %v; want ErrConfig", err)
	}
}

func TestExport_HooksAndBannerLink(t *testing.T) {
	t.Parallel()

	rp, c := exportRepo(t), newFakeClient()

	var done []string

	ctx := WithHooks(t.Context(), &Hooks{PageDone: func(r PageResult) { done = append(done, r.Title) }})
	resolve := func(from, href string) LinkTarget {
		return LinkTarget{
			URL: "https://github.com/o/r/blob/main/" + filepath.ToSlash(filepath.Join(filepath.Dir(from), href)),
		}
	}

	rep, err := Export(ctx, rp, ExportOptions{Client: c, Resolve: resolve})
	if err != nil {
		t.Fatal(err)
	}

	if len(done) != len(rep.Pages) {
		t.Errorf("PageDone fired %d times for %d pages", len(done), len(rep.Pages))
	}

	if body := string(c.byTitle(rfc1).body); !strings.Contains(
		body,
		"https://github.com/o/r/blob/main/docs/rfc/0001-first-proposal.md",
	) {
		t.Errorf("the banner does not link the source:\n%s", body)
	}
}

func TestExport_ForeignPropertyIsSkippedAndNeverArchived(t *testing.T) {
	t.Parallel()

	rp, c := exportRepo(t), newFakeClient()
	export(t, rp, c, ExportOptions{})

	// A page another tool marked with a docz property of its own shape: a
	// document of ours by title, and one that is nobody's document.
	for _, title := range []string{rfc1, "Specimen"} {
		pg := c.byTitle(title)
		if pg == nil {
			created, err := c.CreatePage(t.Context(), &NewPage{
				SpaceID: "space-DOCZ", ParentID: c.byTitle("docz").ID, Title: title, Body: []byte("<p/>"),
			})
			if err != nil {
				t.Fatal(err)
			}

			pg = c.byTitle(created.Title)
		}

		pg.props[propertyKey] = &Property{ID: "x", Key: propertyKey, Value: []byte(`{"id":"` + title + `","hash":"abc"}`), Version: 1}
	}

	appendDoc(t, rp, "docs/rfc/0001-first-proposal.md", "\nChanged.\n")

	rep := export(t, rp, c, ExportOptions{})
	if res := rep.Pages[2]; res.Action != Skipped || res.Reason != "its docz property was not written by docz export" {
		t.Errorf("got %s %q", res.Action, res.Reason)
	}

	if n := rep.Count(Archived); n != 0 {
		t.Errorf("archived %d pages with a foreign property:\n%s", n, actions(&rep))
	}

	rep = export(t, rp, c, ExportOptions{Force: true})
	if res := rep.Pages[2]; res.Action != Updated {
		t.Errorf("under force: %s", res.Action)
	}

	if c.byTitle(archiveTitle) != nil {
		t.Error("Archive was created")
	}
}

func TestExport_SkippedPageIsNeverArchived(t *testing.T) {
	t.Parallel()

	rp, c := exportRepo(t), newFakeClient()
	export(t, rp, c, ExportOptions{})

	// A valid docz property naming an id that is no longer exported, on a
	// page whose title this run still writes: the title wins.
	pg := c.byTitle(rfc2)
	pg.props[propertyKey].Value = []byte(`{"id":"RFC-0999","source":"x","hash":"h","version":1,"docz":"t"}`)

	rep := export(t, rp, c, ExportOptions{})
	if n := rep.Count(Archived); n != 0 {
		t.Errorf("archived a page this run writes:\n%s", actions(&rep))
	}
}

func TestExport_ParentPageStaysWhereItIs(t *testing.T) {
	t.Parallel()

	rp, c := exportRepo(t), newFakeClient()
	export(t, rp, c, ExportOptions{})

	// Somebody files the parent under the space homepage.
	c.byTitle("docz").ParentID = "homepage"
	c.takeWrites()

	rep := export(t, rp, c, ExportOptions{})
	if res := rep.Pages[0]; res.Action != Unchanged {
		t.Errorf("parent: %s; want unchanged where it is", res.Action)
	}

	if w := c.takeWrites(); len(w) != 0 {
		t.Errorf("wrote %v", w)
	}
}

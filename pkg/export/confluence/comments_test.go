package confluence

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestCarryMarkers_Corpus runs each testdata/comments case: a current body
// carrying markers and a new render, against a golden of the output and
// what was kept and lost.
func TestCarryMarkers_Corpus(t *testing.T) {
	t.Parallel()

	cases, err := filepath.Glob(filepath.Join("testdata", "comments", "*.current.xml"))
	if err != nil || len(cases) < 8 {
		t.Fatalf("comment corpus: %d cases, %v", len(cases), err)
	}

	for _, current := range cases {
		name := strings.TrimSuffix(filepath.Base(current), ".current.xml")

		t.Run(name, func(t *testing.T) {
			t.Parallel()

			cur := readFile(t, current)
			render := readFile(t, filepath.Join("testdata", "comments", name+".render.xml"))
			before := bytes.Clone(render)

			out, placed, lost := carryMarkers(render, collectMarkers(cur))
			kept := len(placed)

			if !bytes.Equal(render, before) {
				t.Error("carryMarkers modified its input")
			}

			if err := wellFormed(out); err != nil {
				t.Errorf("output not well-formed: %v", err)
			}

			got := fmt.Sprintf("%s---\nkept: %d\nlost: %q\n", out, kept, texts(lost))
			checkGoldenFile(t, filepath.Join("testdata", "comments", name+".golden"), []byte(got))
		})
	}
}

func readFile(t *testing.T, path string) []byte {
	t.Helper()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	return b
}

// checkGoldenFile compares got with the file at path, or rewrites it under
// -update.
func checkGoldenFile(t *testing.T, path string, got []byte) {
	t.Helper()

	if *update {
		if err := os.WriteFile(path, got, 0o600); err != nil {
			t.Fatal(err)
		}

		return
	}

	if want := readFile(t, path); !bytes.Equal(got, want) {
		t.Errorf("%s mismatch:\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}

func TestCollectMarkers(t *testing.T) {
	t.Parallel()

	body := []byte(`<p><ac:inline-comment-marker ac:ref="a">one <em>two</em></ac:inline-comment-marker> and ` +
		`<ac:inline-comment-marker ac:ref="b">R&amp;D&nbsp;x</ac:inline-comment-marker>` +
		`<ac:inline-comment-marker ac:ref="">no ref</ac:inline-comment-marker></p>`)

	got := collectMarkers(body)
	want := []anchor{
		{Ref: "a", Quote: "one two", Suffix: " and R&D\u00a0xno ref"},
		{Ref: "b", Quote: "R&D\u00a0x", Prefix: "one two and ", Suffix: "no ref"},
	}

	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("collectMarkers = %q; want %q", got, want)
	}

	if got := collectMarkers([]byte("<p>unclosed")); got != nil {
		t.Errorf("a broken body gave %v", got)
	}
}

// addedMarker matches what carryMarkers inserts.
var addedMarker = regexp.MustCompile(`<ac:inline-comment-marker ac:ref="[^"]*">|</ac:inline-comment-marker>`)

// FuzzCarryMarkers: whatever the document and the comments' text, the
// output is well-formed and is the render plus the markers, byte for byte.
func FuzzCarryMarkers(f *testing.F) {
	f.Add([]byte("# T\n\nWe chose Postgres for the store & the queue.\n"), "Postgres", "store & the")
	f.Add([]byte("Some **bold** text and `code`.\n\n```go\nx := 1\n```\n"), "bold", "x := 1")
	f.Add([]byte("> [!NOTE]\n> A note.\n\n- [ ] task one\n"), "note", "task one")
	f.Add([]byte("a a a\n"), "a", " a")

	f.Fuzz(func(t *testing.T, src []byte, one, two string) {
		r, err := Render(src, RenderOptions{Title: "T"})
		if err != nil {
			t.Skip()
		}

		out, placed, lost := carryMarkers(r.Body, []anchor{{Ref: "r1", Quote: one}, {Ref: "r2", Quote: two}})

		if len(placed)+len(lost) != 2 {
			t.Errorf("placed %d + lost %d; want 2", len(placed), len(lost))
		}

		if err := wellFormed(out); err != nil {
			t.Fatalf("not well-formed: %v\n%s", err, out)
		}

		if stripped := addedMarker.ReplaceAll(out, nil); !bytes.Equal(stripped, r.Body) {
			t.Fatalf("stripping the markers does not give back the render:\n%s\n---\n%s", stripped, r.Body)
		}
	})
}

func TestExport_UpdateCarriesInlineComments(t *testing.T) {
	t.Parallel()

	rp, c := exportRepo(t), newFakeClient()
	src := "docs/rfc/0001-first-proposal.md"
	appendDoc(t, rp, src, "\nA unique phrase and a doomed phrase.\n")
	export(t, rp, c, ExportOptions{})

	page := c.byTitle(rfc1)
	page.body = bytes.Replace(page.body, []byte("unique phrase"),
		[]byte(`<ac:inline-comment-marker ac:ref="c1">unique phrase</ac:inline-comment-marker>`), 1)
	page.body = bytes.Replace(page.body, []byte("doomed phrase"),
		[]byte(`<ac:inline-comment-marker ac:ref="c2">doomed phrase</ac:inline-comment-marker>`), 1)
	c.comment(page.ID, "c1")
	c.comment(page.ID, "c2")

	b := readFile(t, rp.Path(src))
	writeDoc(t, rp, src, strings.Replace(string(b), "a doomed phrase", "nothing else", 1))

	rep := export(t, rp, c, ExportOptions{IDs: []string{"RFC-0001"}})

	res := rep.Pages[2]
	if res.Action != Updated || res.Comments.Kept != 1 || len(res.Comments.Lost) != 1 || res.Comments.Lost[0] != "doomed phrase" {
		t.Fatalf("result %+v; want updated, one kept, doomed phrase lost", res)
	}

	if !bytes.Contains(page.body, []byte(`<ac:inline-comment-marker ac:ref="c1">unique phrase</ac:inline-comment-marker>`)) {
		t.Errorf("the comment's marker did not survive:\n%s", page.body)
	}

	if prop := string(page.props[propertyKey].Value); !strings.Contains(prop, res.Hash) {
		t.Errorf("property %s; want the render's hash %s, before markers", prop, res.Hash)
	}

	if rep := export(t, rp, c, ExportOptions{IDs: []string{"RFC-0001"}}); rep.Pages[2].Action != Unchanged {
		t.Errorf("a commented page reads as changed: %+v", rep.Pages[2])
	}
}

// A comment whose text an update removed keeps its anchor in the page's
// property, finds it again when the text returns, and is forgotten once
// the page no longer lists the comment (INV-0021).
func TestExport_LostAnchorIsRememberedAndRestored(t *testing.T) {
	t.Parallel()

	rp, c := exportRepo(t), newFakeClient()
	src := "docs/rfc/0001-first-proposal.md"
	appendDoc(t, rp, src, "\nWe chose Postgres for the store.\n")
	export(t, rp, c, ExportOptions{})

	page := c.byTitle(rfc1)
	page.body = bytes.Replace(page.body, []byte("Postgres"),
		[]byte(`<ac:inline-comment-marker ac:ref="c1">Postgres</ac:inline-comment-marker>`), 1)
	c.comment(page.ID, "c1")

	rewrite := func(from, to string) {
		t.Helper()

		b := readFile(t, rp.Path(src))
		if !strings.Contains(string(b), from) {
			t.Fatalf("%q not in the document", from)
		}

		writeDoc(t, rp, src, strings.Replace(string(b), from, to, 1))
	}

	one := func() PageResult {
		t.Helper()

		rep := export(t, rp, c, ExportOptions{IDs: []string{"RFC-0001"}})

		return rep.Pages[2]
	}

	// Lost: the sentence is gone, the anchor is remembered.
	rewrite("We chose Postgres for the store.", "The store was decided.")

	res := one()
	if res.Action != Updated || res.Comments.Kept != 0 || fmt.Sprint(res.Comments.Lost) != "[Postgres]" {
		t.Fatalf("after the text left: %+v; want lost [Postgres]", res.Comments)
	}

	prop := string(page.props[propertyKey].Value)
	if !strings.Contains(prop, `"anchors":[{"ref":"c1","quote":"Postgres","prefix":"We chose ","suffix":" for the store."`) {
		t.Fatalf("property after the loss does not remember the anchor: %s", prop)
	}

	if got := c.reads; len(got) != 1 || got[0] != "inline-comments "+page.ID {
		t.Errorf("reads %v; want one inline-comments read for the lost anchor", got)
	}

	// Restored: the text is back, the marker goes back under the same ref,
	// and the anchor's context follows the new placement.
	rewrite("The store was decided.", "Later we chose Postgres for the store, as decided.")

	res = one()
	if res.Action != Updated || res.Comments.Kept != 1 || fmt.Sprint(res.Comments.Restored) != "[Postgres]" || res.Comments.Lost != nil {
		t.Fatalf("after the text returned: %+v; want one kept, restored [Postgres]", res.Comments)
	}

	if !bytes.Contains(page.body, []byte(`<ac:inline-comment-marker ac:ref="c1">Postgres</ac:inline-comment-marker>`)) {
		t.Errorf("the marker was not written back:\n%s", page.body)
	}

	if prop := string(page.props[propertyKey].Value); !strings.Contains(prop, `"prefix":"Later we chose "`) {
		t.Errorf("property after the restore keeps the old context: %s", prop)
	}

	// Carried: an ordinary update neither restores nor loses it, and reads
	// no comment list.
	c.reads = nil
	rewrite("as decided.", "as decided, twice.")

	if res = one(); res.Comments.Kept != 1 || res.Comments.Restored != nil || res.Comments.Lost != nil || c.reads != nil {
		t.Errorf("an ordinary update: %+v, reads %v; want one kept and no reads", res.Comments, c.reads)
	}

	// Forgotten: lost again, and the comment was deleted in Confluence.
	c.uncomment(page.ID, "c1")
	rewrite("Later we chose Postgres for the store, as decided, twice.", "Gone.")

	if res = one(); res.Comments.Kept != 0 || res.Comments.Lost != nil {
		t.Errorf("after the comment was deleted: %+v; want nothing lost", res.Comments)
	}

	if prop := string(page.props[propertyKey].Value); strings.Contains(prop, "anchors") {
		t.Errorf("property still remembers a deleted comment's anchor: %s", prop)
	}
}

// A Client without CommentLister never forgets an anchor.
func TestExport_ClientWithoutListerKeepsAnchors(t *testing.T) {
	t.Parallel()

	rp, c := exportRepo(t), newFakeClient()
	src := "docs/rfc/0001-first-proposal.md"
	appendDoc(t, rp, src, "\nWe chose Postgres for the store.\n")
	export(t, rp, c, ExportOptions{})

	page := c.byTitle(rfc1)
	page.body = bytes.Replace(page.body, []byte("Postgres"),
		[]byte(`<ac:inline-comment-marker ac:ref="c1">Postgres</ac:inline-comment-marker>`), 1)

	b := readFile(t, rp.Path(src))
	writeDoc(t, rp, src, strings.Replace(string(b), "We chose Postgres for the store.", "Gone.", 1))

	rep, err := Export(t.Context(), rp, ExportOptions{Client: noLister{c}, Version: "test", IDs: []string{"RFC-0001"}})
	if err != nil {
		t.Fatal(err)
	}

	if res := rep.Pages[2]; fmt.Sprint(res.Comments.Lost) != "[Postgres]" {
		t.Fatalf("%+v; want lost [Postgres]", res.Comments)
	}

	if prop := string(page.props[propertyKey].Value); !strings.Contains(prop, `"anchors":[{"ref":"c1"`) {
		t.Errorf("property forgot the anchor without a lister: %s", prop)
	}
}

// noLister is the fake with its CommentLister hidden: the interface has
// only Client's methods, so the type assertion in listed fails.
type noLister struct {
	Client
}

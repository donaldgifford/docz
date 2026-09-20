package repo

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
)

// coreWriteDoc writes a minimal document with the given id into a type
// directory under the repo, creating the directory as needed.
func coreWriteDoc(t *testing.T, r *Repo, typeName, filename, id, title string) {
	t.Helper()

	dir := r.TypeDir(typeName)
	if err := os.MkdirAll(dir, config.DirMode); err != nil {
		t.Fatal(err)
	}

	body := fmt.Sprintf("---\nid: %s\ntitle: %q\nstatus: Draft\nauthor: T\ncreated: 2026-09-20\n---\n\n# %s\n",
		id, title, title)

	if err := os.WriteFile(filepath.Join(dir, filename), []byte(body), config.FileMode); err != nil {
		t.Fatal(err)
	}
}

func TestScan(t *testing.T) {
	t.Parallel()

	t.Run("documents come back in id order", func(t *testing.T) {
		t.Parallel()

		r := coreRepo(t, t.TempDir())

		coreWriteDoc(t, r, "rfc", "0002-second.md", "RFC-0002", "Second")
		coreWriteDoc(t, r, "rfc", "0001-first.md", "RFC-0001", "First")

		docs, err := r.Scan(t.Context(), "rfc")
		if err != nil {
			t.Fatalf("Scan = %v, want nil", err)
		}

		if len(docs) != 2 {
			t.Fatalf("got %d documents, want 2", len(docs))
		}

		if docs[0].ID != "RFC-0001" || docs[1].ID != "RFC-0002" {
			t.Errorf("ids = %q, %q, want RFC-0001, RFC-0002", docs[0].ID, docs[1].ID)
		}
	})

	t.Run("a missing directory is no documents", func(t *testing.T) {
		t.Parallel()

		r := coreRepo(t, t.TempDir())

		// Nothing to do is not a failure: a type nobody has written to yet
		// is the normal state of a fresh repo, and erroring here would make
		// a no-argument update fail on the first empty type.
		docs, err := r.Scan(t.Context(), "rfc")
		if err != nil {
			t.Fatalf("Scan on a missing directory = %v, want nil", err)
		}

		if len(docs) != 0 {
			t.Errorf("got %d documents, want none", len(docs))
		}
	})

	t.Run("an alias resolves", func(t *testing.T) {
		t.Parallel()

		r := coreRepo(t, t.TempDir())
		coreWriteDoc(t, r, "investigation", "0001-x.md", "INV-0001", "X")

		docs, err := r.Scan(t.Context(), "inv")
		if err != nil {
			t.Fatalf("Scan(inv) = %v, want nil", err)
		}

		if len(docs) != 1 {
			t.Errorf("got %d documents, want 1", len(docs))
		}
	})

	t.Run("a disabled type is an error, not an empty slice", func(t *testing.T) {
		t.Parallel()

		cfg := config.DefaultConfig()
		tc := cfg.Types["rfc"]
		tc.Enabled = false
		cfg.Types["rfc"] = tc

		r := &Repo{Root: t.TempDir(), Cfg: &cfg}

		// Returning nil, nil here is what made "no documents" and "switched
		// off" indistinguishable, and cmd/update.go's "type disabled,
		// skipping" branch needs to tell them apart.
		_, err := r.Scan(t.Context(), "rfc")

		var disabled *TypeDisabledError
		if !errors.As(err, &disabled) {
			t.Fatalf("err = %v, want *TypeDisabledError", err)
		}
	})

	t.Run("an unknown type is UnknownTypeError", func(t *testing.T) {
		t.Parallel()

		r := coreRepo(t, t.TempDir())

		_, err := r.Scan(t.Context(), "nonsense")

		var unknown *UnknownTypeError
		if !errors.As(err, &unknown) {
			t.Fatalf("err = %v, want *UnknownTypeError", err)
		}

		if !errors.Is(err, config.ErrUnknownType) {
			t.Error("the frozen sentinel no longer answers errors.Is")
		}
	})

	t.Run("a cancelled context stops the scan", func(t *testing.T) {
		t.Parallel()

		r := coreRepo(t, t.TempDir())
		coreWriteDoc(t, r, "rfc", "0001-x.md", "RFC-0001", "X")

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		if _, err := r.Scan(ctx, "rfc"); !errors.Is(err, context.Canceled) {
			t.Errorf("Scan on a cancelled context = %v, want context.Canceled", err)
		}
	})

	t.Run("the scan hooks fire around the read", func(t *testing.T) {
		t.Parallel()

		r := coreRepo(t, t.TempDir())
		coreWriteDoc(t, r, "rfc", "0001-x.md", "RFC-0001", "X")

		var events []string

		ctx := WithHooks(t.Context(), &Hooks{
			ScanStart: func(typeName, _ string) { events = append(events, "start:"+typeName) },
			ScanDone:  func(typeName string, n int) { events = append(events, fmt.Sprintf("done:%s:%d", typeName, n)) },
		})

		if _, err := r.Scan(ctx, "rfc"); err != nil {
			t.Fatalf("Scan = %v, want nil", err)
		}

		want := []string{"start:rfc", "done:rfc:1"}
		if len(events) != len(want) {
			t.Fatalf("events = %v, want %v", events, want)
		}

		for i := range want {
			if events[i] != want[i] {
				t.Errorf("event %d = %q, want %q", i, events[i], want[i])
			}
		}
	})
}

func TestList(t *testing.T) {
	t.Parallel()

	t.Run("nil types covers every enabled type", func(t *testing.T) {
		t.Parallel()

		r := coreRepo(t, t.TempDir())

		coreWriteDoc(t, r, "rfc", "0001-a.md", "RFC-0001", "A")
		coreWriteDoc(t, r, "adr", "0001-b.md", "ADR-0001", "B")

		entries, err := r.List(t.Context(), nil)
		if err != nil {
			t.Fatalf("List = %v, want nil", err)
		}

		if len(entries) != 2 {
			t.Fatalf("got %d entries, want 2", len(entries))
		}

		// Built-ins come back in registry-declaration order, which is what
		// makes no-argument output stable across runs.
		if entries[0].Type != "rfc" || entries[1].Type != "adr" {
			t.Errorf("types = %q, %q, want rfc, adr", entries[0].Type, entries[1].Type)
		}
	})

	t.Run("a custom type is included", func(t *testing.T) {
		t.Parallel()

		cfg := config.DefaultConfig()
		cfg.Types["frameworks"] = config.TypeConfig{
			Enabled:  true,
			Dir:      "frameworks",
			IDPrefix: "FW",
			IDWidth:  4,
			Statuses: []string{"Draft"},
		}

		r := &Repo{Root: t.TempDir(), Cfg: &cfg}
		coreWriteDoc(t, r, "frameworks", "0001-x.md", "FW-0001", "X")

		entries, err := r.List(t.Context(), nil)
		if err != nil {
			t.Fatalf("List = %v, want nil", err)
		}

		if len(entries) != 1 || entries[0].Type != "frameworks" {
			t.Fatalf("entries = %+v, want one frameworks entry", entries)
		}
	})

	t.Run("Path is repo-relative", func(t *testing.T) {
		t.Parallel()

		r := coreRepo(t, t.TempDir())
		coreWriteDoc(t, r, "rfc", "0001-a.md", "RFC-0001", "A")

		entries, err := r.List(t.Context(), []string{"rfc"})
		if err != nil {
			t.Fatalf("List = %v, want nil", err)
		}

		want := filepath.Join("docs", "rfc", "0001-a.md")
		if entries[0].Path != want {
			t.Errorf("Path = %q, want %q", entries[0].Path, want)
		}
	})

	t.Run("a cancelled context returns what was gathered", func(t *testing.T) {
		t.Parallel()

		r := coreRepo(t, t.TempDir())
		coreWriteDoc(t, r, "rfc", "0001-a.md", "RFC-0001", "A")

		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		entries, err := r.List(ctx, nil)
		if !errors.Is(err, context.Canceled) {
			t.Errorf("err = %v, want context.Canceled", err)
		}

		// Partial rather than nil: a caller reporting progress should not
		// lose the types that finished.
		if entries != nil {
			t.Errorf("entries = %+v, want nil when nothing was read", entries)
		}
	})
}

func TestFind(t *testing.T) {
	t.Parallel()

	t.Run("the id prefix picks the type", func(t *testing.T) {
		t.Parallel()

		r := coreRepo(t, t.TempDir())
		coreWriteDoc(t, r, "impl", "0018-x.md", "IMPL-0018", "X")

		entry, err := r.Find(t.Context(), "IMPL-0018")
		if err != nil {
			t.Fatalf("Find = %v, want nil", err)
		}

		if entry.Type != "impl" || entry.ID != "IMPL-0018" {
			t.Errorf("entry = {%q, %q}, want {impl, IMPL-0018}", entry.Type, entry.ID)
		}
	})

	t.Run("an id with no prefix is an unknown type", func(t *testing.T) {
		t.Parallel()

		r := coreRepo(t, t.TempDir())

		// Guessing which type to search would give a wrong answer rather
		// than no answer, so the whole id is reported as the bad token.
		_, err := r.Find(t.Context(), "0018")

		var unknown *UnknownTypeError
		if !errors.As(err, &unknown) {
			t.Fatalf("err = %v, want *UnknownTypeError", err)
		}

		if unknown.Token != "0018" {
			t.Errorf("Token = %q, want the whole id", unknown.Token)
		}
	})

	t.Run("a missing id is NotFoundError", func(t *testing.T) {
		t.Parallel()

		r := coreRepo(t, t.TempDir())
		coreWriteDoc(t, r, "impl", "0001-x.md", "IMPL-0001", "X")

		_, err := r.Find(t.Context(), "IMPL-9999")

		var missing *NotFoundError
		if !errors.As(err, &missing) {
			t.Fatalf("err = %v, want *NotFoundError", err)
		}

		if missing.Type != "impl" || missing.ID != "IMPL-9999" {
			t.Errorf("error = {%q, %q}, want {impl, IMPL-9999}", missing.Type, missing.ID)
		}
	})
}

func TestFindIn(t *testing.T) {
	t.Parallel()

	r := coreRepo(t, t.TempDir())
	coreWriteDoc(t, r, "adr", "0003-x.md", "ADR-0003", "X")

	t.Run("an exact id matches", func(t *testing.T) {
		t.Parallel()

		entry, err := r.FindIn(t.Context(), "adr", "ADR-0003")
		if err != nil {
			t.Fatalf("FindIn = %v, want nil", err)
		}

		want := filepath.Join("docs", "adr", "0003-x.md")
		if entry.Path != want {
			t.Errorf("Path = %q, want %q", entry.Path, want)
		}
	})

	t.Run("the comparison is case-sensitive", func(t *testing.T) {
		t.Parallel()

		// An id is a key. A lenient match would let two documents answer to
		// the same name (DESIGN-0005 Decision 3).
		_, err := r.FindIn(t.Context(), "adr", "adr-0003")

		var missing *NotFoundError
		if !errors.As(err, &missing) {
			t.Errorf("err = %v, want *NotFoundError for a case-folded id", err)
		}
	})
}

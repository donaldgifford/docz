package repo_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/index"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
)

// initTestCustomType is a custom type with nothing embedded for it: no index
// header, no body template, no schema. Init has to scaffold it from the
// generic tier, which is the path that used to fail embedded-only.
const initTestCustomType = "frameworks"

// initTestRepo returns a Repo over an empty directory.
//
// withCustom adds the custom type, which is the tier-3 rendered header. The
// issue #99 reach over a header that ends with its own pair comes from
// investigation, an enabled built-in -- index_plan.md was the other one and
// went away with the type (ADR-0003).
func initTestRepo(t *testing.T, withCustom bool) (*repo.Repo, string) {
	t.Helper()

	root := t.TempDir()
	cfg := config.DefaultConfig()

	if withCustom {
		cfg.Types[initTestCustomType] = config.TypeConfig{
			Enabled:  true,
			Dir:      initTestCustomType,
			IDPrefix: "FW",
			IDWidth:  4,
			Statuses: []string{"Draft", "Active"},
		}
	}

	return &repo.Repo{Root: root, Cfg: &cfg}, root
}

// initTestSnapshot reads every file under root, keyed by its path relative to
// root, so a second run can be compared against the first byte for byte.
func initTestSnapshot(t *testing.T, root string) map[string]string {
	t.Helper()

	files := make(map[string]string)

	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if entry.IsDir() {
			return nil
		}

		body, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}

		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}

		files[rel] = string(body)

		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", root, err)
	}

	return files
}

// initTestWantFiles is the report Init must produce for cfg: the config file
// first, then one README per enabled type in EnabledTypes order.
func initTestWantFiles(cfg *config.Config) []string {
	enabled := cfg.EnabledTypes()

	want := make([]string, 0, len(enabled)+1)
	want = append(want, config.ConfigFileName)

	for _, typeName := range enabled {
		want = append(want, filepath.Join(cfg.TypeDir(typeName), config.IndexFileName))
	}

	return want
}

func TestInit_CreatesEverything(t *testing.T) {
	t.Parallel()

	r, root := initTestRepo(t, true)

	report, err := r.Init(t.Context(), repo.InitOptions{})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	want := initTestWantFiles(r.Cfg)

	got := make([]string, 0, len(report.Files))
	for i := range report.Files {
		got = append(got, report.Files[i].Path)
	}

	// The order is part of the report: a caller prints it as it stands, and a
	// consumer diffing two runs needs the entries to line up.
	if !slices.Equal(got, want) {
		t.Fatalf("report paths =\n%v\nwant\n%v", got, want)
	}

	for i := range report.Files {
		file := &report.Files[i]

		if file.Action != repo.InitCreated {
			t.Errorf("%s: action = %v, want created", file.Path, file.Action)
		}

		if _, err := os.Stat(filepath.Join(root, file.Path)); err != nil {
			t.Errorf("%s reported but not on disk: %v", file.Path, err)
		}
	}

	// A directory is not an InitFile, but it does have to be there: a type
	// nobody has written to yet still gets its directory.
	for _, typeName := range r.Cfg.EnabledTypes() {
		dir := filepath.Join(root, r.Cfg.TypeDir(typeName))

		info, err := os.Stat(dir)
		if err != nil {
			t.Errorf("%s: %v", dir, err)

			continue
		}

		if !info.IsDir() {
			t.Errorf("%s is not a directory", dir)
		}
	}
}

// TestInit_ExactlyOneMarkerPair is issue #99: index.Scaffold and nothing else.
// Two of the embedded index headers end with their own marker pair, and
// appending a second one gave every new repository a permanently empty pair
// that no later splice ever touches.
func TestInit_ExactlyOneMarkerPair(t *testing.T) {
	t.Parallel()

	r, root := initTestRepo(t, true)

	report, err := r.Init(t.Context(), repo.InitOptions{})
	if err != nil {
		t.Fatalf("Init: %v", err)
	}

	seen := 0

	for i := range report.Files {
		file := &report.Files[i]
		if filepath.Base(file.Path) != config.IndexFileName {
			continue
		}

		seen++

		body, err := os.ReadFile(filepath.Join(root, file.Path))
		if err != nil {
			t.Fatalf("read %s: %v", file.Path, err)
		}

		if n := strings.Count(string(body), index.BeginMarker); n != 1 {
			t.Errorf("%s has %d begin markers, want 1:\n%s", file.Path, n, body)
		}

		if n := strings.Count(string(body), index.EndMarker); n != 1 {
			t.Errorf("%s has %d end markers, want 1:\n%s", file.Path, n, body)
		}
	}

	// Every built-in plus the custom type, so a header that grows its own pair
	// later cannot slip past by being on a type the loop never reached.
	if want := len(config.DocTypeNames()) + 1; seen != want {
		t.Errorf("checked %d READMEs, want %d", seen, want)
	}
}

func TestInit_SecondRunSkipsEverything(t *testing.T) {
	t.Parallel()

	r, root := initTestRepo(t, true)

	if _, err := r.Init(t.Context(), repo.InitOptions{}); err != nil {
		t.Fatalf("first Init: %v", err)
	}

	before := initTestSnapshot(t, root)

	report, err := r.Init(t.Context(), repo.InitOptions{})
	if err != nil {
		t.Fatalf("second Init: %v", err)
	}

	for i := range report.Files {
		if report.Files[i].Action != repo.InitSkipped {
			t.Errorf("%s: action = %v, want skipped", report.Files[i].Path, report.Files[i].Action)
		}
	}

	after := initTestSnapshot(t, root)

	if len(before) != len(after) {
		t.Fatalf("file count changed: %d -> %d", len(before), len(after))
	}

	for path, body := range before {
		if after[path] != body {
			t.Errorf("%s was rewritten by the second run", path)
		}
	}
}

func TestInit_ForceOverwrites(t *testing.T) {
	t.Parallel()

	r, root := initTestRepo(t, false)

	if _, err := r.Init(t.Context(), repo.InitOptions{}); err != nil {
		t.Fatalf("first Init: %v", err)
	}

	// Something a user would have edited, so the overwrite is observable
	// rather than only reported.
	readme := filepath.Join(root, r.Cfg.TypeDir("adr"), config.IndexFileName)
	if err := os.WriteFile(readme, []byte("mine\n"), config.FileMode); err != nil {
		t.Fatalf("write %s: %v", readme, err)
	}

	report, err := r.Init(t.Context(), repo.InitOptions{Force: true})
	if err != nil {
		t.Fatalf("forced Init: %v", err)
	}

	for i := range report.Files {
		if report.Files[i].Action != repo.InitOverwritten {
			t.Errorf("%s: action = %v, want overwritten", report.Files[i].Path, report.Files[i].Action)
		}
	}

	body, err := os.ReadFile(readme)
	if err != nil {
		t.Fatalf("read %s: %v", readme, err)
	}

	if string(body) == "mine\n" {
		t.Error("README was reported overwritten but still holds the edited content")
	}
}

func TestInit_CancelledContext(t *testing.T) {
	t.Parallel()

	t.Run("already cancelled writes nothing", func(t *testing.T) {
		t.Parallel()

		r, root := initTestRepo(t, false)

		ctx, cancel := context.WithCancel(t.Context())
		cancel()

		report, err := r.Init(ctx, repo.InitOptions{})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}

		if len(report.Files) != 0 {
			t.Errorf("report has %d files, want none", len(report.Files))
		}

		if files := initTestSnapshot(t, root); len(files) != 0 {
			t.Errorf("wrote %v, want nothing", files)
		}
	})

	t.Run("cancelled mid-run returns the partial report", func(t *testing.T) {
		t.Parallel()

		r, _ := initTestRepo(t, false)

		// Cancelling from the hook is what makes this deterministic: hooks run
		// synchronously on the calling goroutine, so the cancel lands between
		// the config file and the first type.
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()

		ctx = repo.WithHooks(ctx, &repo.Hooks{
			FileWritten: func(_ string, kind repo.FileKind) {
				if kind == repo.FileConfig {
					cancel()
				}
			},
		})

		report, err := r.Init(ctx, repo.InitOptions{})
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("err = %v, want context.Canceled", err)
		}

		// What was written stays written, and the report says so.
		if len(report.Files) != 1 || report.Files[0].Path != config.ConfigFileName {
			t.Fatalf("report = %+v, want just %s", report.Files, config.ConfigFileName)
		}
	})
}

func TestInit_FiresHooks(t *testing.T) {
	t.Parallel()

	r, _ := initTestRepo(t, false)

	var written []string

	ctx := repo.WithHooks(t.Context(), &repo.Hooks{
		FileWritten: func(path string, _ repo.FileKind) {
			written = append(written, path)
		},
	})

	if _, err := r.Init(ctx, repo.InitOptions{}); err != nil {
		t.Fatalf("Init: %v", err)
	}

	if !slices.Equal(written, initTestWantFiles(r.Cfg)) {
		t.Errorf("FileWritten saw\n%v\nwant\n%v", written, initTestWantFiles(r.Cfg))
	}

	var skipped []string

	ctx = repo.WithHooks(t.Context(), &repo.Hooks{
		FileSkipped: func(path string, reason repo.SkipReason) {
			if reason == repo.SkipExists {
				skipped = append(skipped, path)
			}
		},
	})

	if _, err := r.Init(ctx, repo.InitOptions{}); err != nil {
		t.Fatalf("second Init: %v", err)
	}

	if !slices.Equal(skipped, initTestWantFiles(r.Cfg)) {
		t.Errorf("FileSkipped saw\n%v\nwant\n%v", skipped, initTestWantFiles(r.Cfg))
	}
}

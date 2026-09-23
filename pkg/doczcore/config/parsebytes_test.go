package config

import (
	"bytes"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// parseBytesCases are the .docz.yaml shapes that exercise each normaliser
// ParseBytes shares with Load: the INV-0003 types-replace rule, the
// changelog backfill, the api exclude collapse, a custom type, and nothing.
var parseBytesCases = []struct {
	name string
	yaml string
}{
	{
		name: "types block replaces the default type map",
		yaml: "types:\n  adr:\n    enabled: true\n  impl:\n    enabled: true\n",
	},
	{
		name: "explicit empty changelog file backfills the default",
		yaml: "changelog:\n  enabled: true\n  file: \"\"\n",
	},
	{
		name: "api exclude trailing slash collapses",
		yaml: "api:\n  enabled: true\n  exclude:\n    - templates/\n    - ./drafts/\n",
	},
	{
		name: "custom type with aliases",
		yaml: "types:\n  frameworks:\n    enabled: true\n    dir: frameworks\n" +
			"    id_prefix: FW\n    id_width: 4\n    statuses: [Draft, Adopted]\n" +
			"    aliases: [fw, framework]\n",
	},
	{
		name: "empty file",
		yaml: "",
	},
}

// TestParseBytes_MatchesLoad pins DESIGN-0016's equivalence: ParseBytes of a
// file's bytes equals Load of a repository holding only that file. Serial,
// because HOME is neutralised with t.Setenv so a developer's global
// ~/.docz.yaml cannot decide the result of Load's merge path.
func TestParseBytes_MatchesLoad(t *testing.T) {
	for _, tc := range parseBytesCases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("HOME", t.TempDir())

			dir := t.TempDir()
			path := filepath.Join(dir, ConfigFileName)
			if err := os.WriteFile(path, []byte(tc.yaml), FileMode); err != nil {
				t.Fatal(err)
			}

			want, err := Load("", dir)
			if err != nil {
				t.Fatalf("Load: %v", err)
			}
			got, err := ParseBytes([]byte(tc.yaml))
			if err != nil {
				t.Fatalf("ParseBytes: %v", err)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("ParseBytes != Load\n got: %+v\nwant: %+v", got, want)
			}

			fromFile, err := Load(path, "")
			if err != nil {
				t.Fatalf("Load(path): %v", err)
			}
			if !reflect.DeepEqual(got, fromFile) {
				t.Errorf("ParseBytes != Load(path, \"\")\n got: %+v\nwant: %+v", got, fromFile)
			}
		})
	}
}

// TestParseBytes_ReadsNoFilesystem proves ParseBytes ignores a global config
// that would change Load's answer — the temp-directory dance it replaces in
// docz-api existed only because Load reads $HOME.
func TestParseBytes_ReadsNoFilesystem(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	global := "docs_dir: from-global\n"
	if err := os.WriteFile(filepath.Join(home, ConfigFileName), []byte(global), FileMode); err != nil {
		t.Fatal(err)
	}
	t.Chdir(t.TempDir())

	got, err := ParseBytes(nil)
	if err != nil {
		t.Fatalf("ParseBytes: %v", err)
	}
	if got.DocsDir == "from-global" {
		t.Fatal("ParseBytes merged $HOME/.docz.yaml")
	}
	if want := DefaultConfig().DocsDir; got.DocsDir != want {
		t.Errorf("DocsDir = %q, want the default %q", got.DocsDir, want)
	}

	// Load over the same HOME does see it, so the check above is not vacuous.
	loaded, err := Load("", t.TempDir())
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if loaded.DocsDir != "from-global" {
		t.Fatalf("control: Load DocsDir = %q, want from-global", loaded.DocsDir)
	}
}

func TestParseBytes_DoesNotModifyInput(t *testing.T) {
	t.Parallel()
	for _, tc := range parseBytesCases {
		in := []byte(tc.yaml)
		before := bytes.Clone(in)
		if _, err := ParseBytes(in); err != nil {
			t.Fatalf("%s: ParseBytes: %v", tc.name, err)
		}
		if !bytes.Equal(in, before) {
			t.Errorf("%s: input modified", tc.name)
		}
	}
}

func TestParseBytes_MalformedYAML(t *testing.T) {
	t.Parallel()
	if _, err := ParseBytes([]byte("types: [unclosed")); err == nil {
		t.Fatal("want an error for malformed YAML")
	}
}

package doczcontract_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"testing"

	doczcfg "github.com/donaldgifford/docz/pkg/doczcore/config"
)

// R11 — the marshaled config shape (upstream DESIGN-0008 clause R11, docz
// v1.2.2): every config field carries a json tag mirroring its yaml tag in
// name and omitempty, so json.Marshal over a post-Load Config emits the
// .docz.yaml key spellings. ingest.Run marshals exactly that value into
// config_snapshot, and docz-site's snapshot gates (changelog:, api:) read
// the yaml spellings — a renamed key, a dropped tag, or a flipped
// omitempty upstream is a consumer-breaking change that must fail here,
// not in a downstream reader. The clause pins key sets and presence
// semantics (order- and formatting-insensitive): the exact key set per
// block on a fully-populated config, the omitempty keys' absence on a
// minimal one, and null (not [] and not absent) for nil slices in
// non-omitempty fields.

// loadSnapshotConfig loads a .docz.yaml with the given body hermetically
// (the loadAPIConfig mold) — the same path production marshals: ingest
// serializes the post-Load config, so the clause must pin Load's output,
// defaults and normalization included, not a hand-built struct literal.
func loadSnapshotConfig(t *testing.T, body string) doczcfg.Config {
	t.Helper()
	t.Setenv("HOME", t.TempDir())

	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, ".docz.yaml"), []byte(body), 0o600); err != nil {
		t.Fatalf("write .docz.yaml: %v", err)
	}
	cfg, err := doczcfg.Load("", repo)
	if err != nil {
		t.Fatalf("config.Load: %v", err)
	}
	return cfg
}

// marshalBlocks marshals cfg and decodes one level: top-level key →
// raw JSON for that block.
func marshalBlocks(t *testing.T, cfg *doczcfg.Config) map[string]json.RawMessage {
	t.Helper()
	b, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	var top map[string]json.RawMessage
	if err := json.Unmarshal(b, &top); err != nil {
		t.Fatalf("decode marshaled config: %v", err)
	}
	return top
}

// objectKeys decodes raw as a JSON object and returns its sorted keys.
func objectKeys(t *testing.T, raw json.RawMessage, label string) []string {
	t.Helper()
	var m map[string]json.RawMessage
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatalf("decode %s as an object: %v (raw %s)", label, err, raw)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}

// fullSnapshotYAML populates every serialized key in every block, so the
// full-fixture test can assert exact key sets — an upstream rename OR an
// additive field both fail loudly, and the bump updates this clause
// deliberately.
const fullSnapshotYAML = `docs_dir: docs
types:
  rfc:
    enabled: true
    dir: rfc
    template: custom.md
    id_prefix: RFC
    id_width: 4
    statuses: [Draft, Accepted]
    status_field: status
    plural_label: RFCs
    aliases: [rfcs]
index:
  auto_update: true
  preserve_header: true
author:
  from_git: true
  default: Jane
wiki:
  auto_update: true
  mkdocs_path: mkdocs.yml
  plugins: [techdocs-core]
  markdown_extensions: [admonition]
  exclude: [templates]
  nav_titles:
    rfc: RFCs
  docs_dir: wikidocs
  repo_url: https://example.com/repo
  site_url: https://example.com
  theme: readthedocs
toc:
  enabled: true
  min_headings: 3
changelog:
  enabled: true
  file: CHANGELOG.md
api:
  enabled: true
  landing_page: docs/index.md
  exclude: [drafts]
  additional_docs: [CONTRIBUTING.md]
`

// minimalSnapshotYAML declares one bare custom type and nothing else, so
// the presence-semantics test sees the omitempty keys absent and the
// non-omitempty nil slices as null.
const minimalSnapshotYAML = `docs_dir: docs
types:
  bare:
    enabled: true
    dir: bare
    id_prefix: B
`

// TestSnapshotKeySpellings pins the exact serialized key set of every
// block on a fully-populated config — the .docz.yaml spellings, snake
// case included (docs_dir, id_prefix, landing_page, additional_docs, …).
func TestSnapshotKeySpellings(t *testing.T) {
	cfg := loadSnapshotConfig(t, fullSnapshotYAML)
	if _, err := cfg.Validate(); err != nil {
		t.Fatalf("Validate: %v", err)
	}
	top := marshalBlocks(t, &cfg)

	topKeys := make([]string, 0, len(top))
	for k := range top {
		topKeys = append(topKeys, k)
	}
	slices.Sort(topKeys)
	wantTop := []string{"api", "author", "changelog", "docs_dir", "index", "toc", "types", "wiki"}
	if !slices.Equal(topKeys, wantTop) {
		t.Fatalf("top-level keys = %v, want %v", topKeys, wantTop)
	}

	blocks := []struct {
		name string
		want []string
	}{
		{"index", []string{"auto_update", "preserve_header"}},
		{"author", []string{"default", "from_git"}},
		{"wiki", []string{
			"auto_update", "docs_dir", "exclude", "markdown_extensions",
			"mkdocs_path", "nav_titles", "plugins", "repo_url", "site_url", "theme",
		}},
		{"toc", []string{"enabled", "min_headings"}},
		{"changelog", []string{"enabled", "file"}},
		{"api", []string{"additional_docs", "enabled", "exclude", "landing_page"}},
	}
	for _, block := range blocks {
		if got := objectKeys(t, top[block.name], block.name); !slices.Equal(got, block.want) {
			t.Errorf("%s keys = %v, want %v", block.name, got, block.want)
		}
	}

	// types is a map of type name → per-type block.
	var types map[string]json.RawMessage
	if err := json.Unmarshal(top["types"], &types); err != nil {
		t.Fatalf("decode types: %v", err)
	}
	wantType := []string{
		"aliases", "dir", "enabled", "id_prefix", "id_width",
		"plural_label", "status_field", "statuses", "template",
	}
	if got := objectKeys(t, types["rfc"], "types.rfc"); !slices.Equal(got, wantType) {
		t.Errorf("types.rfc keys = %v, want %v", got, wantType)
	}
}

// TestSnapshotPresenceSemantics pins the two behaviors snapshot readers
// must rely on: omitempty keys are absent (not empty) when unset, and a
// non-omitempty nil slice serializes as null — not [] and not absent.
// It also pins that a disabled opt-in block still serializes its enabled
// flag: docz-site's gates read changelog.enabled / api.enabled and must
// see an explicit false, never a missing key.
func TestSnapshotPresenceSemantics(t *testing.T) {
	cfg := loadSnapshotConfig(t, minimalSnapshotYAML)
	top := marshalBlocks(t, &cfg)

	// The bare type omits its omitempty keys and nulls its nil statuses.
	var types map[string]map[string]json.RawMessage
	if err := json.Unmarshal(top["types"], &types); err != nil {
		t.Fatalf("decode types: %v", err)
	}
	bare := types["bare"]
	for _, absent := range []string{"aliases", "plural_label"} {
		if _, ok := bare[absent]; ok {
			t.Errorf("types.bare.%s present on a zero value, want absent (omitempty)", absent)
		}
	}
	if string(bare["statuses"]) != "null" {
		t.Errorf("types.bare.statuses = %s, want null (nil non-omitempty slice)", bare["statuses"])
	}
	// template is not omitempty: present even when empty.
	if string(bare["template"]) != `""` {
		t.Errorf("types.bare.template = %s, want \"\" (present, not omitted)", bare["template"])
	}

	// The wiki block's omitempty keys are absent when unset (the defaults
	// leave markdown_extensions and the four scalar overrides zero).
	var wiki map[string]json.RawMessage
	if err := json.Unmarshal(top["wiki"], &wiki); err != nil {
		t.Fatalf("decode wiki: %v", err)
	}
	for _, absent := range []string{"markdown_extensions", "docs_dir", "repo_url", "site_url", "theme"} {
		if _, ok := wiki[absent]; ok {
			t.Errorf("wiki.%s present on a zero value, want absent (omitempty)", absent)
		}
	}

	// A dormant api block still carries its full key set — enabled is an
	// explicit false and the nil slices are null.
	var api map[string]json.RawMessage
	if err := json.Unmarshal(top["api"], &api); err != nil {
		t.Fatalf("decode api: %v", err)
	}
	if string(api["enabled"]) != "false" {
		t.Errorf("api.enabled = %s, want explicit false (the docz-site gate reads it)", api["enabled"])
	}
	for _, field := range []string{"exclude", "additional_docs"} {
		if string(api[field]) != "null" {
			t.Errorf("api.%s = %s, want null (nil non-omitempty slice)", field, api[field])
		}
	}

	// The disabled changelog block likewise serializes an explicit false
	// (plus its defaulted file), never a missing key.
	var changelog map[string]json.RawMessage
	if err := json.Unmarshal(top["changelog"], &changelog); err != nil {
		t.Fatalf("decode changelog: %v", err)
	}
	if string(changelog["enabled"]) != "false" || string(changelog["file"]) != `"CHANGELOG.md"` {
		t.Errorf("changelog = enabled %s file %s, want explicit false + defaulted file",
			changelog["enabled"], changelog["file"])
	}
}

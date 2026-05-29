package config_test

import (
	"reflect"
	"strings"
	"testing"

	"github.com/donaldgifford/docz/internal/config"
	"github.com/donaldgifford/docz/internal/template"
)

// TestDocTypeRegistry_AllHaveEmbeddedTemplate asserts that every doc
// type in the registry has a matching embedded body template
// (`<TemplateName>.md`). This is the contract that lets `docz create`
// render a doc without bespoke per-type plumbing — see DESIGN-0004 §E.
func TestDocTypeRegistry_AllHaveEmbeddedTemplate(t *testing.T) {
	t.Parallel()
	for _, dt := range config.AllDocTypes() {
		if _, err := template.EmbeddedDocumentTemplate(config.DocType(dt.TemplateName)); err != nil {
			t.Errorf(
				"doc type %q has no embedded template %q.md: %v",
				dt.Name, dt.TemplateName, err,
			)
		}
	}
}

// TestDocTypeRegistry_AllHaveEmbeddedIndexHeader asserts that every doc
// type has a matching embedded index header template
// (`index_<TemplateName>.md`) so `docz update` can write the per-type
// README without bespoke per-type plumbing.
func TestDocTypeRegistry_AllHaveEmbeddedIndexHeader(t *testing.T) {
	t.Parallel()
	for _, dt := range config.AllDocTypes() {
		if _, err := template.EmbeddedIndexHeader(dt.TemplateName); err != nil {
			t.Errorf(
				"doc type %q has no embedded index header index_%s.md: %v",
				dt.Name, dt.TemplateName, err,
			)
		}
	}
}

// TestDocTypeRegistry_NoDuplicateNames guards against two registry
// entries claiming the same canonical name — a silent footgun where
// the later entry would shadow the earlier one in maps.
func TestDocTypeRegistry_NoDuplicateNames(t *testing.T) {
	t.Parallel()
	seen := make(map[string]bool, len(config.AllDocTypes()))
	for _, dt := range config.AllDocTypes() {
		if seen[dt.Name] {
			t.Errorf("duplicate canonical name %q in registry", dt.Name)
		}
		seen[dt.Name] = true
	}
}

// TestDocTypeRegistry_NoAliasCollidesWithCanonical guards against an
// alias that shadows a different type's canonical name — e.g. an
// alias "rfc" on the adr entry would route `rfc` lookups to adr.
func TestDocTypeRegistry_NoAliasCollidesWithCanonical(t *testing.T) {
	t.Parallel()
	canonical := make(map[string]bool, len(config.AllDocTypes()))
	for _, dt := range config.AllDocTypes() {
		canonical[dt.Name] = true
	}
	for _, dt := range config.AllDocTypes() {
		for _, a := range dt.Aliases {
			if canonical[a] && a != dt.Name {
				t.Errorf(
					"alias %q on type %q collides with another canonical type name",
					a, dt.Name,
				)
			}
		}
	}
}

// TestDocTypeRegistry_DefaultConfigValidates asserts that the
// TypeConfig produced by each registry entry's DefaultConfig() passes
// Config.Validate() when assembled into a default Config. A registry
// entry that ships a broken default would surface as a startup error
// in every fresh installation.
func TestDocTypeRegistry_DefaultConfigValidates(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	warnings, err := cfg.Validate()
	if err != nil {
		t.Fatalf("DefaultConfig() failed Validate(): %v", err)
	}
	if len(warnings) != 0 {
		t.Errorf("DefaultConfig() emitted unexpected warnings: %v", warnings)
	}
}

// TestDocTypeRegistry_DefaultConfigStatusesNonEmpty asserts that every
// registry entry ships at least one status. Validate enforces this for
// the active Config, but registry entries are the source of those
// defaults so we check them directly too.
func TestDocTypeRegistry_DefaultConfigStatusesNonEmpty(t *testing.T) {
	t.Parallel()
	for _, dt := range config.AllDocTypes() {
		tc := dt.DefaultConfig()
		if len(tc.Statuses) == 0 {
			t.Errorf("doc type %q DefaultConfig() has empty Statuses", dt.Name)
		}
	}
}

// TestDocTypeRegistry_DefaultConfigReturnsFreshSlice asserts that
// DefaultConfig() on a registry entry hands out a distinct Statuses
// slice on each call. The constructor-func shape (DESIGN-0004 §E)
// exists so callers can't mutate one Config and bleed the change into
// another — append-to-shared-slice would defeat the whole point.
func TestDocTypeRegistry_DefaultConfigReturnsFreshSlice(t *testing.T) {
	t.Parallel()
	for _, dt := range config.AllDocTypes() {
		a := dt.DefaultConfig()
		b := dt.DefaultConfig()
		if len(a.Statuses) == 0 {
			continue
		}
		original := a.Statuses[0]
		a.Statuses[0] = "MUTATED"
		if b.Statuses[0] == "MUTATED" {
			t.Errorf(
				"doc type %q DefaultConfig() shares a Statuses backing array across calls",
				dt.Name,
			)
		}
		a.Statuses[0] = original
	}
}

// TestDocTypeRegistry_LookupDocTypeResolvesCanonicalAndAliases checks
// that LookupDocType returns the right entry for both canonical names
// and aliases, and that the lookup is case-insensitive and
// whitespace-tolerant per its doc comment.
func TestDocTypeRegistry_LookupDocTypeResolvesCanonicalAndAliases(t *testing.T) {
	t.Parallel()
	cases := []struct {
		input string
		want  string
	}{
		{"rfc", "rfc"},
		{"RFC", "rfc"},
		{" rfc ", "rfc"},
		{"implementation", "impl"},
		{"IMPL", "impl"},
		{"inv", "investigation"},
		{"INVESTIGATION", "investigation"},
	}
	for _, c := range cases {
		dt, ok := config.LookupDocType(c.input)
		if !ok {
			t.Errorf("LookupDocType(%q) = (_, false), want match", c.input)
			continue
		}
		if dt.Name != c.want {
			t.Errorf(
				"LookupDocType(%q).Name = %q, want %q",
				c.input, dt.Name, c.want,
			)
		}
	}
	if _, ok := config.LookupDocType("nope"); ok {
		t.Errorf("LookupDocType(\"nope\") = (_, true), want (_, false)")
	}
}

// TestDocTypeRegistry_DocTypeNamesMatchesTypesHelp guards against the
// registry drifting out of sync with the static TypesHelp string. If
// someone adds a new registry entry but forgets to update TypesHelp,
// `docz --help` will silently omit the new type. Until TypesHelp is
// derived from the registry, this test pins the relationship.
func TestDocTypeRegistry_DocTypeNamesMatchesTypesHelp(t *testing.T) {
	t.Parallel()
	help := config.TypesHelp()
	for _, name := range config.DocTypeNames() {
		if !strings.Contains(help, name) {
			t.Errorf(
				"TypesHelp() missing entry for registered doc type %q",
				name,
			)
		}
	}
}

// TestDocTypeRegistry_DerivedDefaultConfigMatchesHardcoded pins the
// derived DefaultConfig() against the explicit type/nav-title metadata
// the registry encodes. If a registry entry's PluralLabel or NavTitle
// changes without an intentional doc update, this test catches it.
func TestDocTypeRegistry_DerivedDefaultConfigMatchesHardcoded(t *testing.T) {
	t.Parallel()
	cfg := config.DefaultConfig()
	for _, dt := range config.AllDocTypes() {
		tc, ok := cfg.Types[dt.Name]
		if !ok {
			t.Errorf("DefaultConfig().Types missing entry for %q", dt.Name)
			continue
		}
		want := dt.DefaultConfig()
		if !reflect.DeepEqual(tc, want) {
			t.Errorf(
				"DefaultConfig().Types[%q] = %+v, want %+v",
				dt.Name, tc, want,
			)
		}
		if got, want := cfg.Wiki.NavTitles[dt.Name], dt.NavTitle; got != want {
			t.Errorf(
				"DefaultConfig().Wiki.NavTitles[%q] = %q, want %q",
				dt.Name, got, want,
			)
		}
	}
}

package config

import (
	"slices"
	"strings"
)

// DocTypeDef is the per-document-type metadata that previously lived
// scattered across DefaultConfig, ValidTypes, DefaultNavTitles,
// typeAliases, and TypesHelp. Consolidating it here turns "add a new
// doc type" into a single-file edit plus two embedded templates.
//
// DefaultConfig is held as a constructor func so each lookup gets a
// fresh TypeConfig — preventing accidental mutation of the registry's
// Statuses slice from leaking across callers (DESIGN-0004 §E).
type DocTypeDef struct {
	Name          string
	Aliases       []string
	DefaultConfig func() TypeConfig
	NavTitle      string
	PluralLabel   string
	TemplateName  string
}

// allDocTypes is the single source of truth for built-in doc types.
// Adding a new type means appending one entry here plus the two
// embedded templates (<TemplateName>.md and index_<TemplateName>.md).
var allDocTypes = []DocTypeDef{
	{
		Name:    "rfc",
		Aliases: nil,
		DefaultConfig: func() TypeConfig {
			return TypeConfig{
				Enabled:     true,
				Dir:         "rfc",
				IDPrefix:    "RFC",
				IDWidth:     4,
				Statuses:    []string{"Draft", "Proposed", "Accepted", "Rejected", "Superseded"},
				StatusField: "status",
				PluralLabel: "RFCs",
			}
		},
		NavTitle:     "RFCs",
		PluralLabel:  "RFCs",
		TemplateName: "rfc",
	},
	{
		Name:    "adr",
		Aliases: nil,
		DefaultConfig: func() TypeConfig {
			return TypeConfig{
				Enabled:     true,
				Dir:         "adr",
				IDPrefix:    "ADR",
				IDWidth:     4,
				Statuses:    []string{"Proposed", "Accepted", "Deprecated", "Superseded"},
				StatusField: "status",
				PluralLabel: "ADRs",
			}
		},
		NavTitle:     "ADRs",
		PluralLabel:  "ADRs",
		TemplateName: "adr",
	},
	{
		Name:    "design",
		Aliases: nil,
		DefaultConfig: func() TypeConfig {
			return TypeConfig{
				Enabled:     true,
				Dir:         "design",
				IDPrefix:    "DESIGN",
				IDWidth:     4,
				Statuses:    []string{"Draft", "In Review", "Approved", "Implemented", "Abandoned"},
				StatusField: "status",
				PluralLabel: "Design",
			}
		},
		NavTitle:     "Design",
		PluralLabel:  "Design",
		TemplateName: "design",
	},
	{
		Name:    "impl",
		Aliases: []string{"implementation"},
		DefaultConfig: func() TypeConfig {
			return TypeConfig{
				Enabled:     true,
				Dir:         "impl",
				IDPrefix:    "IMPL",
				IDWidth:     4,
				Statuses:    []string{"Draft", "In Progress", "Completed", "Paused", "Cancelled"},
				StatusField: "status",
				PluralLabel: "Implementation Plans",
			}
		},
		NavTitle:     "Implementation Plans",
		PluralLabel:  "Implementation Plans",
		TemplateName: "impl",
	},
	{
		Name:    "plan",
		Aliases: nil,
		DefaultConfig: func() TypeConfig {
			return TypeConfig{
				Enabled:     true,
				Dir:         "plan",
				IDPrefix:    "PLAN",
				IDWidth:     4,
				Statuses:    []string{"Draft", "In Progress", "Completed", "Cancelled"},
				StatusField: "status",
				PluralLabel: "Plans",
			}
		},
		NavTitle:     "Plans",
		PluralLabel:  "Plans",
		TemplateName: "plan",
	},
	{
		Name:    "investigation",
		Aliases: []string{"inv"},
		DefaultConfig: func() TypeConfig {
			return TypeConfig{
				Enabled:  true,
				Dir:      "investigation",
				IDPrefix: "INV",
				IDWidth:  4,
				Statuses: []string{
					"Open",
					"In Progress",
					"Concluded",
					"Inconclusive",
					"Abandoned",
				},
				StatusField: "status",
				PluralLabel: "Investigations",
			}
		},
		NavTitle:     "Investigations",
		PluralLabel:  "Investigations",
		TemplateName: "investigation",
	},
}

// AllDocTypes returns a shallow copy of the registry so callers can
// iterate the metadata without risking accidental mutation of the
// package-level slice.
func AllDocTypes() []DocTypeDef {
	return slices.Clone(allDocTypes)
}

// LookupDocType resolves a user-supplied name (case-insensitive,
// whitespace-trimmed) to its DocTypeDef via canonical name or alias.
// Returns the entry plus true on match; the zero DocTypeDef plus
// false otherwise.
func LookupDocType(name string) (DocTypeDef, bool) {
	n := strings.TrimSpace(strings.ToLower(name))
	for _, dt := range allDocTypes {
		if dt.Name == n || slices.Contains(dt.Aliases, n) {
			return dt, true
		}
	}
	return DocTypeDef{}, false
}

// DocTypeNames returns the canonical names of every registered doc
// type, in registry-declaration order. Help strings and the error
// message from Config.ValidateType depend on this order, so the
// registry literal in this file is the single source of truth for it.
func DocTypeNames() []string {
	names := make([]string, len(allDocTypes))
	for i, dt := range allDocTypes {
		names[i] = dt.Name
	}
	return names
}

// defaultTypesMap builds the Types map for DefaultConfig from the
// registry. Each call returns fresh TypeConfig values so the registry
// itself stays immutable.
func defaultTypesMap() map[string]TypeConfig {
	types := make(map[string]TypeConfig, len(allDocTypes))
	for _, dt := range allDocTypes {
		types[dt.Name] = dt.DefaultConfig()
	}
	return types
}

// defaultNavTitlesMap builds the dir-to-title map used by the wiki
// nav from the registry.
func defaultNavTitlesMap() map[string]string {
	titles := make(map[string]string, len(allDocTypes))
	for _, dt := range allDocTypes {
		titles[dt.Name] = dt.NavTitle
	}
	return titles
}

// defaultTypeAliases builds the alias-to-canonical map from the
// registry, replacing the previous hand-maintained package var.
func defaultTypeAliases() map[string]string {
	aliases := make(map[string]string)
	for _, dt := range allDocTypes {
		for _, a := range dt.Aliases {
			aliases[a] = dt.Name
		}
	}
	return aliases
}

package wiki

import (
	"fmt"
	"os"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"

	"github.com/donaldgifford/docz/internal/config"
)

// MkDocsConfig is the input for CreateMkDocs. Empty optional fields are
// omitted from the generated YAML.
type MkDocsConfig struct {
	SiteName           string
	SiteDescription    string
	DocsDir            string
	RepoURL            string
	SiteURL            string
	Theme              string
	Plugins            []string
	MarkdownExtensions []string
}

// CreateMkDocs writes an initial mkdocs.yml to path using cfg. The generated
// file always includes a placeholder nav of `Home: index.md`; callers
// typically follow this with a wiki update to populate the real nav.
func CreateMkDocs(path string, cfg *MkDocsConfig) error {
	var b strings.Builder
	fmt.Fprintf(&b, "site_name: %s\n", cfg.SiteName)
	fmt.Fprintf(&b, "site_description: %s\n", cfg.SiteDescription)

	if cfg.DocsDir != "" {
		fmt.Fprintf(&b, "docs_dir: %s\n", cfg.DocsDir)
	}

	if cfg.RepoURL != "" {
		fmt.Fprintf(&b, "repo_url: %s\n", cfg.RepoURL)
	}

	if cfg.SiteURL != "" {
		fmt.Fprintf(&b, "site_url: %s\n", cfg.SiteURL)
	}

	if cfg.Theme != "" {
		fmt.Fprintf(&b, "theme: %s\n", cfg.Theme)
	}

	if len(cfg.Plugins) > 0 {
		b.WriteString("\nplugins:\n")
		for _, plugin := range cfg.Plugins {
			fmt.Fprintf(&b, "    - %s\n", plugin)
		}
	}

	if len(cfg.MarkdownExtensions) > 0 {
		b.WriteString("\nmarkdown_extensions:\n")
		for _, ext := range cfg.MarkdownExtensions {
			fmt.Fprintf(&b, "    - %s\n", ext)
		}
	}

	b.WriteString("\nnav:\n    - Home: index.md\n")

	if err := os.WriteFile(path, []byte(b.String()), config.FileMode); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

// ReadMkDocs reads a mkdocs.yml file into a generic map, preserving all fields.
func ReadMkDocs(path string) (map[string]any, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var result map[string]any
	if err := yaml.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}

	if result == nil {
		result = make(map[string]any)
	}

	return result, nil
}

// WriteMkDocs writes a mkdocs data map back to a YAML file.
func WriteMkDocs(path string, data map[string]any) error {
	out, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshalling mkdocs.yml: %w", err)
	}

	if err := os.WriteFile(path, out, config.FileMode); err != nil {
		return fmt.Errorf("writing %s: %w", path, err)
	}

	return nil
}

// NavToYAML converts a slice of NavEntry into the MkDocs nav YAML structure.
// Each entry becomes a single-key map: {"Title": "path.md"} for leaves,
// or {"Title": [...]} for groups with children.
func NavToYAML(entries []NavEntry) []any {
	result := make([]any, 0, len(entries))
	for i := range entries {
		entry := &entries[i]
		if len(entry.Children) > 0 {
			result = append(result, map[string]any{
				entry.Title: NavToYAML(entry.Children),
			})
		} else {
			result = append(result, map[string]any{
				entry.Title: entry.Path,
			})
		}
	}
	return result
}

// ExistingNavOrder extracts the ordered list of top-level section titles
// from an existing mkdocs.yml data map. Returns nil if no nav exists.
func ExistingNavOrder(data map[string]any) []string {
	navRaw, ok := data["nav"]
	if !ok {
		return nil
	}

	navList, ok := navRaw.([]any)
	if !ok {
		return nil
	}

	var titles []string
	for _, item := range navList {
		m, ok := item.(map[string]any)
		if !ok {
			continue
		}
		for k := range m {
			titles = append(titles, k)
		}
	}

	return titles
}

// MergeNavOrder reorders newEntries to preserve the order of existing section
// titles. Sections not in the existing order are appended alphabetically.
func MergeNavOrder(existing []string, newEntries []NavEntry) []NavEntry {
	if len(existing) == 0 {
		return newEntries
	}

	entryMap := make(map[string]NavEntry, len(newEntries))
	for _, e := range newEntries {
		entryMap[e.Title] = e
	}

	var result []NavEntry

	// Add entries in existing order first.
	seen := make(map[string]bool, len(existing))
	for _, title := range existing {
		if e, ok := entryMap[title]; ok {
			result = append(result, e)
			seen[title] = true
		}
	}

	// Collect new entries not in the existing order.
	var newOnes []NavEntry
	for _, e := range newEntries {
		if !seen[e.Title] {
			newOnes = append(newOnes, e)
		}
	}

	slices.SortFunc(newOnes, func(a, b NavEntry) int {
		return strings.Compare(a.Title, b.Title)
	})

	result = append(result, newOnes...)
	return result
}

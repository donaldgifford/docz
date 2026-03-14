package wiki

import (
	"fmt"
	"os"
	"sort"

	"go.yaml.in/yaml/v3"
)

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

	if err := os.WriteFile(path, out, 0o644); err != nil {
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

	// Sort new entries alphabetically by title.
	sort.Slice(newOnes, func(i, j int) bool {
		return newOnes[i].Title < newOnes[j].Title
	})

	result = append(result, newOnes...)
	return result
}

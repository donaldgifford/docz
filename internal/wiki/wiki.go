// Package wiki provides MkDocs nav generation from a docs directory tree.
package wiki

import (
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/donaldgifford/docz/internal/document"
)

// homeTitle is the nav title applied to the root-level index.md page.
const homeTitle = "Home"

// NavEntry represents a single entry in the MkDocs nav.
type NavEntry struct {
	Title    string
	Path     string     // relative to docs_dir, e.g. "rfc/0001-my-rfc.md"
	Children []NavEntry // non-nil for directory groups
}

// ScanDocs walks the docs directory recursively and builds a tree of NavEntry
// structs representing the MkDocs nav structure. Excluded directories are
// skipped. Empty directories produce no entries.
func ScanDocs(docsDir string, exclude []string, navTitles map[string]string) ([]NavEntry, error) {
	excludeSet := make(map[string]bool, len(exclude))
	for _, e := range exclude {
		excludeSet[e] = true
	}

	return scanDir(docsDir, "", excludeSet, navTitles)
}

// scanDir recursively scans a directory and returns nav entries.
// relDir is the path relative to the docs root (empty for the root itself).
func scanDir(absDir, relDir string, exclude map[string]bool, navTitles map[string]string) ([]NavEntry, error) {
	dirEntries, err := os.ReadDir(absDir)
	if err != nil {
		return nil, err
	}

	var (
		overviewEntry *NavEntry
		doczEntries   []NavEntry
		otherEntries  []NavEntry
		dirGroups     []NavEntry
	)

	for _, de := range dirEntries {
		name := de.Name()

		if de.IsDir() {
			if exclude[name] {
				continue
			}
			childRel := filepath.Join(relDir, name)
			childAbs := filepath.Join(absDir, name)
			children, err := scanDir(childAbs, childRel, exclude, navTitles)
			if err != nil {
				return nil, err
			}
			if len(children) == 0 {
				continue
			}
			dirGroups = append(dirGroups, NavEntry{
				Title:    DirTitle(name, navTitles),
				Children: children,
			})
			continue
		}

		if !strings.HasSuffix(name, ".md") {
			continue
		}

		entryPath := filepath.Join(relDir, name)
		absPath := filepath.Join(absDir, name)

		if isOverviewFile(name) {
			var title string
			if relDir == "" {
				title = homeTitle
			} else {
				title = "Overview"
			}
			overviewEntry = &NavEntry{Title: title, Path: filepath.ToSlash(entryPath)}
			continue
		}

		// DocTitle returns ("", err) on read failure (Decisions §3).
		// scanDir is happy to keep the entry in the nav using a
		// filename-derived title, so the error is intentionally
		// swallowed here. A future logger pass can surface it.
		title, err := DocTitle(absPath)
		if err != nil || title == "" {
			title = FilenameTitle(name)
		}

		entry := NavEntry{Title: title, Path: filepath.ToSlash(entryPath)}
		if document.IsDoczFile(name) {
			doczEntries = append(doczEntries, entry)
		} else {
			otherEntries = append(otherEntries, entry)
		}
	}

	slices.SortFunc(doczEntries, func(a, b NavEntry) int {
		return strings.Compare(filepath.Base(a.Path), filepath.Base(b.Path))
	})

	slices.SortFunc(otherEntries, func(a, b NavEntry) int {
		return strings.Compare(a.Title, b.Title)
	})

	slices.SortFunc(dirGroups, func(a, b NavEntry) int {
		return strings.Compare(a.Title, b.Title)
	})

	var result []NavEntry
	if overviewEntry != nil {
		result = append(result, *overviewEntry)
	}
	result = append(result, doczEntries...)
	result = append(result, otherEntries...)
	result = append(result, dirGroups...)

	return result, nil
}

// SortEntries sorts top-level nav entries: index.md (Home) first,
// then remaining entries alphabetically by title.
func SortEntries(entries []NavEntry) []NavEntry {
	if len(entries) == 0 {
		return entries
	}

	var home *NavEntry
	var rest []NavEntry

	for i := range entries {
		if entries[i].Path == "index.md" {
			entries[i].Title = homeTitle
			home = &entries[i]
		} else {
			rest = append(rest, entries[i])
		}
	}

	slices.SortFunc(rest, func(a, b NavEntry) int {
		return strings.Compare(a.Title, b.Title)
	})

	var result []NavEntry
	if home != nil {
		result = append(result, *home)
	}
	result = append(result, rest...)
	return result
}

// isOverviewFile returns true if the filename is README.md or index.md.
func isOverviewFile(name string) bool {
	lower := strings.ToLower(name)
	return lower == "readme.md" || lower == "index.md"
}

// BuildNav scans docsDir and returns the final ordered nav. When
// existingOrder is non-empty the result preserves that order via
// MergeNavOrder (new sections appended alphabetically); otherwise the
// result is sorted alphabetically with index.md pinned first via
// SortEntries.
//
// Callers typically fetch existingOrder from a previously read
// mkdocs.yml using ExistingNavOrder, and pass nil for a fresh wiki.
func BuildNav(
	docsDir string,
	exclude []string,
	navTitles map[string]string,
	existingOrder []string,
) ([]NavEntry, error) {
	entries, err := ScanDocs(docsDir, exclude, navTitles)
	if err != nil {
		return nil, err
	}

	if len(existingOrder) > 0 {
		return MergeNavOrder(existingOrder, entries), nil
	}
	return SortEntries(entries), nil
}

// CountPages returns the total number of leaf (non-directory) entries in the nav tree.
func CountPages(entries []NavEntry) int {
	count := 0
	for i := range entries {
		if len(entries[i].Children) > 0 {
			count += CountPages(entries[i].Children)
		} else {
			count++
		}
	}
	return count
}

package wiki

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// doczDocPattern matches docz-managed document filenames like "0001-some-title.md".
var doczDocPattern = regexp.MustCompile(`^\d{4,}-.*\.md$`)

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
				// Root-level index.md is always "Home" in the nav.
				title = "Home"
			} else {
				title = "Overview"
			}
			overviewEntry = &NavEntry{Title: title, Path: filepath.ToSlash(entryPath)}
			continue
		}

		title, err := DocTitle(absPath)
		if err != nil {
			title = FilenameTitle(name)
		}

		entry := NavEntry{Title: title, Path: filepath.ToSlash(entryPath)}
		if doczDocPattern.MatchString(name) {
			doczEntries = append(doczEntries, entry)
		} else {
			otherEntries = append(otherEntries, entry)
		}
	}

	// Sort docz entries by filename (numeric ID order).
	sort.Slice(doczEntries, func(i, j int) bool {
		return filepath.Base(doczEntries[i].Path) < filepath.Base(doczEntries[j].Path)
	})

	// Sort other entries alphabetically by title.
	sort.Slice(otherEntries, func(i, j int) bool {
		return otherEntries[i].Title < otherEntries[j].Title
	})

	// Sort directory groups alphabetically by title.
	sort.Slice(dirGroups, func(i, j int) bool {
		return dirGroups[i].Title < dirGroups[j].Title
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
			entries[i].Title = "Home"
			home = &entries[i]
		} else {
			rest = append(rest, entries[i])
		}
	}

	sort.Slice(rest, func(i, j int) bool {
		return rest[i].Title < rest[j].Title
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

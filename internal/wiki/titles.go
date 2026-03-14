// Package wiki provides MkDocs nav generation from a docs directory tree.
package wiki

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/donaldgifford/docz/internal/document"
)

// DirTitle resolves a directory name to a human-friendly nav title.
// It checks the navTitles map first, then title-cases the directory name
// (converting hyphens to spaces).
func DirTitle(dir string, navTitles map[string]string) string {
	if title, ok := navTitles[dir]; ok {
		return title
	}
	return titleCase(strings.ReplaceAll(dir, "-", " "))
}

// DocTitle extracts a nav title from a markdown file.
// For docz documents with frontmatter, it returns "<ID>: <Title>".
// Otherwise it falls back to the first H1 heading, then to a title-cased filename.
func DocTitle(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return FilenameTitle(filepath.Base(filePath)), err
	}

	fm, fmErr := document.ParseFrontmatter(data)
	if fmErr == nil && fm.ID != "" && fm.Title != "" {
		return fm.ID + ": " + fm.Title, nil
	}

	if h1 := firstH1(data); h1 != "" {
		return h1, nil
	}

	return FilenameTitle(filepath.Base(filePath)), nil
}

// FilenameTitle converts a markdown filename to a title-cased display name.
// For example, "system-overview.md" becomes "System Overview".
func FilenameTitle(filename string) string {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	name = strings.ReplaceAll(name, "-", " ")
	name = strings.ReplaceAll(name, "_", " ")
	return titleCase(name)
}

// firstH1 scans file content for the first markdown H1 heading and returns
// the heading text. Returns empty string if no H1 is found.
func firstH1(data []byte) string {
	scanner := bufio.NewScanner(strings.NewReader(string(data)))
	inFrontmatter := false
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Skip YAML frontmatter.
		if trimmed == "---" {
			inFrontmatter = !inFrontmatter
			continue
		}
		if inFrontmatter {
			continue
		}

		if after, ok := strings.CutPrefix(trimmed, "# "); ok {
			return strings.TrimSpace(after)
		}
	}
	return ""
}

// titleCase capitalizes the first letter of each word in s.
func titleCase(s string) string {
	words := strings.Fields(s)
	for i, w := range words {
		if w == "" {
			continue
		}
		runes := []rune(w)
		runes[0] = unicode.ToUpper(runes[0])
		words[i] = string(runes)
	}
	return strings.Join(words, " ")
}

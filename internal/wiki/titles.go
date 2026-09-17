package wiki

import (
	"errors"
	"path/filepath"
	"strings"
	"unicode"

	"github.com/donaldgifford/docz/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/pkg/doczcore/document"
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
//
// Contract: standard "value OR error" — on read failure DocTitle returns
// "", err. Callers that want a fallback title for unreadable files
// should detect the error and call FilenameTitle explicitly.
//
// For docz documents with valid frontmatter the result is
// "<ID>: <Title>". Files with no frontmatter (or with frontmatter
// missing ID/Title) fall back to the first H1 heading via
// docparse.Title; if there is no H1 either, the title-cased filename is
// returned. None of those fallbacks count as an error.
func DocTitle(filePath string) (string, error) {
	fm, data, err := document.LoadFrontmatter(filePath)
	switch {
	case err == nil:
		if fm.ID != "" && fm.Title != "" {
			return fm.ID + ": " + fm.Title, nil
		}
	case errors.Is(err, document.ErrNoFrontmatter):
		// No frontmatter is not fatal — fall through to H1 / filename
		// fallback using the bytes LoadFrontmatter returned.
	default:
		return "", err
	}

	if h1 := docparse.Title(data); h1 != "" {
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

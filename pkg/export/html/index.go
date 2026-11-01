package html

import (
	"strings"
	"unicode"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/repo"
)

// IndexPath is the index page's path under the output directory.
const IndexPath = "index.html"

// index renders the run's index: one tab per type, each a table of id,
// title, status, and date in docz list order, every id linking to its page.
// It is written as markdown and rendered like any document, so it carries
// the theme and the page's escaping. Pages that failed are left out.
func (x *exporter) index(entries []repo.Entry, results []PageResult) PageResult {
	res := PageResult{Path: IndexPath, Links: []Link{}, Problems: []string{}}

	title := "Documents"
	if x.opts.Site != "" {
		title = x.opts.Site + " documents"
	}

	var md strings.Builder

	md.WriteString("---\ntitle: " + yamlString(title) + "\n---\n")

	typ := ""

	for i := range entries {
		e := &entries[i]
		if results[i].Action == ActionFailed {
			continue
		}

		if e.Type != typ {
			if typ != "" {
				md.WriteString(listingEnd)
			}

			typ = e.Type
			md.WriteString(listingStart + "## " + x.label(typ) + "\n\n| ID | Title | Status | Date |\n| --- | --- | --- | --- |\n")
		}

		md.WriteString("| [" + cell(e.ID) + "](" + results[i].Path + ") | " + cell(e.Title) + " | " +
			cell(string(e.Status)) + " | " + cell(e.Created) + " |\n")
	}

	if typ != "" {
		md.WriteString(listingEnd)
	}

	pages := make(map[string]bool, len(results))
	for i := range results {
		pages[results[i].Path] = true
	}

	p, err := Render([]byte(md.String()), RenderOptions{
		Theme: x.opts.Theme, Site: x.opts.Site, Version: x.opts.Version, Fragment: x.opts.Fragment,
		Resolve: func(_, href string) string {
			if pages[strings.SplitN(href, "#", 2)[0]] {
				return href
			}

			return ""
		},
	})
	if err != nil {
		return failed(&res, err)
	}

	res.Hash, res.Body = p.Hash, p.Body

	action, err := x.write(IndexPath, p.Body)
	if err != nil {
		return failed(&res, err)
	}

	res.Action = action

	return res
}

// The index marks each type's table as a region, so the page is not
// reported as inferred; the kind is one the layout table does not list, so
// each is a text tab named by its heading.
const (
	listingStart = "\n<!--docz:listing:start-->\n"
	listingEnd   = "\n<!--docz:listing:end-->\n"
)

// label is a type's plural label as its README index heading names it:
// the configured one, else the type name capitalised.
func (x *exporter) label(typeName string) string {
	if l := x.rp.Cfg.Types[typeName].PluralLabel; l != "" {
		return l
	}

	runes := []rune(typeName)
	if len(runes) > 0 {
		runes[0] = unicode.ToUpper(runes[0])
	}

	return string(runes)
}

// cell is text safe in a table cell: a pipe would end it and a newline
// the row.
func cell(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")

	return strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), "|", `\|`)
}

// yamlString is s as a double-quoted YAML scalar.
func yamlString(s string) string {
	return `"` + strings.ReplaceAll(strings.ReplaceAll(s, `\`, `\\`), `"`, `\"`) + `"`
}

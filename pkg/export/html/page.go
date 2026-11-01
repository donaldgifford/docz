package html

import (
	"bytes"
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"html/template"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/doczcore/docparse"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/theme"
)

//go:embed templates/page.html.tmpl
var pageSource string

//go:embed templates/fragment.html.tmpl
var fragmentSource string

//go:embed templates/wrap.html.tmpl
var wrapSource string

// pageTemplate is the page skeleton, and fragmentTemplate the document
// alone; both execute the one "wrap" template, so the two cannot drift.
// html/template escapes every value it interpolates by its context; the
// few trusted values (the theme, the rendered markdown) are typed as such
// at one place each.
var (
	pageTemplate     = template.Must(template.Must(template.New("page").Parse(wrapSource)).Parse(pageSource))
	fragmentTemplate = template.Must(template.Must(template.New("fragment").Parse(wrapSource)).Parse(fragmentSource))
)

// csp is every page's Content-Security-Policy: nothing loads from the
// network, and inline style and script are the page's own (DESIGN-0025 §8).
const csp = "default-src 'none'; style-src 'unsafe-inline'; " +
	"script-src 'unsafe-inline'; img-src data: https:"

// themeMarker brackets the inlined theme, so a reader of the source (and a
// golden) can find and cut it. html/template drops comments written in a
// template, so the markers are values.
const (
	themeOpen  = template.HTML("<!-- docz:theme -->")
	themeClose = template.HTML("<!-- /docz:theme -->")
)

// pageData is what the template reads.
type pageData struct {
	Theme, ID, Title, FullTitle, Eyebrow string
	CSP                                  string
	Fragment                             bool
	Summary                              template.HTML
	Pills                                []pill
	Inferred                             bool
	Panels                               []panelData
	Source, SHA256, Version              string
	Mermaid                              *mermaidData
	CSS                                  template.CSS
	Runtime                              template.JS
	ThemeOpen, ThemeClose                template.HTML
}

type pill struct{ Class, Text string }

type panelData struct {
	Key, Label, Anchor string
	Contents           []docparse.Heading
	Body               template.HTML
}

// trusted marks markdown the node renderer produced as safe HTML. It is the
// one conversion of rendered output to template.HTML: the renderer escapes
// text, drops comments, and rebuilds raw HTML from its allow-list, so what
// reaches here is the renderer's own markup.
func trusted(b []byte) template.HTML {
	return template.HTML(b) //nolint:gosec // G203: the node renderer's escaped output, see above.
}

// typeLabels are the built-in types' singular labels for the eyebrow; a
// custom type shows its name.
var typeLabels = map[string]string{
	"rfc": "RFC", "adr": "ADR", "design": "Design", "impl": "Implementation plan",
	"investigation": "Investigation", "runbook": "Runbook",
}

// Pill classes for statuses that share a colour.
const (
	pillClass  = "pill"
	stActive   = "st-active"
	stAccepted = "st-accepted"
	stProposed = "st-proposed"
	stRejected = "st-rejected"
)

// statusClasses maps a status to its pill class, as the skill's
// components reference does.
var statusClasses = map[string]string{
	"draft": "st-draft", "proposed": stProposed, "in review": stProposed,
	"active": stActive, "in progress": stActive, "open": stActive,
	"accepted": stAccepted, "approved": stAccepted, "implemented": stAccepted,
	"completed": stAccepted, "concluded": stAccepted, "adopted": "st-adopted",
	"rejected": stRejected, "abandoned": stRejected, "superseded": "st-superseded",
	"deprecated": "st-deprecated",
}

// writePage executes the template over the rendered panels.
//
//nolint:gocritic // RenderOptions by value, as Render takes it.
func writePage(src []byte, fm *document.Frontmatter, d *doc, opts RenderOptions) ([]byte, error) {
	name := opts.Theme
	if name == "" {
		name = "paper"
	}

	css, err := theme.CSS(name)
	if err != nil {
		return nil, fmt.Errorf("html: %w", err)
	}

	sum := sha256.Sum256(src)

	data := pageData{
		Theme: name, ID: fm.ID, Title: fm.Title, FullTitle: fullTitle(fm),
		Eyebrow:  eyebrow(opts.Site, opts.Type),
		Fragment: opts.Fragment,
		Summary:  trusted(d.summary),
		Pills:    pills(fm),
		Inferred: d.inferred,
		Source:   opts.Source, SHA256: hex.EncodeToString(sum[:]), Version: opts.Version,
	}

	// A fragment's host brings the stylesheet, the runtime, and mermaid.
	if !opts.Fragment {
		data.Mermaid = mermaidFor(opts.Mermaid, d.md.nodes.mermaid)
		data.CSS = template.CSS(css)                //nolint:gosec // G203: pkg/theme's own stylesheet.
		data.Runtime = template.JS(theme.Runtime()) //nolint:gosec // G203: pkg/theme's own runtime.
		data.ThemeOpen, data.ThemeClose = themeOpen, themeClose
	}

	for _, p := range d.rendered {
		body := p.Body
		if len(bytes.TrimSpace(body)) == 0 {
			body = []byte(`<p class="empty">Not written yet.</p>` + "\n")
		}

		data.Panels = append(data.Panels, panelData{
			Key: p.Key, Label: p.Label, Anchor: p.Anchor, Contents: p.Contents, Body: trusted(body),
		})
	}

	data.CSP = pagePolicy(data.Mermaid)

	tmpl := pageTemplate
	if opts.Fragment {
		tmpl = fragmentTemplate
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, data); err != nil {
		return nil, fmt.Errorf("html: page: %w", err)
	}

	return out.Bytes(), nil
}

// fullTitle is "ID: Title", or whichever of the two exists.
func fullTitle(fm *document.Frontmatter) string {
	switch {
	case fm.ID != "" && fm.Title != "":
		return fm.ID + ": " + fm.Title
	case fm.Title != "":
		return fm.Title
	default:
		return fm.ID
	}
}

// eyebrow is "site · Label", or the label alone.
func eyebrow(site, typeName string) string {
	label := typeLabels[typeName]
	if label == "" {
		label = typeName
	}

	switch {
	case site != "" && label != "":
		return site + " · " + label
	case site != "":
		return site
	default:
		return label
	}
}

// pills are the header's id, status, author, and created date.
func pills(fm *document.Frontmatter) []pill {
	var out []pill

	if fm.ID != "" {
		out = append(out, pill{Class: pillClass, Text: fm.ID})
	}

	if s := string(fm.Status); s != "" {
		class := pillClass
		if c := statusClasses[strings.ToLower(s)]; c != "" {
			class += " " + c
		}

		out = append(out, pill{Class: class, Text: s})
	}

	for _, v := range []string{fm.Author, fm.Created} {
		if v != "" {
			out = append(out, pill{Class: pillClass, Text: v})
		}
	}

	return out
}

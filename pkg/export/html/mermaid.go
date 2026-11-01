package html

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"html/template"
	"io"
	"sync"
)

// mermaidVersion is the mermaid the page draws with: the version ui/
// resolves, so a diagram looks the same on docz-site and on a page.
const mermaidVersion = "12.0.0"

// mermaidCDN is the pinned URL MermaidCDN loads, and mermaidIntegrity its
// subresource-integrity hash (sha384 of the published mermaid.min.js).
const (
	mermaidCDN       = "https://cdn.jsdelivr.net/npm/mermaid@" + mermaidVersion + "/dist/mermaid.min.js"
	mermaidCDNOrigin = "https://cdn.jsdelivr.net"
	mermaidIntegrity = "sha384-xzghz1GQ5u9HCpVskeDPqMsdogD1yvuMQbEK53+wi+G70+6J1AG0L2cfi9PHjDWI"
)

// mermaidInit draws the page's diagrams in its theme.
//
//go:embed assets/mermaid-init.js
var mermaidInit string

// mermaidGz is mermaid itself, gzipped by just vendor-mermaid from the
// version ui/bun.lock pins.
//
//go:embed assets/mermaid.min.js.gz
var mermaidGz []byte

// mermaidScript returns mermaid itself for MermaidEmbed, decompressed the
// first time a page needs it. The asset is checked by a test, so a failure
// here is a broken build; it leaves a comment, and the page shows each
// diagram's source.
var mermaidScript = sync.OnceValue(func() string {
	zr, err := gzip.NewReader(bytes.NewReader(mermaidGz))
	if err != nil {
		return "/* mermaid: " + err.Error() + " */"
	}

	b, err := io.ReadAll(zr)
	if err != nil {
		return "/* mermaid: " + err.Error() + " */"
	}

	return string(b)
})

// mermaidData is what the template reads to draw diagrams.
type mermaidData struct {
	// Inline is mermaid itself, for MermaidEmbed.
	Inline template.JS
	// URL and Integrity load it, for MermaidCDN.
	URL, Integrity string
	// Init draws the diagrams.
	Init template.JS
}

// mermaidFor is the template's mermaid data for a page, nil when the page
// has no diagram to draw.
func mermaidFor(mode MermaidMode, needed bool) *mermaidData {
	if !needed || mode == MermaidSource {
		return nil
	}

	m := &mermaidData{Init: template.JS(mermaidInit)} //nolint:gosec // G203: the package's own script.

	if mode == MermaidCDN {
		m.URL, m.Integrity = mermaidCDN, mermaidIntegrity

		return m
	}

	m.Inline = template.JS(mermaidScript()) //nolint:gosec // G203: the vendored mermaid.

	return m
}

// pagePolicy is the page's Content-Security-Policy: csp, with the CDN's
// origin allowed for scripts when the page loads mermaid from it.
func pagePolicy(m *mermaidData) string {
	if m != nil && m.URL != "" {
		return "default-src 'none'; style-src 'unsafe-inline'; " +
			"script-src 'unsafe-inline' " + mermaidCDNOrigin + "; img-src data: https:"
	}

	return csp
}

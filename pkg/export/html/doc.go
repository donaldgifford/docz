// Package html renders a docz document as one standalone HTML page
// (DESIGN-0025): a header with the summary and status, a tab per region, and
// docz's theme inlined, so the file opens from disk with no network request.
//
// [Render] is pure: bytes in, a [Page] out, no filesystem, no state between
// calls. It reads the document's regions with kinds.ResolveRegions, so an
// unmarked document gets inferred regions and the page says so, and renders
// each region's markdown with goldmark (GFM and footnotes) through a node
// renderer that gives headings docz's own anchors, turns GitHub alerts into
// callouts and task lists into read-only checkboxes, and escapes raw HTML
// except a short allow-list whose tags lose every attribute.
//
// The page skeleton is an html/template, so every interpolated value is
// escaped by construction. The theme (pkg/theme) is inlined between
// <!-- docz:theme --> markers.
//
// Every page carries a Content-Security-Policy that loads nothing from the
// network, outside the CDN mermaid mode. Nothing time-dependent is written:
// the same input gives the same bytes, and [Page.Hash] is their SHA-256.
package html

// Package theme is the one design for docz's HTML (DESIGN-0025 §3): the
// stylesheet and the small runtime that an exported page inlines, and the
// token files the docz-site and docz-review pages import at build time.
//
// A theme is a set of token values and nothing else. Every component and
// the runtime read colours, radii, and fonts only through custom
// properties, so a theme changes how a page looks without a component
// changing. There are two:
//
//   - paper, the default: a warm palette with light and dark values.
//   - classic: docz-site's dark look, its values unchanged from
//     ui/src/theme/tokens.css.
//
// A page selects one with attributes on its root element:
//
//	<html data-theme="paper" data-mode="dark">
//
// With no data-mode, the reader's prefers-color-scheme decides. The
// runtime's toggle sets data-mode and remembers it in localStorage under
// docz:mode, which every docz page shares.
//
// The files are part of the package's semver contract by name and by
// token: a token a theme defines, or a class a component offers, is not
// removed or renamed outside a major release. Their bytes are not
// governed, so a colour or a spacing value may change in any release.
package theme

# pkgsite vs godoc

Why `go-docz` uses `pkgsite` (the binary behind
pkg.go.dev) rather than parsing raw godoc text.

## The two tools

| Tool | Purpose | Output |
|---|---|---|
| `go doc <pkg>` | CLI inspection of a single symbol or package | Plain text |
| `godoc` (legacy) | Old web UI for godoc, now retired in favor of pkgsite | HTML |
| `pkgsite` | The renderer pkg.go.dev uses | HTML (also JSON via flag) |

`pkgsite` is the canonical choice because:

- **Identifier cross-linking.** Function references in
  comments resolve to other packages' docs.
- **Example rendering.** `Example*` functions in `*_test.go`
  show as runnable examples with output blocks.
- **Deprecation parsing.** `Deprecated:` markers render
  as warnings.
- **Heading detection.** Comment lines starting with `#`
  (per the [proposal](https://go.dev/doc/comment))
  become real headings.
- **Bullet/numbered lists.** Renders as lists, not as
  inline text.

Rolling our own godoc parser would lose all of this. The
output would also drift from what users see on pkg.go.dev,
making it useless as a teaching reference.

## What this skill does with pkgsite

```bash
# Install
go install golang.org/x/pkgsite/cmd/pkgsite@latest

# Boot offline against the local module
pkgsite -http=:8080 -gopath_mode=false .
```

Then the skill fetches the HTML pages and converts them
to markdown using a structured transformer. The fetch
loop is:

1. `GET /<module-path>` → module index page.
2. Extract package list from the index.
3. For each package: `GET /<module-path>/<pkg>` →
   per-package page.
4. HTML → markdown (preserves links, headings, code
   blocks, examples).

## Cross-package links

`pkgsite` emits cross-package links as absolute paths:

```html
<a href="/some/other/pkg#Type">Type</a>
```

The HTML→markdown converter rewrites these into either:

- **Same-module links** → relative paths within the
  generated docs (`./pkg-other.md#Type`).
- **External links** → absolute URLs to pkg.go.dev.

## Example rendering

Examples come through as:

```markdown
### Example

```go
func ExampleParse() {
    cfg, _ := Parse("config.toml")
    fmt.Println(cfg.Name)
    // Output: my-app
}
```

`pkgsite` distinguishes runnable examples from snippets by
the `// Output:` comment. The converter preserves both.

## Alternative: `go doc -all`

For minimalist setups (small private modules, simple
output, no cross-linking needs), `go doc -all <pkg>`
produces serviceable plain text:

```bash
go doc -all . > docs/api/go/index.md
```

This is the fallback path when `pkgsite` isn't available.
The skill detects `pkgsite` in `$PATH`; if absent, it
emits plain text with a warning that cross-package links
and example rendering will be missing.

## See also

- [pkgsite repo](https://github.com/golang/pkgsite)
- [pkg.go.dev](https://pkg.go.dev) — the public instance
- [Go doc comment guide](https://go.dev/doc/comment) —
  the syntax pkgsite parses
- `../../../../go-development/skills/go-doc/` —
  generate the `doc.go` files that pkgsite renders from

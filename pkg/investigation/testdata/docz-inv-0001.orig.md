---
id: INV-0001
title: "Wiki Init Template and Init Enabled Fix"
status: Concluded
author: Donald Gifford
created: 2026-04-02
---
<!-- markdownlint-disable-file MD025 MD041 -->

# INV 0001: Wiki Init Template and Init Enabled Fix

**Status:** Concluded
**Author:** Donald Gifford
**Date:** 2026-04-02

<!--toc:start-->
- [Question](#question)
- [Hypothesis](#hypothesis)
- [Context](#context)
- [Approach](#approach)
- [Findings](#findings)
  - [Finding 1: ensureDocsIndex() is fully hardcoded](#finding-1-ensuredocsindex-is-fully-hardcoded)
  - [Finding 2: writeMkDocsYAML() hardcodes techdocs-core](#finding-2-writemkdocsyaml-hardcodes-techdocs-core)
  - [Finding 3: runInit() skips enabled check](#finding-3-runinit-skips-enabled-check)
  - [Finding 4: Template system already supports wiki-style templates](#finding-4-template-system-already-supports-wiki-style-templates)
- [Conclusion](#conclusion)
- [Recommendation](#recommendation)
  - [Change 1: Wiki index template](#change-1-wiki-index-template)
  - [Change 2: Configurable MkDocs plugins](#change-2-configurable-mkdocs-plugins)
  - [Change 3: docz init respects enabled: false](#change-3-docz-init-respects-enabled-false)
- [Decisions](#decisions)
- [References](#references)
<!--toc:end-->

## Question

What changes are needed to (1) support a customizable template for the wiki
`docs/index.md` homepage, (2) allow additional MkDocs plugins from `.docz.yaml`,
and (3) fix `docz init` ignoring the `enabled: false` type config?

## Hypothesis

These are three small, related improvements that can be addressed in a single
branch without a full design doc. The wiki template and plugin changes extend
the existing `WikiConfig`, and the init fix is a one-line guard.

## Context

Dogfooding `docz` on its own repository revealed:

1. **`docs/index.md` is hardcoded** — `ensureDocsIndex()` in `cmd/wiki.go`
   generates the homepage inline with a hardcoded format. There's no way to
   customize it via a template like the type-specific index headers
   (`index_rfc.md`, `index_adr.md`, etc.). Users who want a different homepage
   layout must manually edit after generation.

2. **MkDocs plugins are hardcoded** — `writeMkDocsYAML()` always writes
   `plugins: [techdocs-core]`. Users who need additional plugins (e.g.
   `search`, `mermaid`, `awesome-pages`) must manually edit `mkdocs.yml` after
   init, and their additions survive `wiki update` (since it only touches
   `nav`), but there's no way to declare them upfront in `.docz.yaml`.

3. **`docz init` ignores `enabled: false`** — `runInit()` iterates over
   `config.ValidTypes()` (hardcoded list of all 6 types) and creates
   directories + READMEs for every type, even when `enabled: false` is set in
   the config. Similarly, `ensureDocsIndex()` lists all types in the generated
   homepage regardless of enabled status.

**Triggered by:** Dogfooding session on the docz repository

## Approach

1. Audit `cmd/wiki.go` — `ensureDocsIndex()` and `writeMkDocsYAML()` to
   understand exactly what's hardcoded
2. Audit `cmd/init.go` — `runInit()` to confirm the enabled check is missing
3. Review the existing template system (`internal/template/embed.go`) to
   understand how index headers are embedded and resolved
4. Determine the right template approach for `docs/index.md`
5. Determine the config shape for additional MkDocs plugins
6. Propose changes and open questions

## Findings

### Finding 1: `ensureDocsIndex()` is fully hardcoded

`cmd/wiki.go:264-296` generates `docs/index.md` inline:

```go
var b strings.Builder
b.WriteString("# " + siteName + "\n\n")
b.WriteString("Welcome to the documentation for " + siteName + ".\n\n")
b.WriteString("## Document Types\n\n")
for _, typeName := range config.ValidTypes() {
    // ...
    b.WriteString(fmt.Sprintf("- [%s](%s/README.md)\n", navTitle, tc.Dir))
}
```

Problems:
- No template — content is built with string concatenation
- Iterates `config.ValidTypes()` — doesn't check `enabled`
- No override mechanism — users can't customize the homepage layout

### Finding 2: `writeMkDocsYAML()` hardcodes `techdocs-core`

`cmd/wiki.go:246-262`:

```go
content := fmt.Sprintf(`site_name: %s
site_description: %s

plugins:
    - techdocs-core

nav:
    - Home: index.md
`, siteName, siteDesc)
```

The `techdocs-core` plugin is always written. Users who need additional plugins
must edit the file manually. Since `wiki update` only modifies the `nav` key,
extra plugins added manually are preserved — but there's no way to declare
them upfront in config.

### Finding 3: `runInit()` skips enabled check

`cmd/init.go:36-46`:

```go
for _, typeName := range config.ValidTypes() {
    typeDir := appCfg.TypeDir(typeName)
    if err := os.MkdirAll(typeDir, 0o750); err != nil {
        return fmt.Errorf("creating directory %s: %w", typeDir, err)
    }
    readmePath := filepath.Join(typeDir, "README.md")
    if err := writeIndexReadme(readmePath, typeName); err != nil {
        return err
    }
}
```

No check for `appCfg.Types[typeName].Enabled`. Every type gets a directory
and README regardless of config.

### Finding 4: Template system already supports wiki-style templates

The existing `internal/template/embed.go` uses `//go:embed templates/*.md` and
provides `EmbeddedIndexHeader(typeName)` for type-specific index headers. Adding
an embedded `wiki_index.md` template would follow the same pattern. The template
could use `text/template` variables like `{{ .SiteName }}` and a list of enabled
types.

## Conclusion

**Answer:** Three targeted changes are needed, all straightforward.

## Recommendation

### Change 1: Wiki index template

- Add `internal/template/templates/wiki_index.md` with template variables:
  `{{ .SiteName }}`, `{{ .Types }}` (list of enabled types with nav titles and
  dirs)
- Add `EmbeddedWikiIndex()` to `internal/template/embed.go` (or a general
  accessor)
- Update `ensureDocsIndex()` in `cmd/wiki.go` to render the template instead of
  hardcoding. Skip disabled types in the type list.
- Allow override via `wiki.index_template` config path (same resolution pattern
  as type templates: config path → local file → embedded default)

### Change 2: Configurable MkDocs plugins

- Add `Plugins []string` field to `WikiConfig`:
  ```yaml
  wiki:
    plugins:
      - techdocs-core
      - search
  ```
- Default: `["techdocs-core"]`
- Update `writeMkDocsYAML()` to write all configured plugins
- `wiki update` already preserves non-nav fields, so manually-added plugins
  remain safe

### Change 3: `docz init` respects `enabled: false`

- Add `if !appCfg.Types[typeName].Enabled { continue }` in `runInit()` loop
- Same guard in `ensureDocsIndex()` type list loop
- Same guard in `writeDefaultConfig()` — or leave that as-is since it's the
  reference config showing all available types

## Decisions

1. **`writeDefaultConfig()` keeps all types in generated `.docz.yaml`.** The
   config file serves as documentation. `docz init` and `wiki init` respect
   `enabled: false` when creating directories and listing types.

2. **Wiki index template uses the existing override pattern.** Override at
   `<docs_dir>/templates/wiki_index.md`. No new config key needed.

3. **`wiki.plugins` defaults to `["techdocs-core"]`.** Primary use case is
   Backstage TechDocs. Users who don't use Backstage can override.

4. **`wiki update` does not sync plugins.** Only `wiki init` writes plugins.
   Manual edits to plugins in `mkdocs.yml` are preserved.

5. **Wiki index template variables:** `{{ .SiteName }}` and `{{ .Types }}`
   (slice of structs with `.Name`, `.NavTitle`, `.Dir` for each enabled type).
   Start with these and extend if needed.

## References

- `cmd/wiki.go` — `ensureDocsIndex()`, `writeMkDocsYAML()`
- `cmd/init.go` — `runInit()`
- `internal/template/embed.go` — template embedding pattern
- `internal/config/config.go` — `WikiConfig` struct
- DESIGN-0002: Wiki Command for MkDocs TechDocs Integration

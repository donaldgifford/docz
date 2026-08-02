package config

import "os"

// File and directory permissions used when docz writes to disk.
const (
	FileMode os.FileMode = 0o644
	DirMode  os.FileMode = 0o750
)

// Well-known filenames and directory names used by docz.
const (
	ConfigFileName = ".docz.yaml"
	IndexFileName  = "README.md"
	WikiIndexName  = "index.md"
	MkDocsFileName = "mkdocs.yml"
	TemplatesDir   = "templates"

	// DefaultChangelogFile is the repo-relative path ChangelogConfig.File
	// falls back to when the changelog block omits it (DESIGN-0010).
	DefaultChangelogFile = "CHANGELOG.md"
)

// defaultMinHeadings is the default minimum heading count required before a
// document's table of contents is rendered.
const defaultMinHeadings = 3

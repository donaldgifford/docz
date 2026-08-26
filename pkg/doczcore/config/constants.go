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

	// APILandingFileName is the filename APIConfig.LandingPage falls back
	// to, resolved under DocsDir so it follows a non-default docs_dir
	// (DESIGN-0011 Decision 3).
	//
	// Deliberately not WikiIndexName, which happens to hold the same
	// string: that one names MkDocs' landing page, and the two should be
	// free to diverge without silently changing each other.
	APILandingFileName = "index.md"
)

// defaultMinHeadings is the default minimum heading count required before a
// document's table of contents is rendered.
const defaultMinHeadings = 3

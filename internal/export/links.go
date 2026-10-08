package export

import (
	"io/fs"
	"path"
	"path/filepath"
	"strings"

	doczcfg "github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	doczdoc "github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/export/confluence"
)

// blobResolver places a relative link the export does not write as the
// file's blob URL on GitHub at the default branch. The server holds only
// what it ingested, so it cannot check that a file outside docs_dir exists,
// and a link to one is the common case (INV-0020 decision 10). What it can
// check is a document: missing reports a path that would be a document of
// an enabled type but was not ingested, which is a document since removed,
// and such a link stays unresolved rather than becoming a blob URL to
// nothing. A link that climbs out of the repository stays unresolved too.
func blobResolver(owner, name, branch string, missing func(rel string) bool) confluence.LinkResolver {
	base := "https://github.com/" + owner + "/" + name + "/blob/" + branch + "/"

	return func(from, href string) confluence.LinkTarget {
		file, frag, hasFrag := strings.Cut(href, "#")
		if file == "" || path.IsAbs(file) {
			return confluence.LinkTarget{}
		}

		rel := path.Clean(path.Join(path.Dir(from), file))
		if rel == ".." || strings.HasPrefix(rel, "../") {
			return confluence.LinkTarget{}
		}

		if missing(rel) {
			return confluence.LinkTarget{}
		}

		url := base + rel
		if hasFrag {
			url += "#" + frag
		}

		return confluence.LinkTarget{URL: url}
	}
}

// missingDocument reports a repository path that names a document file in
// an enabled type's directory and is not in fsys. fsys holds every ingested
// document, whichever types the export is narrowed to, so a document that
// exists is never reported.
func missingDocument(fsys fs.FS, cfg *doczcfg.Config) func(rel string) bool {
	dirs := make(map[string]bool)
	for _, typeName := range cfg.EnabledTypes() {
		dirs[path.Clean(filepath.ToSlash(cfg.TypeDir(typeName)))] = true
	}

	return func(rel string) bool {
		if !dirs[path.Dir(rel)] || !doczdoc.IsDoczFile(path.Base(rel)) {
			return false
		}

		_, err := fs.Stat(fsys, rel)

		return err != nil
	}
}

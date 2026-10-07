package export

import (
	"path"
	"strings"

	"github.com/donaldgifford/docz/v2/pkg/export/confluence"
)

// blobResolver places a relative link the export does not write as the
// file's blob URL on GitHub at the default branch. It does not check the
// file exists: the server holds only what it ingested, and a link to a file
// outside docs_dir is the common case (INV-0020 decision 10). A link that
// climbs out of the repository stays unresolved.
func blobResolver(owner, name, branch string) confluence.LinkResolver {
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

		url := base + rel
		if hasFrag {
			url += "#" + frag
		}

		return confluence.LinkTarget{URL: url}
	}
}

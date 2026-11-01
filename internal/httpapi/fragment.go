package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"
	"path"
	"regexp"
	"slices"
	"strings"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"

	"github.com/donaldgifford/docz/v2/internal/store"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/doctemplate"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/kinds"
	"github.com/donaldgifford/docz/v2/pkg/export/html"
	"github.com/donaldgifford/docz/v2/pkg/theme"
)

// DocumentHTMLDTO is one document rendered by pkg/export/html as a
// fragment, for docz-site's reader to embed (DESIGN-0026 §4).
type DocumentHTMLDTO struct {
	DocID string `json:"doc_id"`
	// HTML is one div.docz-page: the header, the tab bar, and the panels.
	HTML string `json:"html"`
	// Hash is "sha256:<hex>" of HTML.
	Hash string `json:"hash"`
	// Inferred reports that the sections were inferred from headings.
	Inferred bool `json:"inferred"`
	// Problems lists what fell back to plain rendering, such as a widget
	// whose YAML did not validate.
	Problems []string `json:"problems"`
}

// getDocHTML renders one document as a docz-html fragment. It resolves the
// repository, type, and document exactly as getDoc does, so a document the
// caller may not read is the same 404.
func (h *Handler) getDocHTML(w http.ResponseWriter, r *http.Request) {
	repo, canonical, ok := h.resolveRepoType(w, r)
	if !ok {
		return
	}
	row, err := h.store.GetDocumentByID(r.Context(), repo.ID, chi.URLParam(r, "doc_id"))
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	if err != nil {
		serverError(w, "get document", err)
		return
	}
	if row.Document.Type != canonical {
		writeError(w, http.StatusNotFound, "document not found")
		return
	}
	types, err := h.store.GetDocTypesForRepo(r.Context(), repo.ID)
	if err != nil {
		serverError(w, "get doc types", err)
		return
	}
	pages, err := h.store.ListRepoPages(r.Context(), repo.ID)
	if err != nil {
		serverError(w, "list pages", err)
		return
	}

	out, err := renderFragment(&repo, &row.Document, types, pages)
	if err != nil {
		// A stored document always has frontmatter and LF endings (ingest
		// parsed it), so a failure here is the server's, not the caller's.
		serverError(w, "render document", err)
		return
	}

	writeJSON(w, out)
}

// renderFragment renders one stored document as the reader's fragment.
// The UI's MSW fixtures are generated through it too.
func renderFragment(
	repo *store.Repo, doc *store.Document, types []store.DocType, pages []store.ListRepoPagesRow,
) (DocumentHTMLDTO, error) {
	page, err := html.Render([]byte(doc.RawMd), html.RenderOptions{
		Theme:    snapshotTheme(repo.ConfigSnapshot),
		Spec:     headingSpec(doc.Type),
		Type:     doc.Type,
		Source:   doc.Path,
		Site:     repo.Name,
		Resolve:  siteResolver(repo, types, pages),
		Fragment: true,
	})
	if err != nil {
		return DocumentHTMLDTO{}, err
	}

	problems := make([]string, 0, len(page.Problems))
	for _, p := range page.Problems {
		problems = append(problems, p.Error())
	}

	return DocumentHTMLDTO{
		DocID: doc.DocID, HTML: string(page.Body), Hash: page.Hash,
		Inferred: page.Inferred, Problems: problems,
	}, nil
}

// headingSpec is a built-in type's heading spec, derived from its embedded
// template as `docz export html` derives it from the repository's, so an
// unmarked document's sections are inferred the same way. A custom type
// has none: its document lays out from its markers alone.
func headingSpec(typeName string) kinds.HeadingSpec {
	tmpl, err := doctemplate.EmbeddedDocumentTemplate(config.DocType(typeName))
	if err != nil {
		return nil
	}

	return kinds.SpecFromTemplate([]byte(tmpl))
}

// snapshotTheme is the repository's html.theme from its config snapshot,
// or "" (paper) when the snapshot names none or names one this server
// does not know, as config.Load would have normalised it.
func snapshotTheme(snapshot json.RawMessage) string {
	var cfg struct {
		HTML struct {
			Theme string `json:"theme"`
		} `json:"html"`
	}
	if json.Unmarshal(snapshot, &cfg) != nil || !slices.Contains(theme.Names(), cfg.HTML.Theme) {
		return ""
	}

	return cfg.HTML.Theme
}

// docFile is a docz document's file name: its number, then a slug.
var docFile = regexp.MustCompile(`^(\d+)-.+\.md$`)

// siteResolver maps a relative link in a document to docz-site's route for
// what it names: a document of an ingested type, by the type's directory
// and the file's number, or a published page by its repository path.
// Anything else is unresolved, and the fragment shows the link's text.
func siteResolver(repo *store.Repo, types []store.DocType, pages []store.ListRepoPagesRow) html.LinkResolver {
	base := "/" + repo.Owner + "/" + repo.Name

	byRepoPath := make(map[string]string, len(pages))
	for i := range pages {
		byRepoPath[pages[i].RepoPath] = pages[i].Path
	}

	return func(from, href string) string {
		target, frag, _ := strings.Cut(href, "#")
		if target == "" || strings.Contains(target, ":") || strings.HasPrefix(target, "/") {
			return ""
		}

		p := path.Clean(path.Join(path.Dir(from), target))
		if p == ".." || strings.HasPrefix(p, "../") {
			return ""
		}

		if frag != "" {
			frag = "#" + frag
		}

		if published, ok := byRepoPath[p]; ok {
			return base + "/pages/" + published + frag
		}

		m := docFile.FindStringSubmatch(path.Base(p))
		if m == nil {
			return ""
		}

		for i := range types {
			if path.Dir(p) == path.Join(repo.DocsDir, types[i].Dir) {
				return base + "/" + types[i].Name + "/" + types[i].IDPrefix + "-" + m[1] + frag
			}
		}

		return ""
	}
}

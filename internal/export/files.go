package export

import (
	"fmt"
	"path"
	"path/filepath"
	"testing/fstest"
	"unicode"

	"github.com/donaldgifford/docz/v2/internal/store"
	doczcfg "github.com/donaldgifford/docz/v2/pkg/doczcore/config"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/doctemplate"
	doczdoc "github.com/donaldgifford/docz/v2/pkg/doczcore/document"
	"github.com/donaldgifford/docz/v2/pkg/doczcore/index"
)

// files builds the repository tree an export reads, keyed by repository
// path (DESIGN-0021 §5 step 4): each document's markdown at its path, a
// README per enabled type generated from its documents under the embedded
// index header, the cached landing page at api.landing_page, and each
// published page at its repository path.
func files(in *store.ExportInputs, cfg *doczcfg.Config) (fstest.MapFS, error) {
	fsys := fstest.MapFS{}
	byType := make(map[string][]doczdoc.DocEntry)

	for i := range in.Documents {
		d := &in.Documents[i]
		fsys[d.Path] = &fstest.MapFile{Data: []byte(d.RawMd)}

		fm, err := doczdoc.ParseFrontmatter([]byte(d.RawMd))
		if err != nil {
			continue
		}

		byType[d.Type] = append(byType[d.Type], doczdoc.DocEntry{Frontmatter: fm, Filename: path.Base(d.Path)})
	}

	for _, typeName := range cfg.EnabledTypes() {
		readme, err := readme(cfg, typeName, byType[typeName])
		if err != nil {
			return nil, err
		}

		dir := filepath.ToSlash(cfg.TypeDir(typeName))
		fsys[path.Join(dir, doczcfg.IndexFileName)] = &fstest.MapFile{Data: readme}
	}

	if cfg.API.Enabled && cfg.API.LandingPage != "" && in.Repo.IndexSha.Valid {
		fsys[cfg.API.LandingPage] = &fstest.MapFile{Data: []byte(in.Repo.IndexMd.String)}
	}

	for i := range in.Pages {
		p := &in.Pages[i]
		fsys[p.RepoPath] = &fstest.MapFile{Data: []byte(p.RawMd)}
	}

	return fsys, nil
}

// readme is the README docz update would write for a type with no header
// override: the embedded header with an index table of docs spliced in. A
// repository's own templates/index_<type>.md is not fetched (INV-0020
// decision 2), so the embedded tiers are the only ones.
func readme(cfg *doczcfg.Config, typeName string, docs []doczdoc.DocEntry) ([]byte, error) {
	label := pluralLabel(cfg, typeName)

	header, err := doctemplate.EmbeddedIndexHeader(typeName, doctemplate.IndexHeaderData{
		TypeName: typeName, PluralLabel: label,
	})
	if err != nil {
		return nil, fmt.Errorf("index header for %s: %w", typeName, err)
	}

	out, _ := index.Splice(nil, header, index.GenerateTable(docs, "All "+label))

	return out, nil
}

// pluralLabel is the type's label as docz update resolves it: plural_label,
// else the type name with its first letter upper-cased.
func pluralLabel(cfg *doczcfg.Config, typeName string) string {
	if label := cfg.Types[typeName].PluralLabel; label != "" {
		return label
	}

	if typeName == "" {
		return typeName
	}

	r := []rune(typeName)
	r[0] = unicode.ToUpper(r[0])

	return string(r)
}

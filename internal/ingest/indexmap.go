package ingest

import (
	"crypto/sha256"
	"encoding/hex"
	"strconv"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/donaldgifford/docz-api/internal/search"
	"github.com/donaldgifford/docz-api/internal/store"
)

// primaryKey returns the Meilisearch primary key for a document, the composite
// "<repo_id>_<doc_id>". Meilisearch ids allow only [a-zA-Z0-9-_], so the
// separator is "_" (not the ":" of DESIGN-0001's illustration); repo_id is
// purely numeric, so the first "_" unambiguously splits the two parts. The key
// is internal to the index and never appears in the search response.
func primaryKey(repoID int64, docID string) string {
	return strconv.FormatInt(repoID, 10) + "_" + docID
}

// pageHashLen is how many hex chars of the path hash form a page's primary
// key: 16 (64 bits), collision-safe within a repo's page namespace.
const pageHashLen = 16

// pagePrimaryKey returns the Meilisearch primary key for a page,
// "<repo_id>_p_<hex(sha256(path))[:16]>". Published paths carry "/" and "."
// which Meilisearch ids reject, so the path is hashed rather than embedded;
// the "p" marker keeps the key out of the document namespace (doc ids are
// docz `<PREFIX>-<number>` ids, which never take that shape). Like the doc
// key, it is internal to the index and never appears in a response.
func pagePrimaryKey(repoID int64, path string) string {
	sum := sha256.Sum256([]byte(path))
	return strconv.FormatInt(repoID, 10) + "_p_" + hex.EncodeToString(sum[:])[:pageHashLen]
}

// toIndexDoc maps a stored document row to a search.IndexDoc. owner/name form
// the repo label and repoID is the repo's surrogate key; both come from the
// caller (ingest holds them from Run and the reconcile result). Created renders
// as "YYYY-MM-DD" (empty when NULL) and UpdatedAt as Unix seconds.
func toIndexDoc(owner, name string, repoID int64, d *store.Document) search.IndexDoc {
	var created string
	if d.Created.Valid {
		created = d.Created.Time.Format(createdLayout)
	}
	var updatedAt int64
	if d.UpdatedAt.Valid {
		updatedAt = d.UpdatedAt.Time.Unix()
	}
	return search.IndexDoc{
		ID:        primaryKey(repoID, d.DocID),
		Source:    search.SourceDoc,
		Repo:      owner + "/" + name,
		RepoID:    repoID,
		DocID:     d.DocID,
		Type:      d.Type,
		Title:     d.Title,
		Status:    nullableText(d.Status),
		Author:    nullableText(d.Author),
		Created:   created,
		Path:      d.Path,
		Body:      d.RawMd,
		UpdatedAt: updatedAt,
	}
}

// toIndexPage maps a stored page row to a search.IndexDoc. Pages carry the
// published path (the address the pages endpoints serve) and leave the
// doc-only fields (DocID/Type/Status/Author/Created) empty — the wire shape's
// "" convention for a facet that does not apply.
func toIndexPage(owner, name string, repoID int64, p *store.RepoPage) search.IndexDoc {
	var updatedAt int64
	if p.UpdatedAt.Valid {
		updatedAt = p.UpdatedAt.Time.Unix()
	}
	return search.IndexDoc{
		ID:        pagePrimaryKey(repoID, p.Path),
		Source:    search.SourcePage,
		Repo:      owner + "/" + name,
		RepoID:    repoID,
		Title:     p.Title,
		Path:      p.Path,
		Body:      p.RawMd,
		UpdatedAt: updatedAt,
	}
}

// nullableText flattens a pgtype.Text to a plain string ("" when NULL).
func nullableText(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

package ingest

import (
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
		Repo:      owner + "/" + name,
		RepoID:    repoID,
		DocID:     d.DocID,
		Type:      d.Type,
		Title:     d.Title,
		Status:    nullableText(d.Status),
		Author:    nullableText(d.Author),
		Created:   created,
		Body:      d.RawMd,
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

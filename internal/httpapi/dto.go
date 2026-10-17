package httpapi

import (
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/donaldgifford/docz/v2/internal/store"
)

// The DTOs below are the wire shapes documented in DESIGN-0001. They are
// deliberately separate from the sqlc row types so nullable pgtype columns are
// flattened to plain strings ("" for NULL) and never leak into the API.

// RepoSummaryDTO is one entry of GET /repos.
type RepoSummaryDTO struct {
	Repo          string `json:"repo"`
	DefaultBranch string `json:"default_branch"`
	DocsDir       string `json:"docs_dir"`
	LastSyncedSHA string `json:"last_synced_sha"`
}

// RepoDetailDTO is GET /repos/{owner}/{name}.
type RepoDetailDTO struct {
	Repo           string          `json:"repo"`
	DefaultBranch  string          `json:"default_branch"`
	DocsDir        string          `json:"docs_dir"`
	LastSyncedSHA  string          `json:"last_synced_sha"`
	ConfigSnapshot json.RawMessage `json:"config_snapshot"`
	Types          []TypeDTO       `json:"types"`
}

// TypeDTO is one document type of a repository.
type TypeDTO struct {
	Name        string   `json:"name"`
	Dir         string   `json:"dir"`
	IDPrefix    string   `json:"id_prefix"`
	PluralLabel string   `json:"plural_label"`
	Statuses    []string `json:"statuses"`
	Aliases     []string `json:"aliases"`
}

// RepoIndexDTO is GET /repos/{owner}/{name}/index.
type RepoIndexDTO struct {
	Repo     string `json:"repo"`
	IndexMD  string `json:"index_md"`
	IndexSHA string `json:"index_sha"`
}

// RepoChangelogDTO is GET /repos/{owner}/{name}/changelog.
type RepoChangelogDTO struct {
	Repo         string `json:"repo"`
	ChangelogMD  string `json:"changelog_md"`
	ChangelogSHA string `json:"changelog_sha"`
}

// PageSummaryDTO is one entry of GET /repos/{owner}/{name}/pages.
type PageSummaryDTO struct {
	Path   string `json:"path"`
	Title  string `json:"title"`
	GitSHA string `json:"git_sha"`
}

// PageDTO is GET /repos/{owner}/{name}/pages/{path}.
type PageDTO struct {
	Repo   string `json:"repo"`
	Path   string `json:"path"`
	Title  string `json:"title"`
	RawMD  string `json:"raw_md"`
	GitSHA string `json:"git_sha"`
}

// DocumentDTO is a document, with raw_md only from the single-document read.
type DocumentDTO struct {
	Repo        string `json:"repo"`
	DocID       string `json:"doc_id"`
	Type        string `json:"type"`
	Title       string `json:"title"`
	Status      string `json:"status"`
	Author      string `json:"author"`
	Created     string `json:"created"`
	Path        string `json:"path"`
	GitSHA      string `json:"git_sha"`
	ContentHash string `json:"content_hash"`
	UpdatedAt   string `json:"updated_at"`
	// ConfluenceURL is the document's page when its last export left one,
	// "" otherwise (DESIGN-0021 §7).
	ConfluenceURL string `json:"confluence_url"`
	RawMD         string `json:"raw_md,omitempty"`
}

// RepoLabel is the "owner/name" identifier used across the DTOs.
func RepoLabel(r *store.Repo) string { return r.Owner + "/" + r.Name }

// ToRepoSummary maps a repository row to its list entry.
func ToRepoSummary(r *store.Repo) RepoSummaryDTO {
	return RepoSummaryDTO{
		Repo:          RepoLabel(r),
		DefaultBranch: r.DefaultBranch,
		DocsDir:       r.DocsDir,
		LastSyncedSHA: nullText(r.LastSyncedSha),
	}
}

// ToRepoDetail maps a repository row and its types to the detail shape.
func ToRepoDetail(r *store.Repo, types []store.DocType) RepoDetailDTO {
	return RepoDetailDTO{
		Repo:           RepoLabel(r),
		DefaultBranch:  r.DefaultBranch,
		DocsDir:        r.DocsDir,
		LastSyncedSHA:  nullText(r.LastSyncedSha),
		ConfigSnapshot: r.ConfigSnapshot,
		Types:          ToTypeDTOs(types),
	}
}

// ToTypeDTOs maps doc_types rows to the wire shape.
func ToTypeDTOs(types []store.DocType) []TypeDTO {
	dtos := make([]TypeDTO, len(types))
	for i := range types {
		dt := &types[i]
		dtos[i] = TypeDTO{
			Name:        dt.Name,
			Dir:         dt.Dir,
			IDPrefix:    dt.IDPrefix,
			PluralLabel: dt.PluralLabel,
			Statuses:    jsonStrings(dt.Statuses),
			Aliases:     jsonStrings(dt.Aliases),
		}
	}
	return dtos
}

// ToRepoIndex maps the cached repo home (DESIGN-0003). An empty-but-present
// index.md is stored as a NULL body with a valid sha, so nullText yields the
// "" body the contract promises; callers gate existence on IndexSha.Valid.
func ToRepoIndex(r *store.Repo) RepoIndexDTO {
	return RepoIndexDTO{
		Repo:     RepoLabel(r),
		IndexMD:  nullText(r.IndexMd),
		IndexSHA: nullText(r.IndexSha),
	}
}

// ToRepoChangelog maps the cached changelog (IMPL-0005) on the same terms as
// ToRepoIndex: an empty-but-present file is a NULL body with a valid sha, so
// nullText yields "" and callers gate existence on ChangelogSha.Valid.
func ToRepoChangelog(r *store.Repo) RepoChangelogDTO {
	return RepoChangelogDTO{
		Repo:         RepoLabel(r),
		ChangelogMD:  nullText(r.ChangelogMd),
		ChangelogSHA: nullText(r.ChangelogSha),
	}
}

// ToPageSummary maps a page row to its list entry.
func ToPageSummary(p *store.ListRepoPagesRow) PageSummaryDTO {
	return PageSummaryDTO{Path: p.Path, Title: p.Title, GitSHA: p.GitSha}
}

// ToPage maps one page row (DESIGN-0004). Every column is NOT NULL — row
// existence is the presence signal, so there is no sha-gating subtlety here;
// an empty-but-present page serves raw_md "".
func ToPage(repo string, p *store.RepoPage) PageDTO {
	return PageDTO{Repo: repo, Path: p.Path, Title: p.Title, RawMD: p.RawMd, GitSHA: p.GitSha}
}

// ToDocument maps the single-document read, raw markdown included.
func ToDocument(repo string, row *store.GetDocumentByIDRow) DocumentDTO {
	d := &row.Document
	return DocumentDTO{
		Repo:          repo,
		DocID:         d.DocID,
		Type:          d.Type,
		Title:         d.Title,
		Status:        nullText(d.Status),
		Author:        nullText(d.Author),
		Created:       nullDate(d.Created),
		Path:          d.Path,
		GitSHA:        d.GitSha,
		ContentHash:   d.ContentHash,
		UpdatedAt:     nullTimestamp(d.UpdatedAt),
		ConfluenceURL: row.ConfluenceUrl,
		RawMD:         d.RawMd,
	}
}

// ToDocumentSummary maps a list row: the document without its markdown.
func ToDocumentSummary(repo string, d *store.ListDocumentsByTypeRow) DocumentDTO {
	return DocumentDTO{
		Repo:          repo,
		DocID:         d.DocID,
		Type:          d.Type,
		Title:         d.Title,
		Status:        nullText(d.Status),
		Author:        nullText(d.Author),
		Created:       nullDate(d.Created),
		Path:          d.Path,
		GitSHA:        d.GitSha,
		ContentHash:   d.ContentHash,
		UpdatedAt:     nullTimestamp(d.UpdatedAt),
		ConfluenceURL: d.ConfluenceUrl,
	}
}

// jsonStrings unmarshals a JSONB string array, returning an empty (non-nil)
// slice for null/empty/invalid input so the field serializes as [].
func jsonStrings(raw json.RawMessage) []string {
	out := []string{}
	if len(raw) == 0 {
		return out
	}
	if err := json.Unmarshal(raw, &out); err != nil {
		return []string{}
	}
	return out
}

func nullText(t pgtype.Text) string {
	if t.Valid {
		return t.String
	}
	return ""
}

func nullDate(d pgtype.Date) string {
	if d.Valid {
		return d.Time.Format("2006-01-02")
	}
	return ""
}

// nullTimestamp renders a nullable timestamp as RFC3339 in UTC ("" when
// NULL). The zone is pinned because pgx scans a timestamptz into the
// process's own location unless ScanLocation is set, so an unpinned render
// carries the host's offset — "Z" in the distroless image, "-04:00" under a
// local `just run`. Pinning keeps the wire value identical on every host and
// byte-identical to the search layer's formatUnix (DESIGN-0005). Same
// instant either way.
func nullTimestamp(t pgtype.Timestamptz) string {
	if t.Valid {
		return t.Time.UTC().Format(time.RFC3339)
	}
	return ""
}

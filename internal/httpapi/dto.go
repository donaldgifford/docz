package httpapi

import (
	"encoding/json"
	"time"

	"github.com/jackc/pgx/v5/pgtype"

	"github.com/donaldgifford/docz-api/internal/store"
)

// The DTOs below are the wire shapes documented in DESIGN-0001. They are
// deliberately separate from the sqlc row types so nullable pgtype columns are
// flattened to plain strings ("" for NULL) and never leak into the API.

type repoSummaryDTO struct {
	Repo          string `json:"repo"`
	DefaultBranch string `json:"default_branch"`
	DocsDir       string `json:"docs_dir"`
	LastSyncedSHA string `json:"last_synced_sha"`
}

type repoDetailDTO struct {
	Repo           string          `json:"repo"`
	DefaultBranch  string          `json:"default_branch"`
	DocsDir        string          `json:"docs_dir"`
	LastSyncedSHA  string          `json:"last_synced_sha"`
	ConfigSnapshot json.RawMessage `json:"config_snapshot"`
	Types          []typeDTO       `json:"types"`
}

type typeDTO struct {
	Name        string   `json:"name"`
	Dir         string   `json:"dir"`
	IDPrefix    string   `json:"id_prefix"`
	PluralLabel string   `json:"plural_label"`
	Statuses    []string `json:"statuses"`
	Aliases     []string `json:"aliases"`
}

type documentDTO struct {
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
	RawMD       string `json:"raw_md,omitempty"`
}

// repoLabel is the "owner/name" identifier used across the DTOs.
func repoLabel(r *store.Repo) string { return r.Owner + "/" + r.Name }

func toRepoSummary(r *store.Repo) repoSummaryDTO {
	return repoSummaryDTO{
		Repo:          repoLabel(r),
		DefaultBranch: r.DefaultBranch,
		DocsDir:       r.DocsDir,
		LastSyncedSHA: nullText(r.LastSyncedSha),
	}
}

func toRepoDetail(r *store.Repo, types []store.DocType) repoDetailDTO {
	return repoDetailDTO{
		Repo:           repoLabel(r),
		DefaultBranch:  r.DefaultBranch,
		DocsDir:        r.DocsDir,
		LastSyncedSHA:  nullText(r.LastSyncedSha),
		ConfigSnapshot: r.ConfigSnapshot,
		Types:          toTypeDTOs(types),
	}
}

func toTypeDTOs(types []store.DocType) []typeDTO {
	dtos := make([]typeDTO, len(types))
	for i := range types {
		dt := &types[i]
		dtos[i] = typeDTO{
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

func toDocument(repo string, d *store.Document) documentDTO {
	return documentDTO{
		Repo:        repo,
		DocID:       d.DocID,
		Type:        d.Type,
		Title:       d.Title,
		Status:      nullText(d.Status),
		Author:      nullText(d.Author),
		Created:     nullDate(d.Created),
		Path:        d.Path,
		GitSHA:      d.GitSha,
		ContentHash: d.ContentHash,
		UpdatedAt:   nullTimestamp(d.UpdatedAt),
		RawMD:       d.RawMd,
	}
}

func toDocumentSummary(repo string, d *store.ListDocumentsByTypeRow) documentDTO {
	return documentDTO{
		Repo:        repo,
		DocID:       d.DocID,
		Type:        d.Type,
		Title:       d.Title,
		Status:      nullText(d.Status),
		Author:      nullText(d.Author),
		Created:     nullDate(d.Created),
		Path:        d.Path,
		GitSHA:      d.GitSha,
		ContentHash: d.ContentHash,
		UpdatedAt:   nullTimestamp(d.UpdatedAt),
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

func nullTimestamp(t pgtype.Timestamptz) string {
	if t.Valid {
		return t.Time.Format(time.RFC3339)
	}
	return ""
}

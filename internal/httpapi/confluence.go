package httpapi

import (
	"encoding/json"
	"errors"
	"net/http"

	"github.com/jackc/pgx/v5"

	"github.com/donaldgifford/docz/v2/internal/store"
)

// The repository's last Confluence export (DESIGN-0021 §7).
type (
	confluenceSyncDTO struct {
		Repo       string              `json:"repo"`
		Status     string              `json:"status"`
		Reason     string              `json:"reason"`
		Site       string              `json:"site"`
		Space      string              `json:"space"`
		Folder     confluenceFolderDTO `json:"folder"`
		HeadSHA    string              `json:"head_sha"`
		StartedAt  string              `json:"started_at"`
		FinishedAt string              `json:"finished_at"`
		Counts     confluenceCountsDTO `json:"counts"`
		Pages      []confluencePageDTO `json:"pages"`
	}

	confluenceFolderDTO struct {
		ID    string `json:"id"`
		Title string `json:"title"`
		URL   string `json:"url"`
	}

	confluenceCountsDTO struct {
		Created   int `json:"created"`
		Updated   int `json:"updated"`
		Unchanged int `json:"unchanged"`
		Skipped   int `json:"skipped"`
		Archived  int `json:"archived"`
		Failed    int `json:"failed"`
	}

	confluencePageDTO struct {
		Key          string             `json:"key"`
		Title        string             `json:"title"`
		Source       string             `json:"source"`
		Action       string             `json:"action"`
		PageID       string             `json:"page_id"`
		URL          string             `json:"url"`
		Version      int32              `json:"version"`
		Edited       *confluenceEditDTO `json:"edited,omitempty"`
		CommentsLost int32              `json:"comments_lost"`
		Reason       string             `json:"reason"`
	}

	confluenceEditDTO struct {
		Version  int32 `json:"version"`
		Expected int32 `json:"expected"`
	}
)

// Sync statuses only the API reports: no row yet, and the server's export
// switched off.
const (
	syncNever    = "never"
	syncDisabled = "disabled"

	reasonServerOff = "disabled on this server"
)

// getRepoConfluence serves GET /repos/{owner}/{name}/confluence. A
// repository never exported reads never with empty fields, not a 404, as
// the pages listing does for a repository with none.
func (h *Handler) getRepoConfluence(w http.ResponseWriter, r *http.Request) {
	repo, ok := h.resolveRepo(w, r)
	if !ok {
		return
	}

	out := confluenceSyncDTO{Repo: repoLabel(&repo), Status: syncNever, Pages: []confluencePageDTO{}}

	if !h.confluence {
		out.Status, out.Reason = syncDisabled, reasonServerOff
		writeJSON(w, out)
		return
	}

	sync, err := h.store.GetConfluenceSync(r.Context(), repo.ID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeJSON(w, out)
		return
	}
	if err != nil {
		serverError(w, "get confluence sync", err)
		return
	}

	pages, err := h.store.ListConfluencePages(r.Context(), repo.ID)
	if err != nil {
		serverError(w, "list confluence pages", err)
		return
	}

	writeJSON(w, toConfluenceSync(out.Repo, &sync, pages))
}

func toConfluenceSync(repo string, s *store.ConfluenceSync, pages []store.ConfluencePage) confluenceSyncDTO {
	out := confluenceSyncDTO{
		Repo:       repo,
		Status:     s.Status,
		Reason:     s.Reason,
		Site:       s.Site,
		Space:      s.Space,
		Folder:     confluenceFolderDTO{ID: s.FolderID, Title: s.FolderTitle, URL: s.FolderUrl},
		HeadSHA:    s.HeadSha,
		StartedAt:  nullTimestamp(s.StartedAt),
		FinishedAt: nullTimestamp(s.FinishedAt),
		Pages:      make([]confluencePageDTO, 0, len(pages)),
	}

	// The counts are the export's own JSON; one that will not decode reads
	// as zeros rather than failing the whole response.
	_ = json.Unmarshal(s.Counts, &out.Counts) //nolint:errcheck // zeros on a bad blob, by design

	for i := range pages {
		p := &pages[i]
		dto := confluencePageDTO{
			Key: p.Key, Title: p.Title, Source: p.Source, Action: p.Action, PageID: p.PageID,
			URL: p.Url, Version: p.Version, CommentsLost: p.CommentsLost, Reason: p.Reason,
		}
		if p.EditedFrom != 0 {
			dto.Edited = &confluenceEditDTO{Version: p.EditedFrom, Expected: p.EditedExpected}
		}
		out.Pages = append(out.Pages, dto)
	}

	return out
}

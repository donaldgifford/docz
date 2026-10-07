-- name: UpsertConfluenceSync :exec
-- The repository's last export run, overwritten by each attempt
-- (DESIGN-0021 §5 step 6).
INSERT INTO confluence_syncs (
    repo_id, status, reason, site, space, folder_id, folder_title, folder_url,
    head_sha, counts, started_at, finished_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12
)
ON CONFLICT (repo_id) DO UPDATE SET
    status       = EXCLUDED.status,
    reason       = EXCLUDED.reason,
    site         = EXCLUDED.site,
    space        = EXCLUDED.space,
    folder_id    = EXCLUDED.folder_id,
    folder_title = EXCLUDED.folder_title,
    folder_url   = EXCLUDED.folder_url,
    head_sha     = EXCLUDED.head_sha,
    counts       = EXCLUDED.counts,
    started_at   = EXCLUDED.started_at,
    finished_at  = EXCLUDED.finished_at;

-- name: GetConfluenceSync :one
SELECT * FROM confluence_syncs WHERE repo_id = $1;

-- name: UpsertConfluencePage :exec
-- One page's result. A failed page passes an empty page_id and url, and
-- keeps the previous row's, so a link that worked keeps working through a
-- failed run (DESIGN-0021 Data Model).
INSERT INTO confluence_pages (
    repo_id, key, doc_id, page_id, title, source, url, version, hash, action,
    reason, edited_from, edited_expected, comments_lost, synced_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15
)
ON CONFLICT (repo_id, key) DO UPDATE SET
    doc_id        = EXCLUDED.doc_id,
    page_id       = COALESCE(NULLIF(EXCLUDED.page_id, ''), confluence_pages.page_id),
    title         = EXCLUDED.title,
    source        = EXCLUDED.source,
    url           = COALESCE(NULLIF(EXCLUDED.url, ''), confluence_pages.url),
    version       = EXCLUDED.version,
    hash          = EXCLUDED.hash,
    action        = EXCLUDED.action,
    reason        = EXCLUDED.reason,
    edited_from   = EXCLUDED.edited_from,
    edited_expected = EXCLUDED.edited_expected,
    comments_lost = EXCLUDED.comments_lost,
    synced_at     = EXCLUDED.synced_at;

-- name: ListConfluencePages :many
SELECT * FROM confluence_pages WHERE repo_id = $1 ORDER BY key;

-- name: ListConfluencePageIDs :many
-- Key to recorded page id, for ExportOptions.Pages.
SELECT key, page_id FROM confluence_pages WHERE repo_id = $1 AND page_id <> '' ORDER BY key;

-- name: GetRepoByID :one
SELECT * FROM repos WHERE id = $1;

-- name: ListDocumentsForExport :many
-- Every document with its markdown, for the export's file tree.
SELECT * FROM documents WHERE repo_id = $1 ORDER BY type, doc_id;

-- name: ListRepoPagesForExport :many
-- Every published page with its markdown, for the export's file tree.
SELECT * FROM repo_pages WHERE repo_id = $1 ORDER BY path;

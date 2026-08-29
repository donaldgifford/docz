-- name: ListRepoPageHashes :many
-- The reconcile's content-hash gate read: which pages exist and what
-- content they carry.
SELECT path, content_hash FROM repo_pages WHERE repo_id = $1;

-- name: UpsertRepoPage :exec
INSERT INTO repo_pages (
    repo_id, path, repo_path, title, git_sha, content_hash, raw_md, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, now()
)
ON CONFLICT (repo_id, path) DO UPDATE SET
    repo_path    = EXCLUDED.repo_path,
    title        = EXCLUDED.title,
    git_sha      = EXCLUDED.git_sha,
    content_hash = EXCLUDED.content_hash,
    raw_md       = EXCLUDED.raw_md,
    updated_at   = now();

-- name: DeleteRepoPage :exec
DELETE FROM repo_pages WHERE repo_id = $1 AND path = $2;

-- name: ListRepoPages :many
-- Metadata only (no raw_md) for the list endpoint, ordered by the
-- published path.
SELECT id, repo_id, path, repo_path, title, git_sha, content_hash, updated_at
FROM repo_pages
WHERE repo_id = $1
ORDER BY path;

-- name: GetRepoPageByPath :one
-- Full row including raw_md for the single-page endpoint. The match is
-- exact-byte on the stored published path — no case folding (DESIGN-0004).
SELECT * FROM repo_pages WHERE repo_id = $1 AND path = $2;

-- name: GetRepoPagesByPaths :many
-- Full rows (including raw_md) for the search-index sync: the reconcile
-- reports which published paths changed, and this fetches their content.
SELECT * FROM repo_pages
WHERE repo_id = @repo_id AND path = ANY (@paths::text[]);

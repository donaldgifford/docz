-- name: ListDocumentHashes :many
SELECT doc_id, content_hash FROM documents WHERE repo_id = $1;

-- name: UpsertDocument :exec
INSERT INTO documents (
    repo_id, type, doc_id, title, status, author, created,
    path, git_sha, content_hash, raw_md, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, now()
)
ON CONFLICT (repo_id, doc_id) DO UPDATE SET
    type         = EXCLUDED.type,
    title        = EXCLUDED.title,
    status       = EXCLUDED.status,
    author       = EXCLUDED.author,
    created      = EXCLUDED.created,
    path         = EXCLUDED.path,
    git_sha      = EXCLUDED.git_sha,
    content_hash = EXCLUDED.content_hash,
    raw_md       = EXCLUDED.raw_md,
    updated_at   = now();

-- name: DeleteDocument :exec
DELETE FROM documents WHERE repo_id = $1 AND doc_id = $2;

-- name: ListDocumentsByType :many
-- Metadata only (no raw_md) for the list endpoint; type is the canonical name.
SELECT id, repo_id, type, doc_id, title, status, author, created,
       path, git_sha, content_hash, updated_at
FROM documents
WHERE repo_id = $1 AND type = $2
ORDER BY doc_id;

-- name: GetDocumentByID :one
-- Full row including raw_md for the single-doc endpoint.
SELECT * FROM documents WHERE repo_id = $1 AND doc_id = $2;

-- name: GetDocumentsByIDs :many
-- Full rows (including raw_md) for a set of doc ids in one repo; the search
-- indexer uses this to build index documents after a reconcile commit.
SELECT * FROM documents
WHERE repo_id = @repo_id AND doc_id = ANY(@doc_ids::text[])
ORDER BY doc_id;

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
-- confluence_url is the page the last export wrote the document to, '' when
-- there is none (DESIGN-0021 §7).
SELECT d.id, d.repo_id, d.type, d.doc_id, d.title, d.status, d.author, d.created,
       d.path, d.git_sha, d.content_hash, d.updated_at,
       COALESCE(cp.url, '')::text AS confluence_url
FROM documents d
LEFT JOIN confluence_pages cp ON cp.repo_id = d.repo_id AND cp.doc_id = d.doc_id
WHERE d.repo_id = $1 AND d.type = $2
ORDER BY d.doc_id;

-- name: GetDocumentByID :one
-- Full row including raw_md for the single-doc endpoint, with the document's
-- confluence_url as ListDocumentsByType reads it.
SELECT sqlc.embed(d), COALESCE(cp.url, '')::text AS confluence_url
FROM documents d
LEFT JOIN confluence_pages cp ON cp.repo_id = d.repo_id AND cp.doc_id = d.doc_id
WHERE d.repo_id = $1 AND d.doc_id = $2;

-- name: GetDocumentsByIDs :many
-- Full rows (including raw_md) for a set of doc ids in one repo; the search
-- indexer uses this to build index documents after a reconcile commit.
SELECT * FROM documents
WHERE repo_id = @repo_id AND doc_id = ANY(@doc_ids::text[])
ORDER BY doc_id;

-- name: ListDocTypes :many
SELECT id, repo_id, name, dir, id_prefix, plural_label, statuses, aliases
FROM doc_types
WHERE repo_id = $1;

-- name: UpsertDocType :exec
INSERT INTO doc_types (repo_id, name, dir, id_prefix, plural_label, statuses, aliases)
VALUES ($1, $2, $3, $4, $5, $6, $7)
ON CONFLICT (repo_id, name) DO UPDATE SET
    dir          = EXCLUDED.dir,
    id_prefix    = EXCLUDED.id_prefix,
    plural_label = EXCLUDED.plural_label,
    statuses     = EXCLUDED.statuses,
    aliases      = EXCLUDED.aliases;

-- name: DeleteDocType :exec
DELETE FROM doc_types WHERE repo_id = $1 AND name = $2;

-- name: UpsertRepo :one
INSERT INTO repos (
    installation_id, owner, name, default_branch, docs_dir, config_snapshot,
    last_synced_sha, last_synced_at, changelog_md, changelog_sha, updated_at
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, now(), $8, $9, now()
)
ON CONFLICT (owner, name) DO UPDATE SET
    installation_id = EXCLUDED.installation_id,
    default_branch  = EXCLUDED.default_branch,
    docs_dir        = EXCLUDED.docs_dir,
    config_snapshot = EXCLUDED.config_snapshot,
    last_synced_sha = EXCLUDED.last_synced_sha,
    last_synced_at  = now(),
    changelog_md    = EXCLUDED.changelog_md,
    changelog_sha   = EXCLUDED.changelog_sha,
    updated_at      = now()
RETURNING id;

-- name: GetRepoByOwnerName :one
SELECT * FROM repos WHERE owner = $1 AND name = $2;

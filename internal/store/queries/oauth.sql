-- name: InsertRefreshToken :one
INSERT INTO oauth_refresh_tokens (token_hash, family_id, user_id, client_id, scope)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;

-- name: GetRefreshTokenByHash :one
SELECT * FROM oauth_refresh_tokens WHERE token_hash = $1;

-- name: RotateRefreshToken :one
-- Marks the live token used and inserts its successor in the same family, in
-- one statement, so a crash between the two cannot leave both live. Returns
-- no row when the old token is not live (unknown, used, or revoked); the
-- caller then treats a used token as reuse and revokes the family. The new
-- scope is the caller's, recomputed from the person's current groups.
WITH used AS (
    UPDATE oauth_refresh_tokens AS t
    SET last_used_at = now()
    WHERE t.token_hash = sqlc.arg(old_hash)::text AND t.last_used_at IS NULL AND t.revoked_at IS NULL
    RETURNING t.family_id, t.user_id, t.client_id
)
INSERT INTO oauth_refresh_tokens (token_hash, family_id, user_id, client_id, scope)
SELECT sqlc.arg(new_hash)::text, u.family_id, u.user_id, u.client_id, sqlc.arg(scope)::text FROM used AS u
RETURNING oauth_refresh_tokens.*;

-- name: RevokeRefreshFamily :execrows
UPDATE oauth_refresh_tokens
SET revoked_at = now()
WHERE family_id = $1 AND revoked_at IS NULL;

-- name: UpsertConsent :exec
INSERT INTO oauth_consents (user_id, client_id, scope)
VALUES ($1, $2, $3)
ON CONFLICT (user_id, client_id) DO UPDATE SET
    scope      = EXCLUDED.scope,
    granted_at = now(),
    revoked_at = NULL;

-- name: GetConsent :one
SELECT * FROM oauth_consents
WHERE user_id = $1 AND client_id = $2 AND revoked_at IS NULL;

-- name: ListRepoIDsForScopes :many
-- The repositories any of the given scopes unlocks (DESIGN-0023 §7): the
-- JSONB ?| operator is true when the array holds any of the strings.
SELECT id FROM repos
WHERE access_scopes ?| @scopes::text[]
ORDER BY id;

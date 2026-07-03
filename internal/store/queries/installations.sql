-- name: UpsertInstallation :exec
INSERT INTO installations (id, account_login, account_type)
VALUES ($1, $2, $3)
ON CONFLICT (id) DO UPDATE SET
    account_login = EXCLUDED.account_login,
    account_type  = EXCLUDED.account_type,
    updated_at    = now();

-- name: GetInstallation :one
SELECT * FROM installations WHERE id = $1;

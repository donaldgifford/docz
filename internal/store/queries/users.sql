-- name: UpsertUser :one
-- Creates or updates the durable user row for a provider identity, refreshing
-- the email/login on each login. Sessions live in Redis, not here; this row is
-- the audit record of who has logged in. Returns the user id.
INSERT INTO users (provider, subject, email, login)
VALUES (@provider, @subject, @email, @login)
ON CONFLICT (provider, subject) DO UPDATE SET
    email = EXCLUDED.email,
    login = EXCLUDED.login
RETURNING id;

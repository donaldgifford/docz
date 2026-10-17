-- +goose Up

-- Scoped access and docz-api's own authorization server (DESIGN-0023 Data
-- Model, IMPL-0026 Phase 2).

-- The scopes that unlock a repository, from its .docz.yaml access.scopes.
-- Backfilled to ["public:read"], so every repository already ingested stays
-- readable by every signed-in principal until its next ingest says otherwise.
ALTER TABLE repos ADD COLUMN access_scopes JSONB NOT NULL DEFAULT '["public:read"]';
CREATE INDEX repos_access_scopes ON repos USING GIN (access_scopes);

-- Refresh tokens, stored hashed. A rotation marks the old row used and inserts
-- the new one in the same family; presenting a used token revokes the family.
-- An access token is a JWT and is never stored.
CREATE TABLE oauth_refresh_tokens (
    id           BIGSERIAL   PRIMARY KEY,
    token_hash   TEXT        NOT NULL UNIQUE,
    family_id    TEXT        NOT NULL,
    user_id      BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    client_id    TEXT        NOT NULL,            -- the Client ID Metadata Document URL
    scope        TEXT        NOT NULL DEFAULT '', -- space-separated
    issued_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_used_at TIMESTAMPTZ,                     -- set when rotated out
    revoked_at   TIMESTAMPTZ
);

CREATE INDEX oauth_refresh_tokens_family ON oauth_refresh_tokens (family_id);

-- A person's consent for a client, with the scope it was given for; a request
-- for a wider scope asks again.
CREATE TABLE oauth_consents (
    user_id    BIGINT      NOT NULL REFERENCES users (id) ON DELETE CASCADE,
    client_id  TEXT        NOT NULL,
    scope      TEXT        NOT NULL DEFAULT '',
    granted_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revoked_at TIMESTAMPTZ,
    PRIMARY KEY (user_id, client_id)
);

-- +goose Down

DROP TABLE oauth_consents;
DROP TABLE oauth_refresh_tokens;
DROP INDEX repos_access_scopes;
ALTER TABLE repos DROP COLUMN access_scopes;

-- +goose Up

-- A GitHub App installation (one per org/account that installed the app).
CREATE TABLE installations (
    id            BIGINT PRIMARY KEY,            -- GitHub installation id
    account_login TEXT        NOT NULL,          -- org or user that installed
    account_type  TEXT        NOT NULL,          -- 'Organization' | 'User'
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- An onboarded repository. config_snapshot is the parsed .docz.yaml;
-- changelog_md caches the raw root CHANGELOG.md (OQ 10), not parsed.
CREATE TABLE repos (
    id              BIGSERIAL PRIMARY KEY,
    installation_id BIGINT      NOT NULL REFERENCES installations(id) ON DELETE CASCADE,
    owner           TEXT        NOT NULL,
    name            TEXT        NOT NULL,
    default_branch  TEXT        NOT NULL,
    docs_dir        TEXT        NOT NULL,          -- from .docz.yaml
    config_snapshot JSONB       NOT NULL,          -- full parsed .docz.yaml
    last_synced_sha TEXT,                          -- default-branch HEAD last ingested
    last_synced_at  TIMESTAMPTZ,
    changelog_md    TEXT,                          -- cached raw CHANGELOG.md (NOT parsed)
    changelog_sha   TEXT,                          -- blob sha of the cached CHANGELOG
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (owner, name)
);

-- Per-repo doc types, driven entirely by .docz.yaml (custom types included).
CREATE TABLE doc_types (
    id           BIGSERIAL PRIMARY KEY,
    repo_id      BIGINT NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    name         TEXT   NOT NULL,                  -- canonical, e.g. 'rfc','frameworks'
    dir          TEXT   NOT NULL,                  -- e.g. 'rfc'
    id_prefix    TEXT   NOT NULL,                  -- e.g. 'RFC','FW'
    plural_label TEXT   NOT NULL,                  -- display label
    statuses     JSONB  NOT NULL,                  -- ["Draft","Accepted",…]
    aliases      JSONB  NOT NULL DEFAULT '[]',     -- per-type CLI shorthands
    UNIQUE (repo_id, name)
);

-- One row per docz document at the default-branch HEAD.
CREATE TABLE documents (
    id           BIGSERIAL PRIMARY KEY,
    repo_id      BIGINT NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    type         TEXT   NOT NULL,                  -- canonical type name
    doc_id       TEXT   NOT NULL,                  -- frontmatter id, e.g. 'RFC-0001'
    title        TEXT   NOT NULL,
    status       TEXT,
    author       TEXT,
    created      DATE,                             -- frontmatter created
    path         TEXT   NOT NULL,                  -- repo-relative path
    git_sha      TEXT   NOT NULL,                  -- blob sha of the file
    content_hash TEXT   NOT NULL,                  -- sha256 of raw_md (re-ingest gate)
    raw_md       TEXT   NOT NULL,                  -- cached markdown (NOT html)
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (repo_id, doc_id)
);

CREATE INDEX documents_repo_type_idx ON documents (repo_id, type);
CREATE INDEX documents_status_idx    ON documents (status);

-- Site users: one durable row per provider identity that has logged in.
-- Sessions themselves live in Redis, not here.
CREATE TABLE users (
    id         BIGSERIAL PRIMARY KEY,
    provider   TEXT NOT NULL,                      -- 'github' | 'okta' | 'keycloak'
    subject    TEXT NOT NULL,                      -- stable per-provider id
    email      TEXT,
    login      TEXT,                               -- github login when present
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (provider, subject)
);

-- Processed webhook deliveries, for idempotency (DESIGN-0001 Open Question 9).
CREATE TABLE webhook_deliveries (
    delivery_id TEXT PRIMARY KEY,                  -- X-GitHub-Delivery
    event       TEXT        NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down

DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS users;
DROP TABLE IF EXISTS documents;
DROP TABLE IF EXISTS doc_types;
DROP TABLE IF EXISTS repos;
DROP TABLE IF EXISTS installations;

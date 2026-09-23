-- +goose Up

-- The second record type (DESIGN-0004): non-docz markdown a repo's api:
-- block publishes — docs_dir pages, directory pages, additional_docs.
-- Mirrors documents minus the docz-only columns (no doc_id/type/status/
-- author/created); the published path is the addressing key. Row
-- existence is the presence signal, so an empty-but-present page stores
-- raw_md = '' with a valid git_sha (no sha-gating subtlety here).
CREATE TABLE repo_pages (
    id           BIGSERIAL PRIMARY KEY,
    repo_id      BIGINT NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    path         TEXT   NOT NULL,                  -- published path (the addressing key)
    repo_path    TEXT   NOT NULL,                  -- actual file path in the repo
    title        TEXT   NOT NULL,                  -- docparse.Title, fallback applied at ingest
    git_sha      TEXT   NOT NULL,                  -- blob sha (cache key, like index_sha)
    content_hash TEXT   NOT NULL,                  -- sha256 of raw_md (re-ingest gate)
    raw_md       TEXT   NOT NULL,                  -- cached markdown (NOT html)
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (repo_id, path)
);

CREATE INDEX repo_pages_repo_idx ON repo_pages (repo_id);

-- The webhook's exact-match surface for api-block files living outside
-- docs_dir (the changelog_file precedent, pluralized). Written by the
-- reconcile from the post-Load config; NULL when the block is disabled —
-- desired state, not a cache.
ALTER TABLE repos
    ADD COLUMN api_landing_page    TEXT,           -- resolved landing_page (NULL = block disabled)
    ADD COLUMN api_additional_docs JSONB;          -- normalized additional_docs array (NULL = block disabled)

-- +goose Down

ALTER TABLE repos
    DROP COLUMN api_additional_docs,
    DROP COLUMN api_landing_page;

DROP TABLE repo_pages;

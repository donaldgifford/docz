-- +goose Up

-- Cache the repo home document (docs_dir/index.md — the file docz's wiki
-- renders as its landing page) on the repo row, mirroring the changelog pair
-- (DESIGN-0003). NULL means the file was absent at the last ingest.
ALTER TABLE repos
    ADD COLUMN index_md  TEXT,                     -- cached raw docs_dir/index.md (NOT parsed)
    ADD COLUMN index_sha TEXT;                     -- blob sha of the cached index.md

-- +goose Down

ALTER TABLE repos
    DROP COLUMN index_md,
    DROP COLUMN index_sha;

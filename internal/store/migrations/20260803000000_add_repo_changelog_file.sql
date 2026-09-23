-- +goose Up

-- Record which file the cached changelog pair came from: the repo-root-relative
-- path resolved from the .docz.yaml changelog: block (IMPL-0005). NULL means the
-- block was absent or disabled at the last ingest, which is also when the cached
-- pair is nulled. The webhook reads this to decide whether a push touching only
-- the changelog warrants a re-ingest — that file lives outside docs_dir, so the
-- docs_dir prefix check alone would never see it.
ALTER TABLE repos
    ADD COLUMN changelog_file TEXT;                -- resolved changelog path (NULL = not configured)

-- +goose Down

ALTER TABLE repos
    DROP COLUMN changelog_file;

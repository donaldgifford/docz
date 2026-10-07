-- +goose Up

-- The Confluence export's record (DESIGN-0021 Data Model): the last run per
-- repository, and the page each property key was written to. Both cascade
-- from repos, so an uninstalled repository's records go with it while its
-- Confluence pages stay where they are. Columns are NOT NULL with empty
-- defaults so sqlc generates plain strings and ints.
CREATE TABLE confluence_syncs (
    repo_id      BIGINT PRIMARY KEY REFERENCES repos (id) ON DELETE CASCADE,
    status       TEXT        NOT NULL,           -- disabled | refused | running | succeeded | partial | failed
    reason       TEXT        NOT NULL DEFAULT '',
    site         TEXT        NOT NULL DEFAULT '',
    space        TEXT        NOT NULL DEFAULT '',
    folder_id    TEXT        NOT NULL DEFAULT '',
    folder_title TEXT        NOT NULL DEFAULT '',
    folder_url   TEXT        NOT NULL DEFAULT '',
    head_sha     TEXT        NOT NULL DEFAULT '',
    counts       JSONB       NOT NULL DEFAULT '{}',
    started_at   TIMESTAMPTZ NOT NULL,
    finished_at  TIMESTAMPTZ
);

CREATE TABLE confluence_pages (
    repo_id       BIGINT      NOT NULL REFERENCES repos (id) ON DELETE CASCADE,
    key           TEXT        NOT NULL,          -- doc id, docz:parent, docz:type:<name>, docz:page:<path>
    doc_id        TEXT        NOT NULL DEFAULT '',
    page_id       TEXT        NOT NULL,
    title         TEXT        NOT NULL,
    url           TEXT        NOT NULL DEFAULT '',
    version       INTEGER     NOT NULL DEFAULT 0,
    hash          TEXT        NOT NULL DEFAULT '',
    action        TEXT        NOT NULL,
    reason        TEXT        NOT NULL DEFAULT '',
    edited_from   INTEGER     NOT NULL DEFAULT 0, -- the Confluence version overwritten, 0 if none
    comments_lost INTEGER     NOT NULL DEFAULT 0,
    synced_at     TIMESTAMPTZ NOT NULL,
    PRIMARY KEY (repo_id, key)
);

CREATE INDEX confluence_pages_doc ON confluence_pages (repo_id, doc_id) WHERE doc_id <> '';

-- +goose Down

DROP TABLE confluence_pages;
DROP TABLE confluence_syncs;

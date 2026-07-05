package search

import (
	"context"
	"fmt"
	"strconv"
)

// IndexDocuments adds or replaces docs in the index by primary key, blocking
// until the write task completes so a subsequent search reflects it. An empty
// slice is a no-op. The document set is gated upstream by the same content_hash
// check Postgres uses, so unchanged documents are never re-sent.
func (c *Client) IndexDocuments(ctx context.Context, docs []IndexDoc) error {
	if len(docs) == 0 {
		return nil
	}
	task, err := c.svc.Index(indexUID).AddDocumentsWithContext(ctx, docs, nil)
	if err != nil {
		return fmt.Errorf("add %d documents to index: %w", len(docs), err)
	}
	return c.waitTask(ctx, task.TaskUID)
}

// DeleteDocuments removes documents by primary key ("<repo_id>_<doc_id>"),
// blocking until the delete task completes. An empty slice is a no-op.
func (c *Client) DeleteDocuments(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}
	task, err := c.svc.Index(indexUID).DeleteDocumentsWithContext(ctx, ids, nil)
	if err != nil {
		return fmt.Errorf("delete %d documents from index: %w", len(ids), err)
	}
	return c.waitTask(ctx, task.TaskUID)
}

// DeleteRepoDocuments removes every document for one repo from the index,
// matching on the filterable "repo_id" attribute, and blocks until the delete
// task completes. It backs offboarding (uninstall / repo removal), where the
// Postgres rows are gone (ON DELETE CASCADE) but the index still holds the
// repo's documents. repo_id is a positive serial, so the filter value needs no
// escaping.
func (c *Client) DeleteRepoDocuments(ctx context.Context, repoID int64) error {
	filter := "repo_id = " + strconv.FormatInt(repoID, 10)
	task, err := c.svc.Index(indexUID).DeleteDocumentsByFilterWithContext(ctx, filter, nil)
	if err != nil {
		return fmt.Errorf("delete documents for repo %d from index: %w", repoID, err)
	}
	return c.waitTask(ctx, task.TaskUID)
}

package search

import (
	"context"
	"fmt"
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

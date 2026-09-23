package search

import (
	"context"
	"fmt"
	"time"

	"github.com/meilisearch/meilisearch-go"
)

// Index schema identifiers.
const (
	// indexUID is the Meilisearch index holding every onboarded document.
	indexUID = "documents"
	// primaryKeyField names the composite key "<repo_id>_<doc_id>" on each document.
	primaryKeyField = "id"
)

// taskPollInterval is how often index writes poll Meilisearch task status.
const taskPollInterval = 50 * time.Millisecond

// sortableAttributes are the index attributes a search may order by. The
// accepted sort tokens are derived from this list (see ParseSort), so adding
// an attribute here plus its two direction tokens is the whole change.
var sortableAttributes = []string{"created", "updated_at"}

// rankingRules order Meilisearch's ranking, with "sort" ahead of the
// relevance rules. That position is what makes a requested sort a total
// order over the matches instead of a tie-break within relevance: at the
// default position ("words, typo, proximity, attribute, sort, exactness") a
// query's most relevant hit leads regardless of the sort the caller asked
// for. The move is safe because the sort rule is inert on a search that
// carries no sort parameter, so unsorted requests rank exactly as before
// (DESIGN-0005). Relevance still orders records that tie on the sort keys.
var rankingRules = []string{
	"sort", "words", "typo", "proximity", "attribute", "exactness",
}

// Client is the Meilisearch access layer. One Client serves the whole process;
// it satisfies the ingest.Indexer and httpapi.Searcher interfaces.
type Client struct {
	svc meilisearch.ServiceManager
}

// New builds a Client for the Meilisearch host, authenticating with apiKey.
// meilisearch.New never fails (it is a value constructor); reachability is
// checked by Health and when EnsureIndex first talks to the server.
func New(host, apiKey string) *Client {
	return &Client{svc: meilisearch.New(host, meilisearch.WithAPIKey(apiKey))}
}

// EnsureIndex creates the documents index if absent and applies its settings,
// idempotently. It is safe to call on every startup and blocks until the
// settings task completes. Meilisearch serializes tasks per index in FIFO
// order, so the create enqueued here runs before the settings update below,
// giving a fresh index its "id" primary key. On an existing index the create
// task fails harmlessly (it is never waited on) and only the settings apply.
func (c *Client) EnsureIndex(ctx context.Context) error {
	if _, err := c.svc.CreateIndexWithContext(ctx, &meilisearch.IndexConfig{
		Uid:        indexUID,
		PrimaryKey: primaryKeyField,
	}); err != nil {
		return fmt.Errorf("create index %q: %w", indexUID, err)
	}

	// SearchableAttributes order sets relevance priority: title outranks body.
	task, err := c.svc.Index(indexUID).UpdateSettingsWithContext(ctx, &meilisearch.Settings{
		SearchableAttributes: []string{"title", attrBody},
		FilterableAttributes: []string{attrRepo, "repo_id", attrType, attrStatus, attrAuthor, attrSource},
		SortableAttributes:   sortableAttributes,
		RankingRules:         rankingRules,
	})
	if err != nil {
		return fmt.Errorf("update settings for index %q: %w", indexUID, err)
	}
	return c.waitTask(ctx, task.TaskUID)
}

// Health reports whether Meilisearch is reachable; it backs the /readyz probe.
func (c *Client) Health(ctx context.Context) error {
	if _, err := c.svc.HealthWithContext(ctx); err != nil {
		return fmt.Errorf("meilisearch health: %w", err)
	}
	return nil
}

// waitTask blocks until the Meilisearch task reaches a terminal state, returning
// an error if it did not succeed. WaitForTask itself only errors on context
// cancellation or a fetch failure, so a failed task is inspected explicitly.
func (c *Client) waitTask(ctx context.Context, taskUID int64) error {
	task, err := c.svc.WaitForTaskWithContext(ctx, taskUID, taskPollInterval)
	if err != nil {
		return fmt.Errorf("wait for task %d: %w", taskUID, err)
	}
	if task.Status != meilisearch.TaskStatusSucceeded {
		return fmt.Errorf("meilisearch task %d finished with status %q: %s",
			taskUID, task.Status, task.Error.Message)
	}
	return nil
}

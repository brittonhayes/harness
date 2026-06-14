package brain

import "context"

// RecallBackend identifies the path that answered a recall request.
type RecallBackend string

const (
	// RecallBackendSearch means a configured search backend answered the query.
	// For the Notion brain, setup wires this to the reserved Notion MCP server.
	RecallBackendSearch RecallBackend = "search_backend"
	// RecallBackendScan means recall queried the store and filtered rows locally.
	RecallBackendScan RecallBackend = "window_scan"
	// RecallBackendSearchFailed means a search backend was configured but failed,
	// so dynamic recall failed visibly instead of degrading to a local scan.
	RecallBackendSearchFailed RecallBackend = "search_backend_failed"
)

// Recall returns up to limit rows in the named logical database whose contents
// match the free-text query (an empty query matches everything). It is the read
// counterpart to the brain's writers: the agent calls it to check what has
// already been hunted, which intel exists, and whether a detection already
// covers a behavior before opening new work — the move that turns the brain from
// a write-only ledger into the memory each hunt builds on.
func (c *Client) Recall(ctx context.Context, db, query string, limit int) ([]Row, error) {
	rows, _, err := c.RecallWithBackend(ctx, db, query, limit)
	return rows, err
}

// RecallWithBackend is Recall plus an explicit backend label so caller-facing
// tools can say whether a result came from Notion MCP search or the fallback row
// scan instead of forcing the model to infer it.
func (c *Client) RecallWithBackend(ctx context.Context, db, query string, limit int) ([]Row, RecallBackend, error) {
	if limit <= 0 {
		limit = 5
	}
	// A configured search backend (e.g. a Notion MCP server) answers free-text
	// recall with relevance-ranked search rather than the window scan. An empty
	// query still uses Query — it means "list the most recent", which the scan
	// does directly. Search failure is returned rather than silently degrading:
	// dynamic/semantic recall must not pretend a literal row scan is equivalent.
	if s, ok := c.n.(Searcher); ok && s.SearchEnabled() && query != "" {
		rows, err := s.Search(ctx, db, query, limit)
		if err != nil {
			return nil, RecallBackendSearchFailed, err
		}
		return rows, RecallBackendSearch, nil
	}
	rows, err := c.n.Query(ctx, c.dbName(db), query, limit)
	return rows, RecallBackendScan, err
}

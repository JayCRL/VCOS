package memory

import "context"

// Store is the persistent storage interface for memory entries.
// Implementations: MemStore (in-memory, for tests), SQLiteStore (on-disk).
type Store interface {
	// Upsert creates or replaces an entry by ID.
	Upsert(ctx context.Context, entry Entry) error

	// Get retrieves a single entry.
	Get(ctx context.Context, id string) (Entry, error)

	// Delete removes an entry.
	Delete(ctx context.Context, id string) error

	// Query returns entries matching the filter ordered by UpdatedAt desc.
	Query(ctx context.Context, f Filter) ([]Entry, error)

	// QuerySimilar returns entries semantically similar to the given text.
	// If no embedding model is configured it falls back to content-matching.
	QuerySimilar(ctx context.Context, text string, f Filter, k int) ([]Hit, error)

	// Count returns the number of entries matching the filter.
	Count(ctx context.Context, f Filter) (int, error)

	// PurgeExpired removes entries past their ExpireAt.
	PurgeExpired(ctx context.Context) (int, error)
}

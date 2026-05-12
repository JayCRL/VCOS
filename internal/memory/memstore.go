package memory

import (
	"context"
	"sort"
	"strings"
	"sync"
	"time"
)

// MemStore is an in-memory Store implementation for tests and small workloads.
type MemStore struct {
	mu   sync.RWMutex
	data map[string]Entry
}

// NewMemStore returns an empty in-memory store.
func NewMemStore() *MemStore {
	return &MemStore{data: make(map[string]Entry)}
}

func (s *MemStore) Upsert(_ context.Context, entry Entry) error {
	if entry.ID == "" {
		return nil
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now().UTC()
	}
	if entry.UpdatedAt.IsZero() {
		entry.UpdatedAt = time.Now().UTC()
	}
	if entry.TTL > 0 && entry.ExpireAt.IsZero() {
		entry.ExpireAt = time.Now().Add(entry.TTL)
	}
	s.mu.Lock()
	s.data[entry.ID] = entry
	s.mu.Unlock()
	return nil
}

func (s *MemStore) Get(_ context.Context, id string) (Entry, error) {
	s.mu.RLock()
	e, ok := s.data[id]
	s.mu.RUnlock()
	if !ok {
		return Entry{}, nil
	}
	return e, nil
}

func (s *MemStore) Delete(_ context.Context, id string) error {
	s.mu.Lock()
	delete(s.data, id)
	s.mu.Unlock()
	return nil
}

func (s *MemStore) Query(_ context.Context, f Filter) ([]Entry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	results := s.filterLocked(f)
	sort.Slice(results, func(i, j int) bool {
		return results[i].UpdatedAt.After(results[j].UpdatedAt)
	})
	if f.Limit > 0 && len(results) > f.Limit {
		results = results[:f.Limit]
	}
	return results, nil
}

func (s *MemStore) QuerySimilar(_ context.Context, text string, f Filter, k int) ([]Hit, error) {
	entries, err := s.Query(context.Background(), f)
	if err != nil {
		return nil, err
	}
	hits := make([]Hit, 0, len(entries))
	textLower := strings.ToLower(text)
	for _, e := range entries {
		score := simpleTextScore(textLower, e)
		if score <= 0 {
			continue
		}
		hits = append(hits, Hit{Entry: e, Score: score})
	}
	sort.Slice(hits, func(i, j int) bool { return hits[i].Score > hits[j].Score })
	if k > 0 && len(hits) > k {
		hits = hits[:k]
	}
	return hits, nil
}

func (s *MemStore) Count(_ context.Context, f Filter) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.filterLocked(f)), nil
}

func (s *MemStore) PurgeExpired(_ context.Context) (int, error) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()
	count := 0
	for id, e := range s.data {
		if e.Expired(now) {
			delete(s.data, id)
			count++
		}
	}
	return count, nil
}

func (s *MemStore) filterLocked(f Filter) []Entry {
	results := make([]Entry, 0, len(s.data))
	now := time.Now()
	for _, e := range s.data {
		if e.Expired(now) {
			continue
		}
		if !matchAnyString(string(e.Type), f.Types) {
			continue
		}
		if !matchAnyString(string(e.Domain), f.Domains) {
			continue
		}
		if !matchAnyString(string(e.CognitiveKind), f.CognitiveKinds) {
			continue
		}
		if f.SessionID != "" && e.SessionID != f.SessionID {
			continue
		}
		if f.CWD != "" && e.CWD != f.CWD {
			continue
		}
		if f.TitleContains != "" && !strings.Contains(strings.ToLower(e.Title), strings.ToLower(f.TitleContains)) {
			continue
		}
		if f.ContentContains != "" && !strings.Contains(strings.ToLower(e.Content), strings.ToLower(f.ContentContains)) {
			continue
		}
		results = append(results, e)
	}
	return results
}

func matchAnyString[T ~string](v string, xs []T) bool {
	if len(xs) == 0 {
		return true
	}
	for _, x := range xs {
		if string(x) == v {
			return true
		}
	}
	return false
}

func simpleTextScore(query string, e Entry) float64 {
	title := strings.ToLower(e.Title)
	content := strings.ToLower(e.Content)
	if strings.Contains(title, query) || strings.Contains(query, title) {
		return 0.9
	}
	if strings.Contains(content, query) {
		return 0.7
	}
	overlap := wordOverlap(query, content)
	if overlap > 0.5 {
		return 0.5
	}
	// Partial word match
	for _, qw := range strings.Fields(query) {
		if strings.Contains(content, qw) {
			return 0.4
		}
	}
	if overlap > 0.2 {
		return 0.3
	}
	return 0
}

func wordOverlap(a, b string) float64 {
	wa := strings.Fields(a)
	wb := strings.Fields(b)
	if len(wa) == 0 || len(wb) == 0 {
		return 0
	}
	set := make(map[string]struct{}, len(wb))
	for _, w := range wb {
		set[w] = struct{}{}
	}
	matched := 0
	for _, w := range wa {
		if _, ok := set[w]; ok {
			matched++
		}
	}
	return float64(matched) / float64(len(wa))
}

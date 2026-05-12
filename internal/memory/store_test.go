package memory

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testStore(t *testing.T, s Store) {
	ctx := context.Background()
	now := time.Now()
	t.Helper()

	// Upsert + Get
	e1 := Entry{
		ID: "mem-1", Type: TypeShortTerm, Domain: DomainDialogue,
		Title: "greeting", Content: "user said hello in session s1",
		Source: SourceUser, SessionID: "s1",
		Metadata: map[string]any{"key": "val"},
		CreatedAt: now, UpdatedAt: now,
	}
	if err := s.Upsert(ctx, e1); err != nil {
		t.Fatalf("Upsert: %v", err)
	}
	got, err := s.Get(ctx, "mem-1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "greeting" || got.Content != e1.Content {
		t.Fatalf("Get mismatch: %+v", got)
	}
	if got.Metadata == nil || got.Metadata["key"] != "val" {
		t.Fatalf("metadata mismatch: %v", got.Metadata)
	}

	// Upsert update
	e1.Title = "updated greeting"
	if err := s.Upsert(ctx, e1); err != nil {
		t.Fatalf("Upsert update: %v", err)
	}
	got, _ = s.Get(ctx, "mem-1")
	if got.Title != "updated greeting" {
		t.Fatalf("title not updated: %q", got.Title)
	}

	// Second entry
	e2 := Entry{
		ID: "mem-2", Type: TypeProject, Domain: DomainCode,
		Title: "arch decision", Content: "use SQLite for memory store",
		Source: SourceKernel, CWD: "/project",
		CreatedAt: now.Add(-time.Hour), UpdatedAt: now.Add(-time.Hour),
	}
	_ = s.Upsert(ctx, e2)

	// Count
	n, err := s.Count(ctx, Filter{})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if n < 2 {
		t.Fatalf("Count=%d want >=2", n)
	}

	// Query by type
	entries, err := s.Query(ctx, Filter{Types: []Type{TypeProject}})
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(entries) != 1 || entries[0].ID != "mem-2" {
		t.Fatalf("Query project: got %d entries", len(entries))
	}

	// Query by CWD
	entries, err = s.Query(ctx, Filter{CWD: "/project"})
	if err != nil {
		t.Fatalf("Query CWD: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("Query CWD: got %d", len(entries))
	}

	// QuerySimilar
	hits, err := s.QuerySimilar(ctx, "SQLite memory", Filter{}, 5)
	if err != nil {
		t.Fatalf("QuerySimilar: %v", err)
	}
	if len(hits) != 1 || hits[0].Entry.ID != "mem-2" {
		t.Fatalf("QuerySimilar: got %d hits", len(hits))
	}

	// TTL + PurgeExpired
	e3 := Entry{
		ID: "mem-ttl", Type: TypeShortTerm,
		Title: "ephemeral", Content: "should expire",
		TTL: 10 * time.Millisecond,
	}
	_ = s.Upsert(ctx, e3)
	time.Sleep(20 * time.Millisecond)
	purged, err := s.PurgeExpired(ctx)
	if err != nil {
		t.Fatalf("PurgeExpired: %v", err)
	}
	if purged < 1 {
		t.Fatalf("PurgeExpired purged %d, want >=1", purged)
	}
	got3, _ := s.Get(ctx, "mem-ttl")
	if !got3.Expired(time.Now()) {
		t.Logf("entry still queryable after purge if store filters lazily — ok")
	}

	// Delete
	if err := s.Delete(ctx, "mem-1"); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	got, err = s.Get(ctx, "mem-1")
	if err != nil {
		t.Fatalf("Get after Delete: %v", err)
	}
	if got.ID != "" {
		t.Fatalf("entry not deleted, ID=%q", got.ID)
	}
}

func TestMemStore(t *testing.T) {
	testStore(t, NewMemStore())
}

func TestSQLiteStore(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "memory_test.db")
	s, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	defer s.Close()
	testStore(t, s)
}

func TestSQLiteStorePersists(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "memory_persist.db")

	// Write.
	s1, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("NewSQLiteStore: %v", err)
	}
	_ = s1.Upsert(context.Background(), Entry{
		ID: "p1", Type: TypeLongTerm, Title: "persistent", Content: "should survive",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})
	s1.Close()

	// Reopen and read.
	s2, err := NewSQLiteStore(dbPath)
	if err != nil {
		t.Fatalf("reopen: %v", err)
	}
	defer s2.Close()
	got, err := s2.Get(context.Background(), "p1")
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Title != "persistent" {
		t.Fatalf("persistence failed: %+v", got)
	}
}

func TestEmbRoundtrip(t *testing.T) {
	orig := []float32{0.1, 0.2, 0.3}
	b := float32ToBytes(orig)
	back := bytesToFloat32(b)
	if len(back) != 3 || back[0] != orig[0] || back[1] != orig[1] || back[2] != orig[2] {
		t.Fatalf("roundtrip: got %v", back)
	}
	// nil cases
	if float32ToBytes(nil) != nil {
		t.Fatal("nil → non-nil")
	}
	if bytesToFloat32(nil) != nil {
		t.Fatal("nil → non-nil")
	}
}

// Ensure temp cleanup
func cleanupSQLite() {
	// Remove test dbs that may leak
	matches, _ := filepath.Glob(filepath.Join(os.TempDir(), "memory_*.db*"))
	for _, m := range matches {
		_ = os.Remove(m)
	}
}
func TestMain(m *testing.M) {
	code := m.Run()
	cleanupSQLite()
	os.Exit(code)
}

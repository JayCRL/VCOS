package feedback

import (
	"context"
	"testing"

	"mobilevc/memory"
)

func TestProposeAndDecideAccept(t *testing.T) {
	store := memory.NewMemStore()
	c := New(store)

	s := c.Propose("add-caching", "evolve", "sess-f1", []string{"cache miss pattern", "use sync.Pool"}, 0.9)
	if s.ID == "" {
		t.Fatal("empty suggestion ID")
	}

	rec, err := c.Decide(context.Background(), s.ID, DecisionAccept, "")
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}
	if rec.Decision != DecisionAccept {
		t.Fatalf("Decision=%s", rec.Decision)
	}

	// Verify memory was persisted.
	entries, _ := store.Query(context.Background(), memory.Filter{Types: []memory.Type{memory.TypeLongTerm}})
	if len(entries) != 2 {
		t.Fatalf("expected 2 memory entries (one per learning), got %d", len(entries))
	}
	for _, e := range entries {
		if e.Metadata["status"] != "accepted" {
			t.Fatalf("status=%v", e.Metadata["status"])
		}
	}
}

func TestDecideReject(t *testing.T) {
	store := memory.NewMemStore()
	c := New(store)

	s := c.Propose("bad-pattern", "evolve", "sess-f2", []string{"use global mutable state"}, 0.7)
	_, err := c.Decide(context.Background(), s.ID, DecisionReject, "")
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}

	entries, _ := store.Query(context.Background(), memory.Filter{Types: []memory.Type{memory.TypeLongTerm}})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Metadata["status"] != "rejected" {
		t.Fatalf("status=%v", entries[0].Metadata["status"])
	}
	if entries[0].Metadata["confidence"] != 0.1 {
		t.Fatalf("confidence=%v", entries[0].Metadata["confidence"])
	}
}

func TestDecideAdjust(t *testing.T) {
	store := memory.NewMemStore()
	c := New(store)

	s := c.Propose("fix-pattern", "evolve", "sess-f3", []string{"nil pointer in handler"}, 0.7)
	_, err := c.Decide(context.Background(), s.ID, DecisionAdjust, "check nil before deref in all handlers")
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}

	entries, _ := store.Query(context.Background(), memory.Filter{Types: []memory.Type{memory.TypeLongTerm}})
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Title != "check nil before deref in all handlers" {
		t.Fatalf("title=%q", entries[0].Title)
	}
	if entries[0].Metadata["status"] != "adjusted" {
		t.Fatalf("status=%v", entries[0].Metadata["status"])
	}
}

func TestPendingAndStats(t *testing.T) {
	store := memory.NewMemStore()
	c := New(store)

	s1 := c.Propose("sug-1", "evolve", "s", []string{"p1"}, 0.9)
	s2 := c.Propose("sug-2", "intake", "s", []string{"p2"}, 0.8)

	pending := c.Pending()
	if len(pending) != 2 {
		t.Fatalf("Pending=%d", len(pending))
	}

	ctx := context.Background()
	_, _ = c.Decide(ctx, s1.ID, DecisionAccept, "")
	_, _ = c.Decide(ctx, s2.ID, DecisionReject, "")

	stats := c.Stats()
	if stats.Total != 2 || stats.Accepted != 1 || stats.Rejected != 1 || stats.Pending != 0 {
		t.Fatalf("Stats=%+v", stats)
	}
}

func TestProposeFromEvolveResult(t *testing.T) {
	c := New(memory.NewMemStore())

	suggestions := c.ProposeFromEvolveResult("refactor-auth", "sess-ev", true, []string{"OAuth flow correct", "JWT expiry set"})
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion, got %d", len(suggestions))
	}
	if suggestions[0].Confidence != 0.9 {
		t.Fatalf("confidence=%f want 0.9", suggestions[0].Confidence)
	}

	// Failed result → lower confidence.
	suggestions = c.ProposeFromEvolveResult("broken-build", "sess-ev", false, []string{"undefined variable"})
	if suggestions[0].Confidence != 0.6 {
		t.Fatalf("confidence=%f want 0.6", suggestions[0].Confidence)
	}
}

func TestHistory(t *testing.T) {
	c := New(memory.NewMemStore())
	s := c.Propose("s", "e", "sess", []string{"p"}, 0.9)

	_, _ = c.Decide(context.Background(), s.ID, DecisionAccept, "")
	_, _ = c.Decide(context.Background(), s.ID, DecisionReject, "") // second decide on same ID fails

	history := c.History()
	if len(history) != 1 {
		t.Fatalf("History=%d", len(history))
	}
}

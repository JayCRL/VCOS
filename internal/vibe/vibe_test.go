package vibe

import (
	"context"
	"testing"
	"time"

	"mobilevc/internal/memory"
)

func TestGetDefaultState(t *testing.T) {
	c := New(memory.NewMemStore())
	s := c.Get(context.Background(), "no-such-session")
	if s.Style != StyleBalanced || s.Proactivity != ProactivityBalanced || s.Role != RoleDeveloper {
		t.Fatalf("defaults wrong: %+v", s)
	}
}

func TestSetAndGet(t *testing.T) {
	c := New(memory.NewMemStore())
	ctx := context.Background()

	err := c.Set(ctx, "sess-v", State{Style: StyleConcise, Proactivity: ProactivityActive, Role: RoleReviewer})
	if err != nil {
		t.Fatalf("Set: %v", err)
	}

	s := c.Get(ctx, "sess-v")
	if s.Style != StyleConcise || s.Role != RoleReviewer {
		t.Fatalf("got %+v", s)
	}
	if s.UpdatedAt.IsZero() {
		t.Fatal("UpdatedAt not set")
	}
}

func TestUpdateConvenience(t *testing.T) {
	c := New(memory.NewMemStore())
	ctx := context.Background()

	_ = c.Set(ctx, "sess-u", defaultState)
	_ = c.UpdateStyle(ctx, "sess-u", StyleVerbose)
	_ = c.UpdateRole(ctx, "sess-u", RoleArchitect)

	s := c.Get(ctx, "sess-u")
	if s.Style != StyleVerbose || s.Role != RoleArchitect {
		t.Fatalf("got %+v", s)
	}
}

func TestTags(t *testing.T) {
	c := New(memory.NewMemStore())
	ctx := context.Background()
	_ = c.Set(ctx, "sess-t", State{Style: StyleConcise, Proactivity: ProactivityPassive, Role: RoleDebugger})

	tags := c.Tags(ctx, "sess-t")
	found := make(map[string]bool)
	for _, tag := range tags {
		found[tag] = true
	}
	if !found["vibe:style=concise"] || !found["vibe:role=debugger"] {
		t.Fatalf("tags: %v", tags)
	}
}

func TestCacheInvalidation(t *testing.T) {
	c := New(memory.NewMemStore())
	ctx := context.Background()

	_ = c.Set(ctx, "sess-c", State{Style: StyleConcise})
	s1 := c.Get(ctx, "sess-c")
	if s1.Style != StyleConcise {
		t.Fatal("first get failed")
	}

	// Update via Set should refresh cache.
	_ = c.Set(ctx, "sess-c", State{Style: StyleVerbose})
	s2 := c.Get(ctx, "sess-c")
	if s2.Style != StyleVerbose {
		t.Fatalf("cache not refreshed: %+v", s2)
	}
}

func TestPersistenceAcrossController(t *testing.T) {
	store := memory.NewMemStore()
	c1 := New(store)
	ctx := context.Background()

	_ = c1.Set(ctx, "sess-p", State{Style: StyleVerbose, Role: RoleArchitect})

	// New controller with same backing store should load persisted state.
	c2 := New(store)
	s := c2.Get(ctx, "sess-p")
	if s.Style != StyleVerbose || s.Role != RoleArchitect {
		t.Fatalf("persistence failed: %+v", s)
	}
}

func TestParseFormatRoundtrip(t *testing.T) {
	orig := State{Style: StyleConcise, Proactivity: ProactivityActive, Role: RoleReviewer, UpdatedAt: time.Now()}
	content := formatVibeContent(orig)
	parsed := parseVibeContent(content)
	if parsed.Style != orig.Style || parsed.Proactivity != orig.Proactivity || parsed.Role != orig.Role {
		t.Fatalf("roundtrip: %+v → %q → %+v", orig, content, parsed)
	}
}

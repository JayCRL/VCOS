package moe

import (
	"context"
	"testing"
	"time"

	"mobilevc/internal/memory"
	"mobilevc/internal/kernel/scheduler"
)

func TestClassify(t *testing.T) {
	r := NewRouter(memory.NewMemStore())

	tests := []struct {
		name   string
		req    scheduler.IntentRequest
		wantT  memory.Type
		wantD  memory.Domain
	}{
		{
			name: "exec code", req: scheduler.IntentRequest{Kind: scheduler.KindExec, Command: "fix bug", CWD: "/proj"},
			wantT: memory.TypeProject, wantD: memory.DomainCode,
		},
		{
			name: "ai_turn dialogue", req: scheduler.IntentRequest{Kind: scheduler.KindAITurn, Command: "explain this"},
			wantT: memory.TypeShortTerm, wantD: memory.DomainDialogue,
		},
		{
			name: "slash skill", req: scheduler.IntentRequest{Kind: scheduler.KindSlash, Command: "review", Tags: []string{"code"}},
			wantT: memory.TypeProject, wantD: memory.DomainCode,
		},
		{
			name: "task planning", req: scheduler.IntentRequest{Kind: scheduler.KindExec, Command: "plan deployment", CWD: "/proj"},
			wantT: memory.TypeProject, wantD: memory.DomainTask,
		},
	}

	for _, tt := range tests {
		gotT, gotD := r.Classify(tt.req)
		if gotT != tt.wantT {
			t.Errorf("%s: type=%q want %q", tt.name, gotT, tt.wantT)
		}
		if gotD != tt.wantD {
			t.Errorf("%s: domain=%q want %q", tt.name, gotD, tt.wantD)
		}
	}
}

func TestRetrieve(t *testing.T) {
	store := memory.NewMemStore()
	ctx := context.Background()
	now := time.Now()

	_ = store.Upsert(ctx, memory.Entry{
		ID: "m1", Type: memory.TypeProject, Domain: memory.DomainCode,
		Title: "auth refactor", Content: "refactor OAuth flow to use JWT",
		CWD: "/proj", SessionID: "s1",
		CreatedAt: now, UpdatedAt: now,
	})
	_ = store.Upsert(ctx, memory.Entry{
		ID: "m2", Type: memory.TypeShortTerm, Domain: memory.DomainDialogue,
		Title: "greeting", Content: "hello world",
		SessionID: "s1",
		CreatedAt: now, UpdatedAt: now,
	})

	r := NewRouter(store)
	hits, err := r.Retrieve(ctx, scheduler.IntentRequest{
		Kind: scheduler.KindExec, Command: "refactor auth",
		CWD: "/proj", SessionID: "s1",
	}, 3)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(hits) != 1 || hits[0].Entry.ID != "m1" {
		t.Fatalf("expected m1, got %d hits", len(hits))
	}
}

func TestWriteMemory(t *testing.T) {
	store := memory.NewMemStore()
	r := NewRouter(store)

	err := r.WriteMemory(context.Background(), scheduler.IntentRequest{
		Kind: scheduler.KindExec, Owner: "exec-1", SessionID: "s1",
		Command: "fix login", CWD: "/proj",
	}, "user asked to fix login redirect")
	if err != nil {
		t.Fatalf("WriteMemory: %v", err)
	}
	entry, _ := store.Get(context.Background(), "mem-exec-1-exec")
	if entry.Title != "fix login" {
		t.Fatalf("entry: %+v", entry)
	}
}

func TestSchedulerRuleEnrichesTags(t *testing.T) {
	store := memory.NewMemStore()
	ctx := context.Background()
	now := time.Now()

	_ = store.Upsert(ctx, memory.Entry{
		ID: "m-code", Type: memory.TypeProject, Domain: memory.DomainCode,
		Title: "code standard", Content: "use Go 1.25 generics",
		CWD: "/proj", SessionID: "s1",
		CreatedAt: now, UpdatedAt: now,
	})

	r := NewRouter(store)
	rule := r.SchedulerRule(2)

	req := scheduler.IntentRequest{
		Kind: scheduler.KindExec, Command: "refactor generics", CWD: "/proj", SessionID: "s1",
		Tags: []string{},
	}
	d := rule(&req)
	if d.Outcome != scheduler.OutcomeAdmit {
		t.Fatalf("SchedulerRule denied: %s", d.Outcome)
	}
	hasTag := false
	for _, tag := range req.Tags {
		if tag == "moe:hit=m-code" {
			hasTag = true
		}
	}
	if !hasTag {
		t.Fatalf("tags not enriched: %v", req.Tags)
	}
}

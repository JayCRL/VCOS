package kernel

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"mobilevc/data"
	"mobilevc/kernel/eventbus"
	"mobilevc/evolution/evolve"
	"mobilevc/evolution/feedback"
	"mobilevc/kernel/scheduler"
	"mobilevc/kernel/watchdog"
	"mobilevc/memory"
	"mobilevc/protocol"
	"mobilevc/evolution/shadow"
	"mobilevc/cognition/vibe"
	"os"
	"path/filepath"
)

// ——— P3 integration: Intake → Vibe → Scheduler → Dashboard ———

func TestP3IntakeViaKernel(t *testing.T) {
	k := New(newMemStore())
	defer k.Stop()

	// Simulate CreateSession triggering intake (CreateSession calls Intake.Run internally).
	profile, err := k.Intake.Run(context.Background(), "sess-intake", "/tmp/test-project", "review the auth module")
	if err != nil {
		t.Fatalf("Intake.Run: %v", err)
	}
	if profile.Role != "reviewer" {
		t.Fatalf("role=%q", profile.Role)
	}

	// Intake should have seeded memories.
	entries, _ := k.MemStore.Query(context.Background(), memory.Filter{SessionID: "sess-intake"})
	if len(entries) < 2 {
		t.Fatalf("expected >=2 intake entries, got %d", len(entries))
	}
}

func TestP3VibeInScheduler(t *testing.T) {
	k := New(newMemStore())
	defer k.Stop()

	// Set a custom vibe.
	err := k.Vibe.Set(context.Background(), "sess-vibe", vibe.State{Style: vibe.StyleConcise, Proactivity: vibe.ProactivityActive, Role: vibe.RoleDebugger})
	if err != nil {
		t.Fatalf("Vibe.Set: %v", err)
	}

	// Scheduler decides — Vibe rule should attach tags.
	d := k.Scheduler.Decide(scheduler.IntentRequest{
		Kind: scheduler.KindExec, SessionID: "sess-vibe", Owner: "v-1",
		Command: "fix the bug",
	})
	if d.Outcome != scheduler.OutcomeAdmit {
		t.Fatalf("Outcome=%s", d.Outcome)
	}
	// Verify lock was acquired (vibe rule didn't block).
	if owner, _ := k.LockMgr.Holder("sess-vibe"); owner != "v-1" {
		t.Fatalf("lock owner=%q", owner)
	}
	k.Scheduler.Release("sess-vibe", "v-1")
}

func TestP3ActiveSessions(t *testing.T) {
	k := New(newMemStore())
	defer k.Stop()

	// Initially empty.
	if ids := k.ActiveSessions(); len(ids) != 0 {
		t.Fatalf("expected 0 sessions, got %v", ids)
	}

	// Simulate attaching a session.
	conn := &ConnectionState{ConnectionID: "conn-1", SelectedSessionID: "sess-dash"}
	k.SwitchSession(conn, "sess-dash", func(any) {})

	ids := k.ActiveSessions()
	if len(ids) != 1 || ids[0] != "sess-dash" {
		t.Fatalf("ActiveSessions=%v", ids)
	}
}

func TestP3FullCognitiveChain(t *testing.T) {
	k := New(newMemStore())
	defer k.Stop()

	// Step 1: Intake — analyze project and seed memory.
	_, err := k.Intake.Run(context.Background(), "sess-full", "/tmp/app", "design a new caching layer")
	if err != nil {
		t.Fatalf("Intake.Run: %v", err)
	}

	// Step 2: Vibe — set user preference.
	_ = k.Vibe.Set(context.Background(), "sess-full", vibe.State{Style: "verbose", Proactivity: "balanced", Role: "architect"})

	// Step 3: Scheduler with MOE+Vibe — should admit and lock.
	d := k.Scheduler.Decide(scheduler.IntentRequest{
		Kind: scheduler.KindExec, SessionID: "sess-full", Owner: "exec-full",
		Command: "implement cache", CWD: "/tmp/app",
	})
	if d.Outcome != scheduler.OutcomeAdmit {
		t.Fatalf("scheduler denied: %s — %s", d.Outcome, d.Reason)
	}

	// Step 4: Watchdog + release.
	k.Watchdog.Watch(watchdog.WatchOptions{
		ExecutionID: "exec-full", SessionID: "sess-full",
		LockKey: d.LockKey, Timeout: 30 * time.Second,
	})
	k.Watchdog.Settle("exec-full")
	k.Scheduler.Release(d.LockKey, "exec-full")

	// Verify all memories persisted.
	entries, _ := k.MemStore.Query(context.Background(), memory.Filter{SessionID: "sess-full"})
	if len(entries) < 3 {
		t.Fatalf("expected >=3 memories (intake + vibe), got %d", len(entries))
	}
}

// ——— P4 integration: Shadow → Evolve → Memory writeback ———

func TestP4ShadowWorkspaceViaKernel(t *testing.T) {
	k := New(newMemStore())
	defer k.Stop()

	// Create a temp Go project.
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.25\n"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)

	ws, err := k.ShadowMgr.CreateWorkspace(context.Background(), dir)
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	defer ws.Cleanup()

	// Verify the workspace contains the file.
	_, err = os.Stat(filepath.Join(ws.Path, "main.go"))
	if err != nil {
		t.Fatalf("shadow workspace missing file: %v", err)
	}
}

func TestP4EvolveViaKernel(t *testing.T) {
	k := New(newMemStore())
	defer k.Stop()

	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.25\n"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)

	result, err := k.Evolver.Evaluate(context.Background(), evolve.Proposal{
		Title:  "kernel-evolve",
		CWD:    dir,
		Checks: []evolve.CheckType{evolve.CheckBuild},
	})
	if err != nil {
		t.Fatalf("Evolve: %v", err)
	}
	if !result.Passed {
		t.Fatalf("build should pass: %s", result.Summary)
	}

	// Learnings persisted to memory.
	entries, _ := k.MemStore.Query(context.Background(), memory.Filter{Types: []memory.Type{memory.TypeLongTerm}})
	if len(entries) == 0 {
		t.Fatal("no learnings persisted")
	}
}

func TestP4EvolveWithDiffViaKernel(t *testing.T) {
	k := New(newMemStore())
	defer k.Stop()

	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.25\n"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n"), 0644) // invalid

	result, err := k.Evolver.Evaluate(context.Background(), evolve.Proposal{
		Title: "fix-main",
		CWD:   dir,
		Changes: []shadow.FileChange{
			{Path: "main.go", NewContent: "package main\n\nfunc main() {}\n"},
		},
		Checks: []evolve.CheckType{evolve.CheckBuild},
	})
	if err != nil {
		t.Fatalf("Evolve: %v", err)
	}
	if !result.Passed {
		t.Fatalf("diff should fix the build: %s", result.Summary)
	}
}

func TestP4FullEvolutionChain(t *testing.T) {
	k := New(newMemStore())
	defer k.Stop()

	// Step 1: Intake.
	_, _ = k.Intake.Run(context.Background(), "sess-evo", "/tmp/app", "add login")

	// Step 2: Vibe.
	_ = k.Vibe.Set(context.Background(), "sess-evo", vibe.State{Style: vibe.StyleBalanced, Role: vibe.RoleDeveloper})

	// Step 3: Scheduler — MOE + Vibe enrich, lock acquired.
	d := k.Scheduler.Decide(scheduler.IntentRequest{
		Kind: scheduler.KindExec, SessionID: "sess-evo", Owner: "evo-1",
		Command: "implement feature", CWD: "/tmp/app",
	})
	if d.Outcome != scheduler.OutcomeAdmit {
		t.Fatalf("scheduler: %s", d.Outcome)
	}

	// Step 4: Shadow + Evolve — evaluate a proposed change.
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.25\n"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)

	result, err := k.Evolver.Evaluate(context.Background(), evolve.Proposal{
		Title:  "final-check",
		CWD:    dir,
		Checks: []evolve.CheckType{evolve.CheckBuild},
	})
	if err != nil {
		t.Fatalf("Evolve: %v", err)
	}
	if !result.Passed {
		t.Fatalf("evolution failed: %s", result.Summary)
	}

	// Step 5: Watchdog + release.
	k.Watchdog.Watch(watchdog.WatchOptions{
		ExecutionID: "evo-1", SessionID: "sess-evo",
		LockKey: d.LockKey, Timeout: 30 * time.Second,
	})
	k.Watchdog.Settle("evo-1")
	k.Scheduler.Release(d.LockKey, "evo-1")
}

// ——— P5 integration: Feedback closed loop ———

func TestP5FeedbackProposeAcceptCycle(t *testing.T) {
	k := New(newMemStore())
	defer k.Stop()

	// Propose suggestions from an evolution result.
	suggestions := k.Feedback.ProposeFromEvolveResult("add-rate-limiting", "sess-fb",
		true, []string{"use token bucket", "per-user quota"})
	if len(suggestions) != 1 {
		t.Fatalf("expected 1 suggestion")
	}

	// User accepts.
	_, err := k.Feedback.Decide(context.Background(), suggestions[0].ID, feedback.DecisionAccept, "")
	if err != nil {
		t.Fatalf("Decide: %v", err)
	}

	// Verify accepted learnings are in memory.
	entries, _ := k.MemStore.Query(context.Background(), memory.Filter{
		Types: []memory.Type{memory.TypeLongTerm},
	})
	if len(entries) != 2 {
		t.Fatalf("expected 2 accepted entries, got %d", len(entries))
	}
	for _, e := range entries {
		if e.Metadata["status"] != "accepted" {
			t.Fatalf("entry %s not accepted: %v", e.ID, e.Metadata)
		}
	}

	// Stats.
	s := k.Feedback.Stats()
	if s.Accepted != 1 || s.Total != 1 {
		t.Fatalf("Stats=%+v", s)
	}
}

func TestP5FeedbackRejectRemovesConfidence(t *testing.T) {
	k := New(newMemStore())
	defer k.Stop()

	s := k.Feedback.Propose("bad-idea", "evolve", "sess-fr", []string{"global mutable state"}, 0.7)
	_, _ = k.Feedback.Decide(context.Background(), s.ID, feedback.DecisionReject, "")

	entries, _ := k.MemStore.Query(context.Background(), memory.Filter{Types: []memory.Type{memory.TypeLongTerm}})
	if len(entries) != 1 {
		t.Fatalf("expected 1 rejected entry, got %d", len(entries))
	}
	if entries[0].Metadata["confidence"] != 0.1 {
		t.Fatalf("confidence=%v want 0.1", entries[0].Metadata["confidence"])
	}
}

func TestP5FeedbackAdjust(t *testing.T) {
	k := New(newMemStore())
	defer k.Stop()

	s := k.Feedback.Propose("fix-nil", "evolve", "sess-fa", []string{"check nil in handler"}, 0.7)
	_, _ = k.Feedback.Decide(context.Background(), s.ID, feedback.DecisionAdjust,
		"check nil pointer before dereference in all HTTP handlers")

	entries, _ := k.MemStore.Query(context.Background(), memory.Filter{Types: []memory.Type{memory.TypeLongTerm}})
	if entries[0].Title != "check nil pointer before dereference in all HTTP handlers" {
		t.Fatalf("adjusted title=%q", entries[0].Title)
	}
	if entries[0].Metadata["confidence"] != 0.8 {
		t.Fatalf("confidence=%v", entries[0].Metadata["confidence"])
	}
}

func TestP5FullClosedLoop(t *testing.T) {
	k := New(newMemStore())
	defer k.Stop()

	// Step 1: Intake — analyze project.
	_, _ = k.Intake.Run(context.Background(), "sess-loop", "/tmp/app", "add caching layer")

	// Step 2: Vibe — set preferences.
	_ = k.Vibe.Set(context.Background(), "sess-loop", vibe.State{Style: vibe.StyleConcise, Role: vibe.RoleDeveloper})

	// Step 3: Scheduler — MOE + Vibe enriched decision.
	d := k.Scheduler.Decide(scheduler.IntentRequest{
		Kind: scheduler.KindExec, SessionID: "sess-loop", Owner: "loop-1",
		Command: "implement cache", CWD: "/tmp/app",
	})
	if d.Outcome != scheduler.OutcomeAdmit {
		t.Fatalf("scheduler: %s", d.Outcome)
	}

	// Step 4: Evolve — verify a code change.
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.25\n"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {}\n"), 0644)

	result, err := k.Evolver.Evaluate(context.Background(), evolve.Proposal{
		Title:  "add-cache",
		CWD:    dir,
		Checks: []evolve.CheckType{evolve.CheckBuild},
	})
	if err != nil {
		t.Fatalf("Evolve: %v", err)
	}

	// Step 5: Feedback — propose learnings and user accepts.
	learnings := make([]string, len(result.Learnings))
	for i, l := range result.Learnings {
		learnings[i] = l.Pattern
	}
	suggestions := k.Feedback.ProposeFromEvolveResult(result.Title, "sess-loop", result.Passed, learnings)
	for _, sug := range suggestions {
		_, _ = k.Feedback.Decide(context.Background(), sug.ID, feedback.DecisionAccept, "")
	}

	// Step 6: Verify feedback persisted.
	entries, _ := k.MemStore.Query(context.Background(), memory.Filter{Types: []memory.Type{memory.TypeLongTerm}})
	if len(entries) == 0 {
		t.Fatal("no learnings persisted after feedback")
	}

	// Step 7: Watchdog + release.
	k.Watchdog.Watch(watchdog.WatchOptions{
		ExecutionID: "loop-1", SessionID: "sess-loop",
		LockKey: d.LockKey, Timeout: 30 * time.Second,
	})
	k.Watchdog.Settle("loop-1")
	k.Scheduler.Release(d.LockKey, "loop-1")
}

// memStore is a no-op implementation of data.Store sufficient for kernel.New.
type memStore struct{ data.Store }

func newMemStore() data.Store { return memStore{} }

// ——— P1 integration ———

func TestP1OrchestrationHappyPath(t *testing.T) {
	store := newMemStore()
	k := New(store)
	defer k.Stop()

	d := k.Scheduler.Decide(scheduler.IntentRequest{
		Kind:      scheduler.KindExec,
		SessionID: "sess-A",
		Owner:     "exec-1",
		Command:   "claude",
	})
	if d.Outcome != scheduler.OutcomeAdmit {
		t.Fatalf("Outcome=%s want admit", d.Outcome)
	}

	k.Watchdog.Watch(watchdog.WatchOptions{
		ExecutionID: "exec-1",
		SessionID:   "sess-A",
		LockKey:     d.LockKey,
		Timeout:     30 * time.Second,
	})

	for i := 0; i < 3; i++ {
		k.Bus.Publish(eventbus.Envelope{
			Source:    eventbus.SourceSession,
			SessionID: "sess-A",
			Topic:     protocol.EventTypeLog,
			Payload: protocol.LogEvent{
				Event: protocol.Event{
					Type:        protocol.EventTypeLog,
					SessionID:   "sess-A",
					RuntimeMeta: protocol.RuntimeMeta{ExecutionID: "exec-1"},
				},
				Message: "tick",
			},
		})
	}

	k.Watchdog.Settle("exec-1")
	k.Scheduler.Release(d.LockKey, "exec-1")

	if owner, _ := k.LockMgr.Holder("sess-A"); owner != "" {
		t.Fatalf("lock not released, owner=%q", owner)
	}
	if k.Watchdog.Pending() != 0 {
		t.Fatalf("Watchdog.Pending=%d", k.Watchdog.Pending())
	}
}

func TestP1OrchestrationTimeoutPath(t *testing.T) {
	k := New(newMemStore())
	defer k.Stop()
	k.Watchdog = watchdog.NewWithConfig(k.Bus, k.LockMgr, watchdog.Config{MinIdleTimeout: 5 * time.Millisecond})

	d := k.Scheduler.Decide(scheduler.IntentRequest{
		Kind:      scheduler.KindExec,
		SessionID: "sess-T",
		Owner:     "exec-9",
	})
	if d.Outcome != scheduler.OutcomeAdmit {
		t.Fatalf("expected admit got %s", d.Outcome)
	}

	var fired atomic.Int64
	sub := k.Bus.Subscribe("tap", eventbus.Filter{Sources: []eventbus.Source{eventbus.SourceKernel}}, func(env eventbus.Envelope) {
		if e, ok := env.Payload.(protocol.ErrorEvent); ok && e.Code == "watchdog_timeout" {
			fired.Add(1)
		}
	})
	defer sub.Close()

	k.Watchdog.Watch(watchdog.WatchOptions{
		ExecutionID: "exec-9",
		SessionID:   "sess-T",
		LockKey:     d.LockKey,
		Timeout:     30 * time.Millisecond,
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if fired.Load() > 0 {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if fired.Load() == 0 {
		t.Fatalf("watchdog did not fire timeout")
	}
	if owner, _ := k.LockMgr.Holder("sess-T"); owner != "" {
		t.Fatalf("lock not force-released, owner=%q", owner)
	}
}

func TestP1OrchestrationConflictDefers(t *testing.T) {
	k := New(newMemStore())
	defer k.Stop()

	d1 := k.Scheduler.Decide(scheduler.IntentRequest{Kind: scheduler.KindExec, SessionID: "sess-C", Owner: "ownerA"})
	if d1.Outcome != scheduler.OutcomeAdmit {
		t.Fatalf("d1 outcome=%s", d1.Outcome)
	}

	d2 := k.Scheduler.Decide(scheduler.IntentRequest{Kind: scheduler.KindExec, SessionID: "sess-C", Owner: "ownerB"})
	if d2.Outcome != scheduler.OutcomeDefer {
		t.Fatalf("d2 outcome=%s want defer", d2.Outcome)
	}
	if d2.Conflict != "ownerA" {
		t.Fatalf("Conflict=%q", d2.Conflict)
	}
}

// ——— P2 integration: MOE → Scheduler → LockMgr → Watchdog ———

func TestP2MoeEnrichesIntentBeforeLock(t *testing.T) {
	k := New(newMemStore())
	defer k.Stop()

	_ = k.MemStore.Upsert(context.Background(), memory.Entry{
		ID: "mem-code-1", Type: memory.TypeProject, Domain: memory.DomainCode,
		Title: "auth refactor", Content: "refactor auth to use JWT",
		CWD: "/proj", SessionID: "sess-M",
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	})

	d := k.Scheduler.Decide(scheduler.IntentRequest{
		Kind: scheduler.KindExec, SessionID: "sess-M", Owner: "exec-M",
		Command: "refactor", CWD: "/proj",
	})
	if d.Outcome != scheduler.OutcomeAdmit {
		t.Fatalf("Outcome=%s want admit", d.Outcome)
	}
	if owner, _ := k.LockMgr.Holder("sess-M"); owner != "exec-M" {
		t.Fatalf("lock owner=%q want exec-M", owner)
	}
	k.Scheduler.Release("sess-M", "exec-M")
}

func TestP2MoeWriteAndRetrieveCycle(t *testing.T) {
	k := New(newMemStore())
	defer k.Stop()

	err := k.MoeRouter.WriteMemory(context.Background(), scheduler.IntentRequest{
		Kind: scheduler.KindExec, Owner: "exec-W", SessionID: "sess-W",
		Command: "fix login redirect", CWD: "/proj/src",
	}, "the login page had an infinite redirect loop in the OAuth callback")
	if err != nil {
		t.Fatalf("WriteMemory: %v", err)
	}

	hits, err := k.MoeRouter.Retrieve(context.Background(), scheduler.IntentRequest{
		Kind: scheduler.KindExec, SessionID: "sess-W",
		Command: "login bug fix", CWD: "/proj/src",
	}, 5)
	if err != nil {
		t.Fatalf("Retrieve: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("expected 1 hit, got %d", len(hits))
	}
	if hits[0].Entry.Title != "fix login redirect" {
		t.Fatalf("wrong entry: %+v", hits[0].Entry)
	}
}

func TestP2FullOrchestrationChain(t *testing.T) {
	k := New(newMemStore())
	defer k.Stop()

	_ = k.MoeRouter.WriteMemory(context.Background(), scheduler.IntentRequest{
		Kind: scheduler.KindExec, Owner: "chain-1", SessionID: "sess-chain",
		Command: "test coverage", CWD: "/proj",
	}, "added unit tests for the session manager")

	d := k.Scheduler.Decide(scheduler.IntentRequest{
		Kind: scheduler.KindExec, SessionID: "sess-chain", Owner: "chain-1",
		Command: "add more tests", CWD: "/proj",
	})
	if d.Outcome != scheduler.OutcomeAdmit {
		t.Fatalf("scheduler denied: %s — %s", d.Outcome, d.Reason)
	}

	k.Watchdog.Watch(watchdog.WatchOptions{
		ExecutionID: "chain-1", SessionID: "sess-chain",
		LockKey: d.LockKey, Timeout: 30 * time.Second,
	})
	if k.Watchdog.Pending() != 1 {
		t.Fatalf("Watchdog.Pending=%d", k.Watchdog.Pending())
	}

	for i := 0; i < 2; i++ {
		k.Bus.Publish(eventbus.Envelope{
			Source:    eventbus.SourceSession,
			SessionID: "sess-chain",
			Topic:     protocol.EventTypeLog,
			Payload: protocol.LogEvent{
				Event: protocol.Event{
					Type:        protocol.EventTypeLog,
					SessionID:   "sess-chain",
					RuntimeMeta: protocol.RuntimeMeta{ExecutionID: "chain-1"},
				},
				Message: "test output",
			},
		})
	}

	k.Watchdog.Settle("chain-1")
	k.Scheduler.Release(d.LockKey, "chain-1")

	if owner, _ := k.LockMgr.Holder("sess-chain"); owner != "" {
		t.Fatalf("lock not released")
	}
}

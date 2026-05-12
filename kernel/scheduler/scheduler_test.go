package scheduler

import (
	"testing"
	"time"

	"mobilevc/kernel/lockmgr"
)

func TestDecideAdmitsAndAcquires(t *testing.T) {
	lm := lockmgr.New(nil)
	defer lm.Close()
	s := New(lm, Config{DefaultLockTTL: 100 * time.Millisecond})

	d := s.Decide(IntentRequest{
		Kind:      KindExec,
		SessionID: "sess-1",
		Owner:     "exec-1",
		Command:   "claude",
	})
	if d.Outcome != OutcomeAdmit {
		t.Fatalf("Outcome=%s want admit", d.Outcome)
	}
	if d.LockKey != "sess-1" {
		t.Fatalf("LockKey=%q", d.LockKey)
	}
	if owner, _ := lm.Holder("sess-1"); owner != "exec-1" {
		t.Fatalf("Holder=%q", owner)
	}
}

func TestDecideDefersOnConflict(t *testing.T) {
	lm := lockmgr.New(nil)
	defer lm.Close()
	s := New(lm, Config{})

	if _, err := lm.Acquire("sess-1", "exec-A", time.Hour); err != nil {
		t.Fatal(err)
	}

	d := s.Decide(IntentRequest{
		Kind:      KindExec,
		SessionID: "sess-1",
		Owner:     "exec-B",
	})
	if d.Outcome != OutcomeDefer {
		t.Fatalf("Outcome=%s want defer", d.Outcome)
	}
	if d.Conflict != "exec-A" {
		t.Fatalf("Conflict=%q want exec-A", d.Conflict)
	}
}

func TestRuleShortCircuits(t *testing.T) {
	lm := lockmgr.New(nil)
	defer lm.Close()
	s := New(lm, Config{})

	denyEngineRule := func(req *IntentRequest) Decision {
		if req.Engine == "blocked-engine" {
			return Decision{Outcome: OutcomeDeny, Reason: "engine disabled"}
		}
		return Decision{}
	}
	s.AddRule(denyEngineRule)

	d := s.Decide(IntentRequest{
		Kind:      KindAITurn,
		SessionID: "s",
		Owner:     "o",
		Engine:    "blocked-engine",
	})
	if d.Outcome != OutcomeDeny {
		t.Fatalf("Outcome=%s want deny", d.Outcome)
	}
	if owner, _ := lm.Holder("s"); owner != "" {
		t.Fatalf("rule denied but lock acquired, holder=%q", owner)
	}
}

func TestReleaseFreesLock(t *testing.T) {
	lm := lockmgr.New(nil)
	defer lm.Close()
	s := New(lm, Config{})

	d := s.Decide(IntentRequest{Kind: KindExec, SessionID: "s", Owner: "o"})
	if d.Outcome != OutcomeAdmit {
		t.Fatalf("Outcome=%s", d.Outcome)
	}
	s.Release(d.LockKey, "o")
	if owner, _ := lm.Holder("s"); owner != "" {
		t.Fatalf("Holder=%q after Release", owner)
	}
}

func TestReentrantAdmitForSameOwner(t *testing.T) {
	lm := lockmgr.New(nil)
	defer lm.Close()
	s := New(lm, Config{})

	d1 := s.Decide(IntentRequest{Kind: KindExec, SessionID: "s", Owner: "o"})
	d2 := s.Decide(IntentRequest{Kind: KindExec, SessionID: "s", Owner: "o"})
	if d1.Outcome != OutcomeAdmit || d2.Outcome != OutcomeAdmit {
		t.Fatalf("both should admit reentrantly: d1=%s d2=%s", d1.Outcome, d2.Outcome)
	}
}

func TestNoLockKeyAdmits(t *testing.T) {
	lm := lockmgr.New(nil)
	defer lm.Close()
	s := New(lm, Config{})

	d := s.Decide(IntentRequest{Kind: KindSlash})
	if d.Outcome != OutcomeAdmit {
		t.Fatalf("Outcome=%s", d.Outcome)
	}
	if d.LockKey != "" {
		t.Fatalf("LockKey=%q want empty", d.LockKey)
	}
}

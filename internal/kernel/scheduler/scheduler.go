// Package scheduler is the "意图调度器" — it inspects incoming user intents
// (execute / ai_turn / slash command) before they reach session.Service and
// returns an admission decision.
//
// In P1 it is a thin rule-driven preflight: it checks lockmgr conflicts and
// runs a registered chain of Rules. Memory MOE injection is reserved for P2,
// where the scheduler becomes the consumer of MOE-routed context.
package scheduler

import (
	"strings"
	"sync"
	"time"

	"mobilevc/internal/kernel/lockmgr"
)

// Kind classifies an incoming intent.
type Kind string

const (
	KindExec    Kind = "exec"
	KindAITurn  Kind = "ai_turn"
	KindSlash   Kind = "slash"
	KindSkill   Kind = "skill"
	KindUnknown Kind = "unknown"
)

// IntentRequest is the input to Decide.
type IntentRequest struct {
	Kind        Kind
	SessionID   string
	ExecutionID string   // optional; assigned by caller if known
	Owner       string   // typically the same as ExecutionID or ConnectionID
	ResourceKey string   // optional lock key; defaults to SessionID
	Engine      string
	Command     string
	CWD         string
	Tags        []string // free-form metadata for rules
}

// Outcome is the disposition the scheduler chose.
type Outcome string

const (
	OutcomeAdmit       Outcome = "admit"
	OutcomeDeny        Outcome = "deny"
	OutcomeDefer       Outcome = "defer"
	OutcomeNeedConfirm Outcome = "need_confirm"
)

// Decision is the output of Decide.
type Decision struct {
	Outcome  Outcome
	Reason   string
	LockKey  string        // when Admit, the key that was acquired
	LockTTL  time.Duration // when Admit, the TTL granted
	Conflict string        // when Defer, the owner currently holding the resource
}

// Rule is a pluggable advisor consulted before the default lock check.
// Returning OutcomeAdmit (the zero outcome) means "no opinion, continue".
// The Rule may mutate *IntentRequest (e.g. to enrich Tags with MOE context).
type Rule func(req *IntentRequest) Decision

// Config tunes a Scheduler.
type Config struct {
	DefaultLockTTL time.Duration
}

// Scheduler is the intent admission entry point.
type Scheduler struct {
	lock *lockmgr.Manager
	cfg  Config

	mu    sync.RWMutex
	rules []Rule
}

// New constructs a Scheduler. lock is required; rules can be added via AddRule.
func New(lock *lockmgr.Manager, cfg Config) *Scheduler {
	if cfg.DefaultLockTTL <= 0 {
		cfg.DefaultLockTTL = 10 * time.Minute
	}
	return &Scheduler{lock: lock, cfg: cfg}
}

// AddRule appends a rule. Rules are evaluated in registration order; the
// first rule returning a non-Admit outcome short-circuits.
func (s *Scheduler) AddRule(r Rule) {
	if r == nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rules = append(s.rules, r)
}

// Decide runs the rule chain, then attempts to lock the resource.
//
// If the request omits ResourceKey, SessionID is used. If both are empty the
// request is admitted without locking (callers that need isolation should
// always provide a key).
func (s *Scheduler) Decide(req IntentRequest) Decision {
	if req.Kind == "" {
		req.Kind = KindUnknown
	}
	s.mu.RLock()
	rules := append([]Rule(nil), s.rules...)
	s.mu.RUnlock()
	for _, r := range rules {
		if d := r(&req); d.Outcome != "" && d.Outcome != OutcomeAdmit {
			return d
		}
	}

	key := strings.TrimSpace(req.ResourceKey)
	if key == "" {
		key = strings.TrimSpace(req.SessionID)
	}
	if key == "" || s.lock == nil {
		return Decision{Outcome: OutcomeAdmit, Reason: "no resource to lock"}
	}

	owner := strings.TrimSpace(req.Owner)
	if owner == "" {
		owner = strings.TrimSpace(req.ExecutionID)
	}
	if owner == "" {
		owner = "anonymous"
	}

	ttl := s.cfg.DefaultLockTTL
	lease, err := s.lock.Acquire(key, owner, ttl)
	if err == lockmgr.ErrConflict {
		holder, _ := s.lock.Holder(key)
		return Decision{
			Outcome:  OutcomeDefer,
			Reason:   "resource held by another owner",
			Conflict: holder,
		}
	}
	if err != nil {
		return Decision{Outcome: OutcomeDeny, Reason: err.Error()}
	}
	return Decision{
		Outcome: OutcomeAdmit,
		LockKey: lease.Key,
		LockTTL: ttl,
	}
}

// Release frees a resource previously acquired via Decide. The owner must
// match what Decide saw.
func (s *Scheduler) Release(key, owner string) {
	if s == nil || s.lock == nil || key == "" {
		return
	}
	_ = s.lock.Release(key, owner)
}

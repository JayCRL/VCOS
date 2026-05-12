// Package moe implements the Memory Mixture-of-Experts router.
// It classifies an incoming intent into a (Type × Domain) pair, routes
// retrieval to the appropriate expert, and surfaces the resulting context
// as a scheduler.Rule that enriches the IntentRequest before admission.
//
// Rule-based routing (P2): type is derived from the event source and kind;
// domain is derived from command/target heuristics. Future P3+ will add
// embedding-based classification.
package moe

import (
	"context"
	"strings"

	"mobilevc/internal/memory"
	"mobilevc/internal/kernel/scheduler"
)

// Router classifies intents and dispatches to the appropriate memory expert.
type Router struct {
	Store memory.Store
}

// NewRouter creates a Router backed by the given memory store.
func NewRouter(store memory.Store) *Router {
	return &Router{Store: store}
}

// Classify maps an intent request to a memory (Type, Domain) pair using
// rule-based heuristics.
func (r *Router) Classify(req scheduler.IntentRequest) (memory.Type, memory.Domain) {
	// — Type —
	typ := memory.TypeShortTerm
	switch req.Kind {
	case scheduler.KindExec, scheduler.KindAITurn:
		typ = memory.TypeShortTerm // current dialog turn
	case scheduler.KindSlash, scheduler.KindSkill:
		typ = memory.TypeProject // prompts/skills are project-scoped
	}
	if req.CWD != "" && (typ == memory.TypeShortTerm) {
		// Intent has project context → promote to project memory.
		typ = memory.TypeProject
	}

	// — Domain —
	domain := memory.DomainDialogue
	cmd := strings.ToLower(req.Command)
	target := strings.ToLower(strings.Join(req.Tags, " "))
	combined := cmd + " " + target
	switch {
	case strings.Contains(combined, "task") || strings.Contains(combined, "todo") ||
		strings.Contains(combined, "plan") || strings.Contains(combined, "schedule") ||
		strings.Contains(combined, "check") || strings.Contains(combined, "status"):
		domain = memory.DomainTask
	case strings.Contains(combined, "code") || strings.Contains(combined, "refactor") ||
		strings.Contains(combined, "fix") || strings.Contains(combined, "bug") ||
		strings.Contains(combined, "test") || strings.Contains(combined, "build") ||
		strings.Contains(combined, "deploy") || strings.Contains(combined, "review"):
		domain = memory.DomainCode
	default:
		domain = memory.DomainDialogue
	}
	return typ, domain
}

// Retrieve queries the memory store for entries relevant to the given intent,
// using the classification result to scope the filter.
func (r *Router) Retrieve(ctx context.Context, req scheduler.IntentRequest, k int) ([]memory.Hit, error) {
	typ, domain := r.Classify(req)
	f := memory.Filter{
		Types:   []memory.Type{typ},
		Domains: []memory.Domain{domain},
		Limit:   k * 2, // over-fetch then trim via similarity
	}
	if req.CWD != "" {
		f.CWD = req.CWD
	}
	if req.SessionID != "" {
		f.SessionID = req.SessionID
	}
	queryText := req.Command
	if queryText == "" {
		queryText = strings.Join(req.Tags, " ")
	}
	if queryText == "" {
		queryText = string(req.Kind)
	}
	return r.Store.QuerySimilar(ctx, queryText, f, k)
}

// SchedulerRule returns a scheduler.Rule that queries memory before admission
// and attaches relevant context hits to the intent's Tags as "moe:hit=<id>".
func (r *Router) SchedulerRule(k int) scheduler.Rule {
	return func(req *scheduler.IntentRequest) scheduler.Decision {
		if r.Store == nil {
			return scheduler.Decision{}
		}
		hits, err := r.Retrieve(context.Background(), *req, k)
		if err != nil || len(hits) == 0 {
			return scheduler.Decision{}
		}
		for _, hit := range hits {
			req.Tags = append(req.Tags, "moe:hit="+hit.Entry.ID)
		}
		// Always Admit — this rule only enriches, never blocks.
		return scheduler.Decision{Outcome: scheduler.OutcomeAdmit}
	}
}

// WriteMemory persists a memory entry derived from an intent. This is the
// "initial Memory 写入" path from the architecture diagram.
func (r *Router) WriteMemory(ctx context.Context, req scheduler.IntentRequest, content string) error {
	if r == nil || r.Store == nil || req.SessionID == "" {
		return nil
	}
	typ, domain := r.Classify(req)
	entry := memory.Entry{
		ID:      "mem-" + req.Owner + "-" + string(req.Kind),
		Type:    typ,
		Domain:  domain,
		Title:   req.Command,
		Content: content,
		Source:  memory.SourceKernel,
		SessionID: req.SessionID,
		CWD:     req.CWD,
	}
	return r.Store.Upsert(ctx, entry)
}

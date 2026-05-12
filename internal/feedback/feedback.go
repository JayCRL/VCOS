// Package feedback implements the user feedback loop — the topmost layer in the
// AgentOS architecture. It turns evolution results into reviewable suggestions,
// records user decisions (accept / reject / adjust), and writes the adjusted
// confidence back to the Memory MOE, closing the "进化建议 → 用户确认 → 写回
// 系统生效" loop.
package feedback

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"mobilevc/internal/memory"
)

// Decision is the user's response to a suggestion.
type Decision string

const (
	DecisionAccept Decision = "accept"
	DecisionReject Decision = "reject"
	DecisionAdjust Decision = "adjust"
)

// Suggestion is a proposed change or learning for user review.
type Suggestion struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Learnings   []string  `json:"learnings"`   // extracted patterns
	Confidence  float64   `json:"confidence"`   // 0-1
	Source      string    `json:"source"`       // "evolve", "intake", "manual"
	SessionID   string    `json:"sessionId,omitempty"`
	CreatedAt   time.Time `json:"createdAt"`
}

// Record captures a user's decision on a suggestion.
type Record struct {
	SuggestionID string    `json:"suggestionId"`
	Decision     Decision  `json:"decision"`
	AdjustedText string    `json:"adjustedText,omitempty"` // user's adjusted text
	Timestamp    time.Time `json:"timestamp"`
}

// Stats holds aggregate feedback statistics.
type Stats struct {
	Total      int `json:"total"`
	Accepted   int `json:"accepted"`
	Rejected   int `json:"rejected"`
	Adjusted   int `json:"adjusted"`
	Pending    int `json:"pending"`
}

// Store is the subset of memory.Store needed by the feedback controller.
type Store interface {
	Upsert(ctx context.Context, entry memory.Entry) error
	Get(ctx context.Context, id string) (memory.Entry, error)
	Delete(ctx context.Context, id string) error
	Query(ctx context.Context, f memory.Filter) ([]memory.Entry, error)
}

// Controller manages the feedback lifecycle.
type Controller struct {
	Store Store

	mu      sync.RWMutex
	history []Record
	pending map[string]Suggestion
}

// New creates a feedback controller backed by the given memory store.
func New(store Store) *Controller {
	return &Controller{
		Store:   store,
		pending: make(map[string]Suggestion),
	}
}

// Propose creates a new suggestion from evolution learnings and adds it to the pending set.
func (c *Controller) Propose(title, source, sessionID string, learnings []string, confidence float64) Suggestion {
	s := Suggestion{
		ID:          fmt.Sprintf("sug-%d", time.Now().UnixNano()),
		Title:       title,
		Description: fmt.Sprintf("Evolution result: %s (%d findings)", title, len(learnings)),
		Learnings:   learnings,
		Confidence:  confidence,
		Source:      source,
		SessionID:   sessionID,
		CreatedAt:   time.Now().UTC(),
	}
	c.mu.Lock()
	c.pending[s.ID] = s
	c.mu.Unlock()
	return s
}

// ProposeFromEvolveResult creates suggestions from an evolution result.
func (c *Controller) ProposeFromEvolveResult(title, sessionID string, passed bool, learnings []string) []Suggestion {
	var suggestions []Suggestion
	confidence := 0.9
	if !passed {
		confidence = 0.6
	}
	s := c.Propose(title, "evolve", sessionID, learnings, confidence)
	suggestions = append(suggestions, s)
	return suggestions
}

// Decide records a user decision and updates the Memory MOE accordingly.
func (c *Controller) Decide(ctx context.Context, suggestionID string, decision Decision, adjustedText string) (Record, error) {
	c.mu.Lock()
	sug, ok := c.pending[suggestionID]
	if ok {
		delete(c.pending, suggestionID)
	}
	c.mu.Unlock()

	if !ok {
		return Record{}, fmt.Errorf("feedback: suggestion %s not found", suggestionID)
	}

	rec := Record{
		SuggestionID: suggestionID,
		Decision:     decision,
		AdjustedText: adjustedText,
		Timestamp:    time.Now().UTC(),
	}

	c.mu.Lock()
	c.history = append(c.history, rec)
	c.mu.Unlock()

	// Write back to Memory MOE.
	if c.Store != nil {
		c.applyDecision(ctx, sug, rec)
	}

	return rec, nil
}

// applyDecision updates the Memory store based on the user's decision.
func (c *Controller) applyDecision(ctx context.Context, sug Suggestion, rec Record) {
	now := time.Now().UTC()
	for i, pattern := range sug.Learnings {
		memID := fmt.Sprintf("feedback-%s-%d", sug.ID, i)
		switch rec.Decision {
		case DecisionAccept:
			_ = c.Store.Upsert(ctx, memory.Entry{
				ID:        memID,
				Type:      memory.TypeLongTerm,
				Domain:    memory.DomainCode,
				Title:     pattern,
				Content:   fmt.Sprintf("confirmed pattern: %s (confidence=%.2f)", pattern, sug.Confidence),
				Source:    memory.SourceUser, // user-confirmed
				SessionID: sug.SessionID,
				Metadata:  map[string]any{"confidence": sug.Confidence, "status": "accepted"},
				CreatedAt: now, UpdatedAt: now,
			})
		case DecisionReject:
			// Write as low-confidence so MOE deprioritizes it.
			_ = c.Store.Upsert(ctx, memory.Entry{
				ID:        memID,
				Type:      memory.TypeLongTerm,
				Domain:    memory.DomainCode,
				Title:     pattern,
				Content:   fmt.Sprintf("rejected pattern: %s (confidence=0.1)", pattern),
				Source:    memory.SourceUser,
				SessionID: sug.SessionID,
				Metadata:  map[string]any{"confidence": 0.1, "status": "rejected"},
				CreatedAt: now, UpdatedAt: now,
			})
		case DecisionAdjust:
			adjusted := strings.TrimSpace(rec.AdjustedText)
			if adjusted == "" {
				adjusted = pattern
			}
			_ = c.Store.Upsert(ctx, memory.Entry{
				ID:        memID,
				Type:      memory.TypeLongTerm,
				Domain:    memory.DomainCode,
				Title:     adjusted,
				Content:   fmt.Sprintf("adjusted pattern: %s (user corrected from: %s)", adjusted, pattern),
				Source:    memory.SourceUser,
				SessionID: sug.SessionID,
				Metadata:  map[string]any{"confidence": 0.8, "status": "adjusted", "original": pattern},
				CreatedAt: now, UpdatedAt: now,
			})
		}
	}
}

// Pending returns all suggestions awaiting a decision.
func (c *Controller) Pending() []Suggestion {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Suggestion, 0, len(c.pending))
	for _, s := range c.pending {
		out = append(out, s)
	}
	return out
}

// History returns all recorded decisions.
func (c *Controller) History() []Record {
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]Record, len(c.history))
	copy(out, c.history)
	return out
}

// Stats returns aggregate feedback statistics.
func (c *Controller) Stats() Stats {
	c.mu.RLock()
	defer c.mu.RUnlock()
	s := Stats{Pending: len(c.pending)}
	for _, r := range c.history {
		s.Total++
		switch r.Decision {
		case DecisionAccept:
			s.Accepted++
		case DecisionReject:
			s.Rejected++
		case DecisionAdjust:
			s.Adjusted++
		}
	}
	return s
}

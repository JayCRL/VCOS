// Package vibe manages the user's interaction preferences — style, proactivity,
// and role — as a stateful controller backed by the memory store. The scheduler
// and other kernel components can query the current vibe to tailor behaviour.
//
// This is the "Vibe 控制器" block from the architecture diagram: it governs
// "偏好/行为流" and feeds into task dispatch and MOE routing.
package vibe

import (
	"context"
	"strings"
	"sync"
	"time"

	"mobilevc/internal/memory"
)

// Style governs verbosity and tone.
type Style string

const (
	StyleConcise  Style = "concise"
	StyleVerbose  Style = "verbose"
	StyleBalanced Style = "balanced"
)

// Proactivity governs how much the agent acts without explicit prompting.
type Proactivity string

const (
	ProactivityPassive  Proactivity = "passive"
	ProactivityBalanced Proactivity = "balanced"
	ProactivityActive   Proactivity = "active"
)

// Role is the user's current functional role.
type Role string

const (
	RoleDeveloper Role = "developer"
	RoleReviewer  Role = "reviewer"
	RoleArchitect Role = "architect"
	RoleDebugger  Role = "debugger"
)

// State is a snapshot of current preferences.
type State struct {
	Style       Style       `json:"style"`
	Proactivity Proactivity `json:"proactivity"`
	Role        Role        `json:"role"`
	UpdatedAt   time.Time   `json:"updatedAt"`
}

var defaultState = State{
	Style:       StyleBalanced,
	Proactivity: ProactivityBalanced,
	Role:        RoleDeveloper,
}

// Store is the subset of memory.Store needed by the Vibe controller.
type Store interface {
	Upsert(ctx context.Context, entry memory.Entry) error
	Query(ctx context.Context, f memory.Filter) ([]memory.Entry, error)
}

// Controller manages vibe state per session.
type Controller struct {
	store Store

	mu    sync.RWMutex
	cache map[string]State // sessionID → state
}

// New creates a Vibe controller backed by the given memory store.
func New(store Store) *Controller {
	return &Controller{store: store, cache: make(map[string]State)}
}

const vibeMemoryIDPrefix = "vibe-state-"

// Get returns the current vibe state for a session. Returns defaults if none set.
func (c *Controller) Get(ctx context.Context, sessionID string) State {
	c.mu.RLock()
	if s, ok := c.cache[sessionID]; ok {
		c.mu.RUnlock()
		return s
	}
	c.mu.RUnlock()

	if c.store == nil {
		return defaultState
	}
	entries, err := c.store.Query(ctx, memory.Filter{
		Types:     []memory.Type{memory.TypeLongTerm},
		Domains:   []memory.Domain{memory.DomainDialogue},
		SessionID: sessionID,
		Limit:     1,
	})
	if err != nil || len(entries) == 0 {
		return defaultState
	}
	state := parseVibeContent(entries[0].Content)
	c.mu.Lock()
	c.cache[sessionID] = state
	c.mu.Unlock()
	return state
}

// Set updates the vibe state for a session and persists it.
func (c *Controller) Set(ctx context.Context, sessionID string, state State) error {
	state.UpdatedAt = time.Now().UTC()
	c.mu.Lock()
	c.cache[sessionID] = state
	c.mu.Unlock()

	if c.store == nil {
		return nil
	}
	return c.store.Upsert(ctx, memory.Entry{
		ID:        vibeMemoryIDPrefix + sessionID,
		Type:      memory.TypeLongTerm,
		Domain:    memory.DomainDialogue,
		Title:     "vibe state",
		Content:   formatVibeContent(state),
		SessionID: sessionID,
		Source:    memory.SourceKernel,
		CreatedAt: state.UpdatedAt,
		UpdatedAt: state.UpdatedAt,
	})
}

// UpdateStyle is a convenience setter for style only.
func (c *Controller) UpdateStyle(ctx context.Context, sessionID string, style Style) error {
	s := c.Get(ctx, sessionID)
	s.Style = style
	return c.Set(ctx, sessionID, s)
}

// UpdateProactivity is a convenience setter for proactivity only.
func (c *Controller) UpdateProactivity(ctx context.Context, sessionID string, p Proactivity) error {
	s := c.Get(ctx, sessionID)
	s.Proactivity = p
	return c.Set(ctx, sessionID, s)
}

// UpdateRole is a convenience setter for role only.
func (c *Controller) UpdateRole(ctx context.Context, sessionID string, role Role) error {
	s := c.Get(ctx, sessionID)
	s.Role = role
	return c.Set(ctx, sessionID, s)
}

// Tags returns the vibe state as key=value tags for a session.
func (c *Controller) Tags(ctx context.Context, sessionID string) []string {
	s := c.Get(ctx, sessionID)
	return []string{
		"vibe:style=" + string(s.Style),
		"vibe:proactivity=" + string(s.Proactivity),
		"vibe:role=" + string(s.Role),
	}
}

// ——— helpers ———

func parseVibeContent(content string) State {
	s := defaultState
	for _, part := range strings.Fields(content) {
		kv := strings.SplitN(part, "=", 2)
		if len(kv) != 2 {
			continue
		}
		switch kv[0] {
		case "style":
			s.Style = Style(kv[1])
		case "proactivity":
			s.Proactivity = Proactivity(kv[1])
		case "role":
			s.Role = Role(kv[1])
		}
	}
	return s
}

func formatVibeContent(s State) string {
	return "style=" + string(s.Style) + " proactivity=" + string(s.Proactivity) + " role=" + string(s.Role)
}

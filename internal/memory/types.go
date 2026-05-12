// Package memory defines the Memory entity and its persistent storage
// interface. Memory entries flow from four sources — user input, session
// events, kernel decisions, and external integrations — into a typed,
// domain-tagged store. The Memory MOE (moe/ sub-package) routes queries
// across experts keyed by (Type × Domain).
package memory

import "time"

// Type categorises memory by retention policy. Used as the MOE primary expert
// dimension (decides write strategy, TTL, and archive behaviour).
type Type string

const (
	TypeShortTerm Type = "short_term" // transient, high-churn (current turn context)
	TypeLongTerm  Type = "long_term"  // durable, learned (preferences, patterns)
	TypeProject   Type = "project"    // scoped to the current workspace
)

// Domain categorises memory by subject matter. Used as the MOE secondary
// expert dimension (decides retrieval channel and embedding model affinity).
type Domain string

const (
	DomainCode    Domain = "code"
	DomainDialogue Domain = "dialogue"
	DomainTask    Domain = "task"
)

// CognitiveKind labels the cognitive function of a memory — reserved for
// future procedural / episodic classifiers (see design doc §4.4). Not used
// in rule-based routing yet.
type CognitiveKind string

const (
	CogEpisodic   CognitiveKind = "episodic"
	CogSemantic   CognitiveKind = "semantic"
	CogProcedural CognitiveKind = "procedural"
	CogWorking    CognitiveKind = "working"
)

// Source records where a memory entry was created from.
type Source string

const (
	SourceUser    Source = "user"
	SourceSession Source = "session"
	SourceKernel  Source = "kernel"
	SourceExternal Source = "external"
)

// Entry is a single memory record.
type Entry struct {
	ID            string        `json:"id"`
	Type          Type          `json:"type"`
	Domain        Domain        `json:"domain,omitempty"`
	CognitiveKind CognitiveKind `json:"cognitiveKind,omitempty"`
	Title         string        `json:"title"`
	Content       string        `json:"content"`
	Source        Source        `json:"source,omitempty"`
	SessionID     string        `json:"sessionId,omitempty"`
	CWD           string        `json:"cwd,omitempty"`
	Embedding     []float32     `json:"embedding,omitempty"`
	Metadata      map[string]any  `json:"metadata,omitempty"`
	TTL           time.Duration   `json:"-"`
	CreatedAt     time.Time       `json:"createdAt"`
	UpdatedAt     time.Time       `json:"updatedAt"`
	ExpireAt      time.Time       `json:"expireAt,omitempty"`
}

// Expired reports whether the entry has a set TTL and that TTL has passed.
func (e Entry) Expired(now time.Time) bool {
	return !e.ExpireAt.IsZero() && now.After(e.ExpireAt)
}

// Filter selects entries from the store.
type Filter struct {
	Types          []Type
	Domains        []Domain
	CognitiveKinds []CognitiveKind
	SessionID      string
	CWD            string
	TitleContains  string
	ContentContains string
	Limit          int
	Offset         int
}

// Hit is a matched entry with an optional relevance score (0-1).
type Hit struct {
	Entry Entry
	Score float64
}

// Metadata is a convenience alias for JSON-compatible metadata.
type Metadata = map[string]any

// Package eventbus provides a process-wide pub/sub bus that sits on top of the
// existing per-session EventSink callbacks. It tags every event with a Source
// (user / session / kernel / external) and assigns a monotonic system-level
// cursor, so downstream consumers — the intent scheduler, watchdog, and lock
// manager introduced in Phase 1 — can subscribe with declarative filters
// instead of patching every callsite.
//
// Persistence and cross-process delivery are out of scope for P0; only the
// Persister interface is reserved.
package eventbus

import (
	"time"
)

// Source identifies where an event originated.
type Source string

const (
	SourceUser     Source = "user"
	SourceSession  Source = "session"
	SourceKernel   Source = "kernel"
	SourceExternal Source = "external"
)

// Envelope wraps a payload with bus-level metadata.
type Envelope struct {
	Cursor    int64
	Source    Source
	Topic     string
	SessionID string
	Timestamp time.Time
	Payload   any
}

// Filter selects envelopes a subscriber wants. Empty slices match everything
// in that dimension; non-empty slices match by ANY membership.
type Filter struct {
	Sources    []Source
	Topics     []string
	SessionIDs []string
}

// Match reports whether env satisfies the filter.
func (f Filter) Match(env Envelope) bool {
	if len(f.Sources) > 0 && !containsSource(f.Sources, env.Source) {
		return false
	}
	if len(f.Topics) > 0 && !containsString(f.Topics, env.Topic) {
		return false
	}
	if len(f.SessionIDs) > 0 && !containsString(f.SessionIDs, env.SessionID) {
		return false
	}
	return true
}

// Handler is invoked for each envelope a subscription receives.
type Handler func(Envelope)

// Subscription represents a live subscription. Close it to detach.
type Subscription interface {
	ID() string
	Close()
}

// Persister optionally persists the latest dispatched cursor for crash recovery.
// P0 does not wire any implementation; reserved for P1+.
type Persister interface {
	Save(cursor int64) error
	Load() (int64, error)
}

func containsSource(xs []Source, x Source) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

func containsString(xs []string, x string) bool {
	for _, v := range xs {
		if v == x {
			return true
		}
	}
	return false
}

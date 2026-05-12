// Package watchdog tracks long-running executions, fires an idle timeout when
// no activity is seen on the bus for a configured window, and force-releases
// any associated lock. It is the "进程看门人" in the architecture diagram.
//
// Behavior is intentionally non-destructive in P1: on timeout, the watchdog
// only emits a warning event onto the bus and releases the lock — it does
// not kill the underlying process. Force-kill semantics are deferred.
package watchdog

import (
	"sync"
	"time"

	"mobilevc/kernel/eventbus"
	"mobilevc/kernel/lockmgr"
	"mobilevc/protocol"
)

const (
	// DefaultIdleTimeout is the default per-execution idle window.
	DefaultIdleTimeout = 10 * time.Minute
	// MinIdleTimeout caps the lower bound to avoid runaway timer churn.
	MinIdleTimeout = 5 * time.Second
)

// WatchOptions configures a single Watch call.
type WatchOptions struct {
	ExecutionID string
	SessionID   string
	LockKey     string // optional; force-released on timeout
	Timeout     time.Duration
}

// Watchdog observes per-execution activity via the bus and times out idle work.
type Watchdog struct {
	bus  eventbus.Bus
	lock *lockmgr.Manager

	minIdle time.Duration

	mu    sync.Mutex
	items map[string]*item

	sub      eventbus.Subscription
	stopOnce sync.Once
}

type item struct {
	opts     WatchOptions
	deadline time.Time
	timer    *time.Timer
}

// Config tunes a Watchdog instance.
type Config struct {
	MinIdleTimeout time.Duration // floor; default DefaultIdleTimeout floor (MinIdleTimeout const)
}

// New creates a Watchdog with default config.
func New(bus eventbus.Bus, lock *lockmgr.Manager) *Watchdog {
	return NewWithConfig(bus, lock, Config{})
}

// NewWithConfig creates a Watchdog with the given config.
func NewWithConfig(bus eventbus.Bus, lock *lockmgr.Manager, cfg Config) *Watchdog {
	if bus == nil {
		return nil
	}
	if cfg.MinIdleTimeout <= 0 {
		cfg.MinIdleTimeout = MinIdleTimeout
	}
	w := &Watchdog{
		bus:     bus,
		lock:    lock,
		minIdle: cfg.MinIdleTimeout,
		items:   make(map[string]*item),
	}
	w.sub = bus.Subscribe("watchdog", eventbus.Filter{
		Sources: []eventbus.Source{eventbus.SourceSession, eventbus.SourceKernel},
	}, w.onEvent)
	return w
}

// Watch starts (or restarts) monitoring of an execution.
func (w *Watchdog) Watch(opts WatchOptions) {
	if w == nil || opts.ExecutionID == "" {
		return
	}
	if opts.Timeout < w.minIdle {
		if opts.Timeout > 0 {
			opts.Timeout = w.minIdle
		} else {
			opts.Timeout = DefaultIdleTimeout
		}
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if existing, ok := w.items[opts.ExecutionID]; ok {
		if existing.timer != nil {
			existing.timer.Stop()
		}
	}
	it := &item{opts: opts, deadline: time.Now().Add(opts.Timeout)}
	it.timer = time.AfterFunc(opts.Timeout, func() { w.onTimeout(opts.ExecutionID) })
	w.items[opts.ExecutionID] = it
}

// Heartbeat refreshes the idle deadline for an execution.
func (w *Watchdog) Heartbeat(executionID string) {
	if w == nil || executionID == "" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	it, ok := w.items[executionID]
	if !ok {
		return
	}
	it.deadline = time.Now().Add(it.opts.Timeout)
	if it.timer != nil {
		it.timer.Stop()
	}
	it.timer = time.AfterFunc(it.opts.Timeout, func() { w.onTimeout(executionID) })
}

// Settle marks an execution as completed and stops monitoring.
func (w *Watchdog) Settle(executionID string) {
	if w == nil || executionID == "" {
		return
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	if it, ok := w.items[executionID]; ok {
		if it.timer != nil {
			it.timer.Stop()
		}
		delete(w.items, executionID)
	}
}

// Pending returns the count of currently watched executions.
func (w *Watchdog) Pending() int {
	if w == nil {
		return 0
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	return len(w.items)
}

// Close detaches from the bus and stops all timers.
func (w *Watchdog) Close() {
	if w == nil {
		return
	}
	w.stopOnce.Do(func() {
		if w.sub != nil {
			w.sub.Close()
		}
		w.mu.Lock()
		for _, it := range w.items {
			if it.timer != nil {
				it.timer.Stop()
			}
		}
		w.items = nil
		w.mu.Unlock()
	})
}

func (w *Watchdog) onEvent(env eventbus.Envelope) {
	executionID := executionIDOf(env.Payload)
	if executionID != "" {
		w.Heartbeat(executionID)
	}
}

func (w *Watchdog) onTimeout(executionID string) {
	w.mu.Lock()
	it, ok := w.items[executionID]
	if !ok {
		w.mu.Unlock()
		return
	}
	if time.Now().Before(it.deadline) {
		// A heartbeat raced with the timer; reschedule.
		remaining := time.Until(it.deadline)
		it.timer = time.AfterFunc(remaining, func() { w.onTimeout(executionID) })
		w.mu.Unlock()
		return
	}
	opts := it.opts
	delete(w.items, executionID)
	w.mu.Unlock()

	if w.lock != nil && opts.LockKey != "" {
		w.lock.ForceRelease(opts.LockKey)
	}
	w.bus.Publish(eventbus.Envelope{
		Source:    eventbus.SourceKernel,
		Topic:     protocol.EventTypeError,
		SessionID: opts.SessionID,
		Timestamp: time.Now(),
		Payload: protocol.ErrorEvent{
			Event: protocol.Event{
				Type:      protocol.EventTypeError,
				Timestamp: time.Now().UTC(),
				SessionID: opts.SessionID,
				RuntimeMeta: protocol.RuntimeMeta{
					ExecutionID:  executionID,
					BlockingKind: "watchdog_timeout",
				},
			},
			Message: "watchdog: execution idle past " + opts.Timeout.String(),
			Code:    "watchdog_timeout",
		},
	})
}

func executionIDOf(payload any) string {
	switch e := payload.(type) {
	case protocol.LogEvent:
		return e.ExecutionID
	case protocol.ProgressEvent:
		return e.ExecutionID
	case protocol.ErrorEvent:
		return e.ExecutionID
	case protocol.AgentStateEvent:
		return e.ExecutionID
	case protocol.AIStatusEvent:
		return e.ExecutionID
	case protocol.RuntimePhaseEvent:
		return e.ExecutionID
	case protocol.StepUpdateEvent:
		return e.ExecutionID
	case protocol.FileDiffEvent:
		return e.ExecutionID
	case protocol.InteractionRequestEvent:
		return e.ExecutionID
	case protocol.PromptRequestEvent:
		return e.ExecutionID
	}
	return ""
}

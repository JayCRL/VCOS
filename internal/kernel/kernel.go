// Package kernel provides the agent orchestration layer — session lifecycle,
// AI command execution, permission routing, event projection, catalog management,
// and slash command dispatch. It is transport-agnostic: all I/O flows through
// EventSink callbacks. Consumers include the WebSocket gateway (mobile) and
// the desktop CLI daemon (agentd).
package kernel

import (
	"context"
	"fmt"
	"strings"
	"time"

	"mobilevc/internal/data"
	"mobilevc/internal/data/skills"
	"mobilevc/internal/engine"
	"mobilevc/internal/eventbus"
	"mobilevc/internal/evolve"
	"mobilevc/internal/feedback"
	"mobilevc/internal/intake"
	"mobilevc/internal/kernel/lockmgr"
	"mobilevc/internal/kernel/scheduler"
	"mobilevc/internal/kernel/watchdog"
	"mobilevc/internal/logx"
	"mobilevc/internal/memory"
	"mobilevc/internal/memory/moe"
	"mobilevc/internal/semantic"
	"mobilevc/internal/session"
	"mobilevc/internal/shadow"
	"mobilevc/internal/vibe"
)

const dataOpTimeout = 2 * time.Second

// EventSink delivers events to the transport layer.
type EventSink func(any)

// PushNotifier optionally sends push notifications (mobile-only).
type PushNotifier interface {
	SendIfNeeded(ctx context.Context, sessionID string, event any)
}

// ConnectionState holds the mutable per-connection session binding.
// It is owned by the transport (gateway or CLI) and read/modified by the kernel.
type ConnectionState struct {
	ConnectionID   string
	SessionID      string
	RemoteAddr     string
	RuntimeSvc     *session.Service
	ActiveRuntime  *RuntimeSession
	SelectedSessionID string
}

// Kernel is the agent orchestration core.
type Kernel struct {
	Store         data.Store
	SkillLauncher *skills.Launcher
	NewExecRunner func() engine.Runner
	NewPtyRunner  func() engine.Runner
	PushNotifier  PushNotifier
	Registry      *RuntimeSessionRegistry
	Bus           eventbus.Bus
	LockMgr       *lockmgr.Manager
	Watchdog      *watchdog.Watchdog
	Scheduler     *scheduler.Scheduler
	MemStore      memory.Store
	MoeRouter     *moe.Router
	Semantic      *semantic.Service
	Intake        *intake.SessionIntake
	Vibe          *vibe.Controller
	ShadowMgr     *shadow.Manager
	Evolver       *evolve.Evolver
	Feedback      *feedback.Controller
}

// New creates a new Kernel with defaults for unset fields.
func New(store data.Store) *Kernel {
	bus := eventbus.New()
	lm := lockmgr.New(nil)
	memStore := memory.NewMemStore()
	moeRouter := moe.NewRouter(memStore)
	vibeCtrl := vibe.New(memStore)
	shadowMgr := shadow.NewManager("")
	k := &Kernel{
		Store:     store,
		Registry:  NewRuntimeSessionRegistry(nil, 0, nil),
		Bus:       bus,
		LockMgr:   lm,
		Watchdog:  watchdog.New(bus, lm),
		Scheduler: scheduler.New(lm, scheduler.Config{}),
		MemStore:  memStore,
		MoeRouter: moeRouter,
		Semantic:  semantic.NewDefaultService(),
		Intake:    intake.New(memStore),
		Vibe:      vibeCtrl,
		ShadowMgr: shadowMgr,
		Evolver:   evolve.New(shadowMgr, memStore),
		Feedback:  feedback.New(memStore),
	}
	// Register MOE as the first rule, then Vibe tagging.
	k.Scheduler.AddRule(moeRouter.SchedulerRule(3))
	k.Scheduler.AddRule(func(req *scheduler.IntentRequest) scheduler.Decision {
		tags := vibeCtrl.Tags(context.Background(), req.SessionID)
		req.Tags = append(req.Tags, tags...)
		return scheduler.Decision{} // enrich only, always admit
	})
	if k.NewExecRunner == nil {
		k.NewExecRunner = func() engine.Runner { return engine.NewExecRunner() }
	}
	if k.NewPtyRunner == nil {
		k.NewPtyRunner = func() engine.Runner { return engine.NewPtyRunner() }
	}
	if k.SkillLauncher == nil {
		k.SkillLauncher = skills.NewLauncher(store)
	}
	// Wire Registry's service factory to use the kernel's runner factories.
	k.Registry = NewRuntimeSessionRegistry(func(sessionID string) *session.Service {
		return session.NewService(sessionID, session.Dependencies{
			NewExecRunner: k.NewExecRunner,
			NewPtyRunner:  k.NewPtyRunner,
		})
	}, DefaultReleaseAfter, k.defaultCleanup)
	return k
}

// ActiveSessions returns the list of session IDs currently tracked by the
// registry. Used by the dashboard.
func (k *Kernel) ActiveSessions() []string {
	if k == nil || k.Registry == nil {
		return nil
	}
	return k.Registry.SessionIDs()
}

// NewDetachedService creates a session.Service not bound to any session.
func (k *Kernel) NewDetachedService() *session.Service {
	return session.NewService("", session.Dependencies{
		NewExecRunner: k.NewExecRunner,
		NewPtyRunner:  k.NewPtyRunner,
	})
}

// Stop releases the kernel's runtime resources: it tears down the watchdog,
// lock manager, and event bus. The registry is not cleaned up here — call
// Registry.CleanupAll separately when shutting down a server.
func (k *Kernel) Stop() {
	if k == nil {
		return
	}
	if k.Watchdog != nil {
		k.Watchdog.Close()
	}
	if k.LockMgr != nil {
		k.LockMgr.Close()
	}
	if k.Bus != nil {
		_ = k.Bus.Close()
	}
}

func (k *Kernel) defaultCleanup(sessionID string) {
	if k.Store == nil || sessionID == "" {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), dataOpTimeout)
	defer cancel()
	record, err := k.Store.GetSession(ctx, sessionID)
	if err != nil {
		return
	}
	record.Summary.ExecutionActive = false
	k.Store.UpsertSession(ctx, record)
}

// NewRuntimeSession creates a new RuntimeSession wired to a session.Service.
func (k *Kernel) NewRuntimeSession(svc *session.Service) *RuntimeSession {
	return newRuntimeSession(svc)
}

// ConsoleExec executes a free-text message as an AI command for the dashboard
// console. If sessionID is empty, a new session is created via the store.
// Events flow to the bus so the dashboard SSE picks them up.
func (k *Kernel) ConsoleExec(ctx context.Context, sessionID, message, cwd string) (string, error) {
	if strings.TrimSpace(message) == "" {
		return sessionID, nil
	}
	if cwd == "" {
		cwd = "."
	}

	// If no session provided, create one through the store.
	if strings.TrimSpace(sessionID) == "" {
		summary, err := k.Store.CreateSession(ctx, "Dashboard Console")
		if err != nil {
			return "", fmt.Errorf("create session: %w", err)
		}
		sessionID = summary.ID
	}

	svc := session.NewService(sessionID, session.Dependencies{
		NewExecRunner: k.NewExecRunner,
		NewPtyRunner:  k.NewPtyRunner,
	})

	req := session.ExecuteRequest{
		Command:      "claude",
		CWD:          cwd,
		Mode:         engine.ModePTY,
		InitialInput: message,
	}

	// Sink publishes events to the bus so the dashboard SSE can display them.
	busSink := func(ev any) {
		if k.Bus != nil {
			k.Bus.Publish(eventbus.Envelope{
				Source:    eventbus.SourceSession,
				Topic:     eventbus.TopicOf(ev),
				SessionID: sessionID,
				Timestamp: time.Now(),
				Payload:   ev,
			})
		}
	}

	logx.Info("kernel", "console exec: sessionID=%s message=%s", sessionID, message[:min(len(message), 60)])
	if err := svc.Execute(ctx, sessionID, req, busSink); err != nil {
		return sessionID, fmt.Errorf("console exec: %w", err)
	}
	return sessionID, nil
}

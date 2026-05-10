// Package kernel provides the agent orchestration layer — session lifecycle,
// AI command execution, permission routing, event projection, catalog management,
// and slash command dispatch. It is transport-agnostic: all I/O flows through
// EventSink callbacks. Consumers include the WebSocket gateway (mobile) and
// the desktop CLI daemon (agentd).
package kernel

import (
	"context"
	"time"

	"mobilevc/internal/data"
	"mobilevc/internal/data/skills"
	"mobilevc/internal/engine"
	"mobilevc/internal/session"
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
}

// New creates a new Kernel with defaults for unset fields.
func New(store data.Store) *Kernel {
	k := &Kernel{
		Store:    store,
		Registry: NewRuntimeSessionRegistry(nil, 0, nil),
	}
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

// NewDetachedService creates a session.Service not bound to any session.
func (k *Kernel) NewDetachedService() *session.Service {
	return session.NewService("", session.Dependencies{
		NewExecRunner: k.NewExecRunner,
		NewPtyRunner:  k.NewPtyRunner,
	})
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

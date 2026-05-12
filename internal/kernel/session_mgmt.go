package kernel

import (
	"context"
	"strings"

	"mobilevc/internal/data"
	"mobilevc/internal/protocol"
)

// ── ConnectionState management ──

// SwitchSession detaches from the current session and attaches to a new one.
func (k *Kernel) SwitchSession(conn *ConnectionState, sessionID string, sink EventSink) {
	nextSessionID := strings.TrimSpace(sessionID)
	previousSessionID := strings.TrimSpace(conn.SelectedSessionID)

	if previousSessionID == nextSessionID && nextSessionID != "" {
		if entry := k.Registry.Attach(nextSessionID, conn.ConnectionID, sink); entry != nil {
			conn.RuntimeSvc = entry.Service
			conn.ActiveRuntime = entry
		}
		conn.SelectedSessionID = nextSessionID
		return
	}

	if previousSessionID != "" {
		k.Registry.Release(previousSessionID, conn.ConnectionID, true)
	} else if conn.RuntimeSvc != nil {
		conn.RuntimeSvc.Cleanup()
	}

	conn.SelectedSessionID = nextSessionID
	if nextSessionID == "" {
		conn.RuntimeSvc = k.NewDetachedService()
		conn.ActiveRuntime = nil
		return
	}

	if entry := k.Registry.Attach(nextSessionID, conn.ConnectionID, sink); entry != nil {
		conn.RuntimeSvc = entry.Service
		conn.ActiveRuntime = entry
		return
	}
	conn.RuntimeSvc = k.NewDetachedService()
	conn.ActiveRuntime = nil
}

// EnsureSession ensures a runtime session exists for the given sessionID.
func (k *Kernel) EnsureSession(sessionID string) *RuntimeSession {
	return k.Registry.Ensure(sessionID)
}

// ── Session CRUD ──

// CreateSession creates a new session and switches to it.
func (k *Kernel) CreateSession(ctx context.Context, conn *ConnectionState, title, cwd string, sink EventSink) data.SessionSummary {
	if k.Store == nil {
		sink(protocol.NewErrorEvent(conn.SelectedSessionID, "session store unavailable", ""))
		return data.SessionSummary{}
	}

	created, err := k.Store.CreateSession(ctx, title)
	if err != nil {
		sink(protocol.NewErrorEvent(conn.SelectedSessionID, err.Error(), ""))
		return data.SessionSummary{}
	}

	cwd = strings.TrimSpace(cwd)
	if cwd != "" {
		record, getErr := k.Store.GetSession(ctx, created.ID)
		if getErr == nil {
			record.Projection.Runtime.CWD = cwd
			record.Projection.Runtime.Source = "mobilevc"
			record.Summary.Runtime = record.Projection.Runtime
			if _, upsertErr := k.Store.UpsertSession(ctx, record); upsertErr == nil {
				created = record.Summary
			}
		}
	}

	// Run cognitive intake for the new session.
	if k.Intake != nil {
		_, _ = k.Intake.Run(ctx, created.ID, cwd, title)
	}

	k.SwitchSession(conn, created.ID, sink)
	return created
}

// ListSessions lists all sessions from the store.
func (k *Kernel) ListSessions(ctx context.Context) ([]data.SessionSummary, error) {
	if k.Store == nil {
		return nil, nil
	}
	return k.Store.ListSessions(ctx)
}

// GetSession returns a session record.
func (k *Kernel) GetSession(ctx context.Context, sessionID string) (data.SessionRecord, error) {
	return k.Store.GetSession(ctx, sessionID)
}

// DeleteSession deletes a session and falls back if current.
func (k *Kernel) DeleteSession(ctx context.Context, conn *ConnectionState, sessionID string, sink EventSink) error {
	if k.Store == nil {
		return nil
	}

	record, err := k.Store.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}

	if record.Summary.External || strings.EqualFold(strings.TrimSpace(record.Summary.Source), "codex-native") {
		sink(protocol.NewErrorEvent(conn.SelectedSessionID, "external sessions cannot be deleted", ""))
		return nil
	}

	if err := k.Store.DeleteSession(ctx, sessionID); err != nil {
		return err
	}

	if sessionID == conn.SelectedSessionID {
		items, _ := k.Store.ListSessions(ctx)
		fallbackSessionID := ""
		for _, item := range items {
			if strings.TrimSpace(item.ID) != "" {
				fallbackSessionID = item.ID
				break
			}
		}
		k.SwitchSession(conn, fallbackSessionID, sink)
	}

	return nil
}

// ActiveRuntimeRecord checks if a session has an active runtime.
func (k *Kernel) ActiveRuntimeRecord(sessionID string) bool {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return false
	}
	entry := k.Registry.Get(sessionID)
	if entry == nil || entry.Service == nil {
		return false
	}
	snapshot := entry.Service.RuntimeSnapshot()
	return snapshot.Running && strings.TrimSpace(snapshot.ActiveSession) == sessionID
}

// RuntimeForSession resolves the (*RuntimeSession, *session.Service) for a sessionID.
func (k *Kernel) RuntimeForSession(sessionID string, conn *ConnectionState) (*RuntimeSession, interface{}) {
	trimmed := strings.TrimSpace(sessionID)
	if trimmed == "" {
		return nil, conn.RuntimeSvc
	}
	if trimmed == strings.TrimSpace(conn.SelectedSessionID) && conn.ActiveRuntime != nil {
		return conn.ActiveRuntime, conn.ActiveRuntime.Service
	}
	entry := k.Registry.Ensure(trimmed)
	if entry != nil {
		return entry, entry.Service
	}
	return nil, conn.RuntimeSvc
}

// ── Protocol helpers ──

func toProtocolSummary(item data.SessionSummary) protocol.SessionSummary {
	return protocol.SessionSummary{
		ID:              item.ID,
		Title:           item.Title,
		CreatedAt:       item.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:       item.UpdatedAt.Format("2006-01-02T15:04:05Z07:00"),
		LastPreview:     item.LastPreview,
		EntryCount:      item.EntryCount,
		Source:          item.Source,
		External:        item.External,
		Ownership:       item.Ownership,
		ExecutionActive: item.ExecutionActive,
	}
}

func toProtocolSummaries(items []data.SessionSummary) []protocol.SessionSummary {
	out := make([]protocol.SessionSummary, 0, len(items))
	for _, item := range items {
		out = append(out, toProtocolSummary(item))
	}
	return out
}

func toProtocolCatalogMetadata(meta data.CatalogMetadata) protocol.CatalogMetadata {
	return protocol.CatalogMetadata{
		Domain:        string(meta.Domain),
		SourceOfTruth: string(meta.SourceOfTruth),
		SyncState:     string(meta.SyncState),
		DriftDetected: meta.DriftDetected,
		LastSyncedAt:  meta.LastSyncedAt.Format("2006-01-02T15:04:05Z07:00"),
		VersionToken:  meta.VersionToken,
		LastError:     meta.LastError,
	}
}

func toProtocolSessionContext(ctx data.SessionContext) protocol.SessionContext {
	return protocol.SessionContext{
		EnabledSkillNames: ctx.EnabledSkillNames,
		EnabledMemoryIDs:  ctx.EnabledMemoryIDs,
	}
}

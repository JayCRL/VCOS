package wizard

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"mobilevc/memory"
)

// LoadCursor reads the wizard cursor for the given session.
// Returns (zero, false, nil) when there is no cursor yet.
func LoadCursor(ctx context.Context, store memory.Store, sid string) (CursorPayload, bool, error) {
	if store == nil || sid == "" {
		return CursorPayload{}, false, nil
	}
	entry, err := store.Get(ctx, IDCursor(sid))
	if err != nil {
		// Missing cursor is not an error — it means the wizard has not
		// started yet.
		return CursorPayload{}, false, nil
	}
	var c CursorPayload
	if entry.Content == "" {
		return CursorPayload{CurrentStage: StageUserIntent}, true, nil
	}
	if err := json.Unmarshal([]byte(entry.Content), &c); err != nil {
		return CursorPayload{}, false, fmt.Errorf("unmarshal cursor: %w", err)
	}
	return c, true, nil
}

// SaveCursor upserts the wizard cursor for the given session.
func SaveCursor(ctx context.Context, store memory.Store, sid string, c CursorPayload) error {
	if store == nil || sid == "" {
		return fmt.Errorf("save cursor: nil store or empty sid")
	}
	body, err := json.Marshal(c)
	if err != nil {
		return fmt.Errorf("marshal cursor: %w", err)
	}
	now := time.Now().UTC()
	return store.Upsert(ctx, memory.Entry{
		ID:        IDCursor(sid),
		Type:      memory.TypeShortTerm,
		Domain:    memory.DomainDialogue,
		Title:     "wizard cursor",
		Content:   string(body),
		Source:    memory.SourceUser,
		SessionID: sid,
		Metadata:  commonMeta(StageCursor),
		CreatedAt: now,
		UpdatedAt: now,
	})
}

// Advance marks `stage` as completed and sets `next` as the current stage.
// If `next` is empty, current stage is left unchanged (useful to record
// "completed but waiting for user to proceed").
func Advance(ctx context.Context, store memory.Store, sid string, stage, next Stage) error {
	cur, _, err := LoadCursor(ctx, store, sid)
	if err != nil {
		return err
	}
	if !contains(cur.CompletedStages, stage) {
		cur.CompletedStages = append(cur.CompletedStages, stage)
	}
	if next != "" {
		cur.CurrentStage = next
	}
	return SaveCursor(ctx, store, sid, cur)
}

func contains(stages []Stage, s Stage) bool {
	for _, x := range stages {
		if x == s {
			return true
		}
	}
	return false
}

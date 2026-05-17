package wizard

import (
	"context"
	"encoding/json"
	"strings"

	"mobilevc/memory"
)

// Snapshot is the union of every stage's persisted product for one session.
// Each field is a pointer so the GUI can tell "not yet completed" (nil) from
// "completed with zero value".
type Snapshot struct {
	SessionID      string                  `json:"sessionId"`
	Cursor         *CursorPayload          `json:"cursor,omitempty"`
	UserIntent     *UserIntentPayload      `json:"userIntent,omitempty"`
	ProjectIntent  *ProjectIntentPayload   `json:"projectIntent,omitempty"`
	UIComponents   []UIComponentPayload    `json:"uiComponents,omitempty"`
	UIPrompt       *UIPromptPayload        `json:"uiPrompt,omitempty"`
	TechPlan       *TechPlanPayload        `json:"techPlan,omitempty"`
	Permissions    *PermissionsPayload     `json:"permissions,omitempty"`
	DecisionStyle  *DecisionStylePayload   `json:"decisionStyle,omitempty"`
}

// LoadSnapshot rehydrates every wizard product for a session by scanning
// the memory store. Missing entries are left nil.
func LoadSnapshot(ctx context.Context, store memory.Store, sid string) (Snapshot, error) {
	snap := Snapshot{SessionID: sid}
	if store == nil || sid == "" {
		return snap, nil
	}

	if cur, ok, err := LoadCursor(ctx, store, sid); err != nil {
		return snap, err
	} else if ok {
		snap.Cursor = &cur
	}

	// Pull every wizard-* entry for this session in one query, then route
	// each by ID prefix. Cheaper than 7 round-trips.
	entries, err := store.Query(ctx, memory.Filter{SessionID: sid, Limit: 1024})
	if err != nil {
		return snap, err
	}

	uiPrefix := IDPrefix + "ui-component-" + sid + "-"
	for _, e := range entries {
		switch {
		case e.ID == IDUserIntent(sid):
			var p UserIntentPayload
			if err := json.Unmarshal([]byte(e.Content), &p); err == nil {
				snap.UserIntent = &p
			}
		case e.ID == IDProjectIntent(sid):
			var p ProjectIntentPayload
			if err := json.Unmarshal([]byte(e.Content), &p); err == nil {
				snap.ProjectIntent = &p
			}
		case strings.HasPrefix(e.ID, uiPrefix):
			var p UIComponentPayload
			if err := json.Unmarshal([]byte(e.Content), &p); err == nil {
				snap.UIComponents = append(snap.UIComponents, p)
			}
		case e.ID == IDUIPrompt(sid):
			var p UIPromptPayload
			if err := json.Unmarshal([]byte(e.Content), &p); err == nil {
				snap.UIPrompt = &p
			}
		case e.ID == IDTechPlan(sid):
			var p TechPlanPayload
			if err := json.Unmarshal([]byte(e.Content), &p); err == nil {
				snap.TechPlan = &p
			}
		case e.ID == IDPermissions(sid):
			var p PermissionsPayload
			if err := json.Unmarshal([]byte(e.Content), &p); err == nil {
				snap.Permissions = &p
			}
		case e.ID == IDDecisionStyle(sid):
			var p DecisionStylePayload
			if err := json.Unmarshal([]byte(e.Content), &p); err == nil {
				snap.DecisionStyle = &p
			}
		}
	}
	return snap, nil
}

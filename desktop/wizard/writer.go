package wizard

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"mobilevc/cognition/intake"
	"mobilevc/memory"
)

// WriteUserIntent persists Stage 1 — the user's free-text intent.
// Long-term + dialogue: the goal survives across sessions for the same user.
func WriteUserIntent(ctx context.Context, store memory.Store, sid, cwd, text string) error {
	if store == nil || sid == "" {
		return fmt.Errorf("write user intent: nil store or empty sid")
	}
	body, _ := json.Marshal(UserIntentPayload{Text: text})
	now := time.Now().UTC()
	return store.Upsert(ctx, memory.Entry{
		ID:        IDUserIntent(sid),
		Type:      memory.TypeLongTerm,
		Domain:    memory.DomainDialogue,
		Title:     "user intent",
		Content:   string(body),
		Source:    memory.SourceUser,
		SessionID: sid,
		CWD:       cwd,
		Metadata:  commonMeta(StageUserIntent),
		CreatedAt: now,
		UpdatedAt: now,
	})
}

// WriteProjectIntent persists Stage 2. The intake.CognitiveProfile is
// produced by intake.SessionIntake.Run, which also seeds three companion
// entries (intake-project/goal/pref-<sid>) that the MOE will pick up.
func WriteProjectIntent(ctx context.Context, store memory.Store, sid, cwd, prompt, userNote string, profile intake.CognitiveProfile) error {
	if store == nil || sid == "" {
		return fmt.Errorf("write project intent: nil store or empty sid")
	}
	payload := ProjectIntentPayload{Prompt: prompt, UserNote: userNote, Profile: profile}
	body, _ := json.Marshal(payload)
	now := time.Now().UTC()
	return store.Upsert(ctx, memory.Entry{
		ID:        IDProjectIntent(sid),
		Type:      memory.TypeProject,
		Domain:    memory.DomainTask,
		Title:     "project intent",
		Content:   string(body),
		Source:    memory.SourceUser,
		SessionID: sid,
		CWD:       cwd,
		Metadata:  commonMeta(StageProjectIntent),
		CreatedAt: now,
		UpdatedAt: now,
	})
}

// WriteUIComponent persists one component from the Stage 3 canvas.
// Caller is expected to call this once per component on save.
func WriteUIComponent(ctx context.Context, store memory.Store, sid, cwd string, comp UIComponentPayload) error {
	if store == nil || sid == "" || comp.ID == "" {
		return fmt.Errorf("write ui component: missing store/sid/componentID")
	}
	body, _ := json.Marshal(comp)
	now := time.Now().UTC()
	return store.Upsert(ctx, memory.Entry{
		ID:        IDUIComponent(sid, comp.ID),
		Type:      memory.TypeProject,
		Domain:    memory.DomainCode,
		Title:     "ui component: " + comp.Name,
		Content:   string(body),
		Source:    memory.SourceUser,
		SessionID: sid,
		CWD:       cwd,
		Metadata: commonMeta(StageUISpec, memory.Metadata{
			"componentId":   comp.ID,
			"componentKind": comp.Kind,
		}),
		CreatedAt: now,
		UpdatedAt: now,
	})
}

// WriteUIPrompt persists the synthesized mapping prompt for Stage 3.
func WriteUIPrompt(ctx context.Context, store memory.Store, sid, cwd, prompt string, componentIDs []string) error {
	if store == nil || sid == "" {
		return fmt.Errorf("write ui prompt: nil store or empty sid")
	}
	body, _ := json.Marshal(UIPromptPayload{Prompt: prompt, ComponentIDs: componentIDs})
	now := time.Now().UTC()
	return store.Upsert(ctx, memory.Entry{
		ID:        IDUIPrompt(sid),
		Type:      memory.TypeProject,
		Domain:    memory.DomainCode,
		Title:     "ui mapping prompt",
		Content:   string(body),
		Source:    memory.SourceUser,
		SessionID: sid,
		CWD:       cwd,
		Metadata:  commonMeta(StageUIPrompt),
		CreatedAt: now,
		UpdatedAt: now,
	})
}

// WriteTechPlan persists Stage 4.
func WriteTechPlan(ctx context.Context, store memory.Store, sid, cwd string, payload TechPlanPayload) error {
	if store == nil || sid == "" {
		return fmt.Errorf("write tech plan: nil store or empty sid")
	}
	body, _ := json.Marshal(payload)
	now := time.Now().UTC()
	return store.Upsert(ctx, memory.Entry{
		ID:        IDTechPlan(sid),
		Type:      memory.TypeProject,
		Domain:    memory.DomainTask,
		Title:     "tech plan",
		Content:   string(body),
		Source:    memory.SourceSession,
		SessionID: sid,
		CWD:       cwd,
		Metadata: commonMeta(StageTechPlan, memory.Metadata{
			"approved": payload.Approved,
			"decision": payload.Decision,
		}),
		CreatedAt: now,
		UpdatedAt: now,
	})
}

// WritePermissions persists Stage 5.
func WritePermissions(ctx context.Context, store memory.Store, sid, cwd string, p PermissionsPayload) error {
	if store == nil || sid == "" {
		return fmt.Errorf("write permissions: nil store or empty sid")
	}
	body, _ := json.Marshal(p)
	now := time.Now().UTC()
	return store.Upsert(ctx, memory.Entry{
		ID:        IDPermissions(sid),
		Type:      memory.TypeProject,
		Domain:    memory.DomainTask,
		Title:     "permissions",
		Content:   string(body),
		Source:    memory.SourceUser,
		SessionID: sid,
		CWD:       cwd,
		Metadata: commonMeta(StagePermissions, memory.Metadata{
			"mode": p.Mode,
		}),
		CreatedAt: now,
		UpdatedAt: now,
	})
}

// WriteDecisionStyle persists Stage 6. Long-term: the user's collaboration
// preference outlives a single session.
func WriteDecisionStyle(ctx context.Context, store memory.Store, sid, cwd string, style DecisionStyle) error {
	if store == nil || sid == "" {
		return fmt.Errorf("write decision style: nil store or empty sid")
	}
	body, _ := json.Marshal(DecisionStylePayload{Style: style})
	now := time.Now().UTC()
	return store.Upsert(ctx, memory.Entry{
		ID:        IDDecisionStyle(sid),
		Type:      memory.TypeLongTerm,
		Domain:    memory.DomainDialogue,
		Title:     "decision style",
		Content:   string(body),
		Source:    memory.SourceUser,
		SessionID: sid,
		CWD:       cwd,
		Metadata: commonMeta(StageDecisionStyle, memory.Metadata{
			"style": string(style),
		}),
		CreatedAt: now,
		UpdatedAt: now,
	})
}

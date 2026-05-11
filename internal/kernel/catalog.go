package kernel

import (
	"context"
	"encoding/json"
	"strings"

	"mobilevc/internal/data"
)

// ── Skill catalog ──

// GetSkillCatalogSnapshot returns the skill catalog snapshot from the store.
func (k *Kernel) GetSkillCatalogSnapshot(ctx context.Context) (data.SkillCatalogSnapshot, error) {
	if k.Store == nil {
		return data.SkillCatalogSnapshot{}, nil
	}
	return k.Store.GetSkillCatalogSnapshot(ctx)
}

// SaveSkillCatalogSnapshot saves the skill catalog snapshot.
func (k *Kernel) SaveSkillCatalogSnapshot(ctx context.Context, snapshot data.SkillCatalogSnapshot) error {
	if k.Store == nil {
		return nil
	}
	return k.Store.SaveSkillCatalogSnapshot(ctx, snapshot)
}

// ── Memory catalog ──

// GetMemoryCatalogSnapshot returns the memory catalog snapshot.
func (k *Kernel) GetMemoryCatalogSnapshot(ctx context.Context) (data.MemoryCatalogSnapshot, error) {
	if k.Store == nil {
		return data.MemoryCatalogSnapshot{}, nil
	}
	return k.Store.GetMemoryCatalogSnapshot(ctx)
}

// SaveMemoryCatalogSnapshot saves the memory catalog snapshot.
func (k *Kernel) SaveMemoryCatalogSnapshot(ctx context.Context, snapshot data.MemoryCatalogSnapshot) error {
	if k.Store == nil {
		return nil
	}
	return k.Store.SaveMemoryCatalogSnapshot(ctx, snapshot)
}

// ── Permission rules ──

// GetPermissionRuleSnapshot returns the permission rule snapshot.
func (k *Kernel) GetPermissionRuleSnapshot(ctx context.Context) (data.PermissionRuleSnapshot, error) {
	if k.Store == nil {
		return data.PermissionRuleSnapshot{}, nil
	}
	return k.Store.GetPermissionRuleSnapshot(ctx)
}

// SavePermissionRuleSnapshot saves the permission rule snapshot.
func (k *Kernel) SavePermissionRuleSnapshot(ctx context.Context, snapshot data.PermissionRuleSnapshot) error {
	if k.Store == nil {
		return nil
	}
	return k.Store.SavePermissionRuleSnapshot(ctx, snapshot)
}

// ── Session context ──

// UpdateSessionContext updates the enabled skills/memory for a session.
func (k *Kernel) UpdateSessionContext(ctx context.Context, sessionID string, context data.SessionContext) error {
	if k.Store == nil {
		return nil
	}
	record, err := k.Store.GetSession(ctx, sessionID)
	if err != nil {
		return err
	}
	record.Projection.SessionContext = context
	record.Projection.SessionContextSet = true
	_, err = k.Store.SaveProjection(ctx, sessionID, record.Projection)
	return err
}

// ── Skill/memory sync result helpers ──

type SkillExecRequest struct {
	Name         string `json:"name"`
	Engine       string `json:"engine,omitempty"`
	CWD          string `json:"cwd,omitempty"`
	Target       string `json:"target,omitempty"`
	TargetType   string `json:"targetType,omitempty"`
	TargetPath   string `json:"targetPath,omitempty"`
	TargetDiff   string `json:"targetDiff,omitempty"`
	TargetTitle  string `json:"targetTitle,omitempty"`
	ResultView   string `json:"resultView,omitempty"`
	ContextID    string `json:"contextId,omitempty"`
	ContextTitle string `json:"contextTitle,omitempty"`
	TargetText   string `json:"targetText,omitempty"`
	TargetStack  string `json:"targetStack,omitempty"`
}

func ParseSkillExecRequest(payload []byte) (SkillExecRequest, error) {
	var req SkillExecRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return SkillExecRequest{}, err
	}
	req.Name = strings.TrimSpace(req.Name)
	return req, nil
}

// ── Slash command helpers ──

type SlashCommandRequest struct {
	Command        string `json:"command"`
	CWD            string `json:"cwd,omitempty"`
	Engine         string `json:"engine,omitempty"`
	PermissionMode string `json:"permissionMode,omitempty"`
	TargetType     string `json:"targetType,omitempty"`
	TargetPath     string `json:"targetPath,omitempty"`
	TargetDiff     string `json:"targetDiff,omitempty"`
	TargetTitle    string `json:"targetTitle,omitempty"`
	ContextID      string `json:"contextId,omitempty"`
	ContextTitle   string `json:"contextTitle,omitempty"`
	TargetText     string `json:"targetText,omitempty"`
	TargetStack    string `json:"targetStack,omitempty"`
}

func ParseSlashCommandRequest(payload []byte) (SlashCommandRequest, error) {
	var req SlashCommandRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return SlashCommandRequest{}, err
	}
	return req, nil
}

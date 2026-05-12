package kernel

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"mobilevc/data"
	"mobilevc/engine"
	"mobilevc/protocol"
	"mobilevc/session"
)

// PermissionDecisionRequest wraps the permission_decision action.
type PermissionDecisionRequest struct {
	Decision            string `json:"decision"`
	Scope               string `json:"scope,omitempty"`
	PermissionMode      string `json:"permissionMode,omitempty"`
	PermissionRequestID string `json:"permissionRequestId,omitempty"`
	ResumeSessionID     string `json:"resumeSessionId,omitempty"`
	TargetPath          string `json:"targetPath,omitempty"`
	ContextID           string `json:"contextId,omitempty"`
	ContextTitle        string `json:"contextTitle,omitempty"`
	PromptMessage       string `json:"promptMessage,omitempty"`
	FallbackCommand     string `json:"command,omitempty"`
	FallbackCWD         string `json:"cwd,omitempty"`
	FallbackEngine      string `json:"engine,omitempty"`
	FallbackTarget      string `json:"target,omitempty"`
	FallbackTargetType  string `json:"targetType,omitempty"`
}

// ReviewDecisionRequest wraps review_decision.
type ReviewDecisionRequest struct {
	Decision       string `json:"decision"`
	ExecutionID    string `json:"executionId,omitempty"`
	GroupID        string `json:"groupId,omitempty"`
	GroupTitle     string `json:"groupTitle,omitempty"`
	ContextID      string `json:"contextId,omitempty"`
	ContextTitle   string `json:"contextTitle,omitempty"`
	TargetPath     string `json:"targetPath,omitempty"`
	PermissionMode string `json:"permissionMode,omitempty"`
	IsReviewOnly   bool   `json:"is_review_only,omitempty"`
}

// PlanDecisionRequest wraps plan_decision.
type PlanDecisionRequest struct {
	Decision        string `json:"decision"`
	ExecutionID     string `json:"executionId,omitempty"`
	GroupID         string `json:"groupId,omitempty"`
	GroupTitle      string `json:"groupTitle,omitempty"`
	ContextID       string `json:"contextId,omitempty"`
	ContextTitle    string `json:"contextTitle,omitempty"`
	PromptMessage   string `json:"promptMessage,omitempty"`
	PermissionMode  string `json:"permissionMode,omitempty"`
	ResumeSessionID string `json:"resumeSessionId,omitempty"`
	Command         string `json:"command,omitempty"`
	CWD             string `json:"cwd,omitempty"`
	Engine          string `json:"engine,omitempty"`
	Target          string `json:"target,omitempty"`
	TargetType      string `json:"targetType,omitempty"`
	TargetPath      string `json:"targetPath,omitempty"`
	TargetText      string `json:"targetText,omitempty"`
}

// SendPermissionDecision sends a permission decision to the active runner.
func (k *Kernel) SendPermissionDecision(ctx context.Context, conn *ConnectionState, decision string, meta protocol.RuntimeMeta, sink EventSink) error {
	if conn.RuntimeSvc == nil {
		return session.ErrNoActiveRunner
	}
	return conn.RuntimeSvc.SendPermissionDecision(ctx, conn.SelectedSessionID, decision, meta, sink)
}

// CurrentPermissionRequestID returns the active runner's pending permission request ID.
func (k *Kernel) CurrentPermissionRequestID(conn *ConnectionState) string {
	if conn.RuntimeSvc == nil {
		return ""
	}
	return conn.RuntimeSvc.CurrentPermissionRequestID(conn.SelectedSessionID)
}

// UpdatePermissionMode updates the permission mode on the service and active runner.
func (k *Kernel) UpdatePermissionMode(conn *ConnectionState, mode string) {
	if conn.RuntimeSvc != nil {
		conn.RuntimeSvc.UpdatePermissionMode(mode)
	}
}

// ReviewDecision sends a review decision (accept/revert/revise) to the runner.
func (k *Kernel) ReviewDecision(ctx context.Context, conn *ConnectionState, req session.ReviewDecisionRequest, sink EventSink) error {
	if conn.RuntimeSvc == nil {
		return session.ErrNoActiveRunner
	}
	return conn.RuntimeSvc.ReviewDecision(ctx, conn.SelectedSessionID, req, sink)
}

// PlanDecision sends a plan approval/rejection decision.
func (k *Kernel) PlanDecision(ctx context.Context, conn *ConnectionState, req session.PlanDecisionRequest, sink EventSink) error {
	if conn.RuntimeSvc == nil {
		return session.ErrNoActiveRunner
	}
	return conn.RuntimeSvc.PlanDecision(ctx, conn.SelectedSessionID, req, sink)
}

// ── Permission rule helpers ──

// ExecutePermissionDecision executes a permission decision, optionally saving a rule.
func (k *Kernel) ExecutePermissionDecision(
	ctx context.Context,
	conn *ConnectionState,
	permissionEvent protocol.PermissionDecisionRequestEvent,
	projection data.ProjectionSnapshot,
	controller session.ControllerSnapshot,
	sink EventSink,
) error {
	plan, err := session.BuildPermissionDecisionPlan(permissionEvent, projection, controller)
	if err != nil {
		return err
	}

	if plan.Meta.PermissionMode != "" {
		k.UpdatePermissionMode(conn, plan.Meta.PermissionMode)
	}

	switch plan.Action {
	case session.PermissionDecisionActionDirect:
		if err := k.SendPermissionDecision(ctx, conn, plan.Decision, plan.Meta, sink); err == nil {
			return nil
		} else if errors.Is(err, engine.ErrNoPendingControlRequest) {
			return session.ErrPermissionRequestExpired
		} else {
			return err
		}

	case session.PermissionDecisionActionDenyThenInput:
		if plan.Meta.PermissionRequestID != "" {
			if err := k.SendPermissionDecision(ctx, conn, plan.Decision, plan.Meta, sink); err == nil {
				return nil
			} else if !errors.Is(err, engine.ErrNoPendingControlRequest) && !errors.Is(err, engine.ErrInputNotSupported) {
				return err
			}
		}
		return conn.RuntimeSvc.SendInput(ctx, conn.SelectedSessionID, session.InputRequest{
			Data:        plan.Prompt,
			RuntimeMeta: plan.Meta,
		}, sink)

	case session.PermissionDecisionActionAutoThenDirect:
		k.UpdatePermissionMode(conn, "auto")
		if err := k.SendPermissionDecision(ctx, conn, plan.Decision, plan.Meta, sink); err == nil {
			return nil
		} else if errors.Is(err, engine.ErrNoPendingControlRequest) {
			return session.ErrPermissionRequestExpired
		} else {
			return err
		}

	default:
		return k.SendPermissionDecision(ctx, conn, plan.Decision, plan.Meta, sink)
	}
}

// ParsePermissionDecisionRequest parses raw JSON into permission decision event.
func ParsePermissionDecisionRequest(payload []byte) (protocol.PermissionDecisionRequestEvent, error) {
	var req protocol.PermissionDecisionRequestEvent
	if err := json.Unmarshal(payload, &req); err != nil {
		return protocol.PermissionDecisionRequestEvent{}, err
	}
	return req, nil
}

// ParseReviewDecisionRequest parses raw JSON into review decision.
func ParseReviewDecisionRequest(payload []byte) (ReviewDecisionRequest, string, error) {
	var req ReviewDecisionRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return ReviewDecisionRequest{}, "", err
	}
	decision := strings.TrimSpace(strings.ToLower(req.Decision))
	return req, decision, nil
}

// ParsePlanDecisionRequest parses raw JSON into plan decision.
func ParsePlanDecisionRequest(payload []byte) (PlanDecisionRequest, string, error) {
	var req PlanDecisionRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return PlanDecisionRequest{}, "", err
	}
	decision := strings.TrimSpace(req.Decision)
	return req, decision, nil
}

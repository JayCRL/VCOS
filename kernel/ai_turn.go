package kernel

import (
	"context"
	"encoding/json"
	"strings"

	"mobilevc/engine"
	"mobilevc/support/logx"
	"mobilevc/protocol"
	"mobilevc/session"
)

// AITurnRequest wraps the ai_turn action payload.
type AITurnRequest struct {
	Engine         string `json:"engine,omitempty"`
	Data           string `json:"data,omitempty"`
	CWD            string `json:"cwd,omitempty"`
	PermissionMode string `json:"permissionMode,omitempty"`
	Source         string `json:"source,omitempty"`
	SkillName      string `json:"skillName,omitempty"`
	Target         string `json:"target,omitempty"`
	TargetType     string `json:"targetType,omitempty"`
	TargetPath     string `json:"targetPath,omitempty"`
	ContextID      string `json:"contextId,omitempty"`
	ContextTitle   string `json:"contextTitle,omitempty"`
	TargetStack    string `json:"targetStack,omitempty"`
	Model          string `json:"model,omitempty"`
	ReasoningEffort string `json:"reasoningEffort,omitempty"`
}

// ExecRequest wraps the exec action payload.
type ExecRequest struct {
	Command        string `json:"cmd"`
	CWD            string `json:"cwd,omitempty"`
	Mode           string `json:"mode,omitempty"`
	PermissionMode string `json:"permissionMode,omitempty"`
	InputData      string `json:"inputData,omitempty"`
	Source         string `json:"source,omitempty"`
	SkillName      string `json:"skillName,omitempty"`
	Target         string `json:"target,omitempty"`
	TargetType     string `json:"targetType,omitempty"`
	TargetPath     string `json:"targetPath,omitempty"`
	ContextID      string `json:"contextId,omitempty"`
	ContextTitle   string `json:"contextTitle,omitempty"`
	Model          string `json:"model,omitempty"`
	ReasoningEffort string `json:"reasoningEffort,omitempty"`
	Engine         string `json:"engine,omitempty"`
}

// InputRequest wraps the input action payload.
type InputRequest struct {
	Data           string `json:"data"`
	PermissionMode string `json:"permissionMode,omitempty"`
}

// BuildExecParams constructs session.ExecuteRequest and session.InputRequest from raw JSON payloads.
func (k *Kernel) BuildExecParams(payload []byte) (session.ExecuteRequest, session.InputRequest, error) {
	var req ExecRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return session.ExecuteRequest{}, session.InputRequest{}, err
	}

	mode, err := parseMode(req.Mode)
	if err != nil {
		return session.ExecuteRequest{}, session.InputRequest{}, err
	}

	command := strings.TrimSpace(req.Command)
	if command == "" {
		command = defaultAICommandFromEngine(req.SkillName, "claude")
	}
	command = applyAICommandPreferences(command, "", req.Model, req.ReasoningEffort)

	execReq := session.ExecuteRequest{
		Command:        command,
		CWD:            strings.TrimSpace(req.CWD),
		Mode:           mode,
		PermissionMode: normalizePermissionModeForClaude(req.PermissionMode),
		InitialInput:   req.InputData,
		RuntimeMeta: protocol.RuntimeMeta{
			Source:       firstNonEmptyString(req.Source, "ai_turn"),
			SkillName:    strings.TrimSpace(req.SkillName),
			Target:       strings.TrimSpace(req.Target),
			TargetType:   strings.TrimSpace(req.TargetType),
			TargetPath:   strings.TrimSpace(req.TargetPath),
			ContextID:    strings.TrimSpace(req.ContextID),
			ContextTitle: strings.TrimSpace(req.ContextTitle),
			Command:      command,
			CWD:          strings.TrimSpace(req.CWD),
			Engine:       strings.TrimSpace(req.Engine),
			Model:        strings.TrimSpace(req.Model),
			ReasoningEffort: strings.TrimSpace(req.ReasoningEffort),
		},
	}

	inputReq := session.InputRequest{
		Data: req.InputData,
	}

	return execReq, inputReq, nil
}

// BuildAITurnParams constructs session.ExecuteRequest and session.InputRequest for ai_turn.
func (k *Kernel) BuildAITurnParams(payload []byte) (session.ExecuteRequest, session.InputRequest, error) {
	var req AITurnRequest
	if err := json.Unmarshal(payload, &req); err != nil {
		return session.ExecuteRequest{}, session.InputRequest{}, err
	}

	command := defaultAICommandFromEngine(req.Engine, "claude")
	command = applyAICommandPreferences(command, req.Engine, req.Model, req.ReasoningEffort)

	execReq := session.ExecuteRequest{
		Command:        command,
		CWD:            strings.TrimSpace(req.CWD),
		Mode:           engine.ModePTY,
		PermissionMode: normalizePermissionModeForClaude(req.PermissionMode),
		RuntimeMeta: protocol.RuntimeMeta{
			Source:       firstNonEmptyString(req.Source, "ai_turn"),
			SkillName:    strings.TrimSpace(req.SkillName),
			Target:       strings.TrimSpace(req.Target),
			TargetType:   strings.TrimSpace(req.TargetType),
			TargetPath:   strings.TrimSpace(req.TargetPath),
			ContextID:    strings.TrimSpace(req.ContextID),
			ContextTitle: strings.TrimSpace(req.ContextTitle),
			Command:      command,
			CWD:          strings.TrimSpace(req.CWD),
			Engine:       strings.TrimSpace(req.Engine),
			Model:        strings.TrimSpace(req.Model),
			ReasoningEffort: strings.TrimSpace(req.ReasoningEffort),
		},
	}

	inputReq := session.InputRequest{
		Data: req.Data,
	}

	return execReq, inputReq, nil
}

// AcknowledgeClientAction checks and acknowledges a client action, emits ACK event.
func (k *Kernel) AcknowledgeClientAction(conn *ConnectionState, sessionID, action, clientActionID string, sink EventSink) bool {
	clientActionID = strings.TrimSpace(clientActionID)
	if clientActionID == "" {
		return true
	}

	targetSessionID := strings.TrimSpace(sessionID)
	if targetSessionID == "" {
		targetSessionID = strings.TrimSpace(conn.SelectedSessionID)
	}

	sessionRuntime := k.Registry.Ensure(targetSessionID)
	accepted := true
	if sessionRuntime != nil {
		accepted = sessionRuntime.MarkClientAction(clientActionID)
	}

	sink(protocol.NewClientActionAckEvent(targetSessionID, action, clientActionID, "accepted", !accepted))
	return accepted
}

// ExecuteAICommand starts an AI CLI runner for the given request.
func (k *Kernel) ExecuteAICommand(ctx context.Context, conn *ConnectionState, req session.ExecuteRequest, sink EventSink) error {
	svc := conn.RuntimeSvc
	if svc == nil {
		svc = k.NewDetachedService()
		conn.RuntimeSvc = svc
	}

	sessionID := strings.TrimSpace(conn.SelectedSessionID)
	if sessionID == "" {
		sessionID = "default"
	}

	logx.Info("kernel", "execute AI command: sessionID=%s command=%s mode=%s", sessionID, req.Command, req.Mode)
	return svc.Execute(ctx, sessionID, req, sink)
}

// SendInput sends text input to the active runner.
func (k *Kernel) SendInput(ctx context.Context, conn *ConnectionState, req session.InputRequest, sink EventSink) error {
	svc := conn.RuntimeSvc
	if svc == nil {
		return session.ErrNoActiveRunner
	}
	return svc.SendInput(ctx, conn.SelectedSessionID, req, sink)
}

// SendInputOrResume tries to send input, and if no runner is active, resumes the session first.
func (k *Kernel) SendInputOrResume(ctx context.Context, conn *ConnectionState, execReq session.ExecuteRequest, inputReq session.InputRequest, sink EventSink) error {
	if conn.RuntimeSvc == nil {
		return session.ErrNoActiveRunner
	}
	return conn.RuntimeSvc.SendInputOrResume(ctx, conn.SelectedSessionID, execReq, inputReq, sink)
}

// StopActive stops the currently running command.
func (k *Kernel) StopActive(conn *ConnectionState, sink EventSink) error {
	if conn.RuntimeSvc == nil {
		return session.ErrNoActiveRunner
	}
	return conn.RuntimeSvc.StopActive(conn.SelectedSessionID, sink)
}

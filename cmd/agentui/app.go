package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"

	"mobilevc/cognition/intake"
	"mobilevc/cognition/vibe"
	"mobilevc/data"
	"mobilevc/desktop/wizard"
	"mobilevc/engine"
	"mobilevc/evolution/feedback"
	"mobilevc/kernel"
	"mobilevc/protocol"
	"mobilevc/session"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the root binding object exposed to the Wails JS runtime. Every
// exported method becomes window.go.main.App.<Method> on the frontend.
type App struct {
	ctx    context.Context
	kernel *kernel.Kernel
	bridge *eventBridge

	mu       sync.Mutex
	services map[string]*session.Service
}

func NewApp(k *kernel.Kernel) *App {
	return &App{
		kernel:   k,
		bridge:   newEventBridge(),
		services: make(map[string]*session.Service),
	}
}

func (a *App) OnStartup(ctx context.Context) {
	a.ctx = ctx
	wailsRuntime.LogInfo(ctx, "agentui starting")
}

func (a *App) OnShutdown(ctx context.Context) {
	wailsRuntime.LogInfo(ctx, "agentui shutting down")
	a.bridge.DetachAll()
	a.mu.Lock()
	for _, svc := range a.services {
		svc.Cleanup()
	}
	a.services = nil
	a.mu.Unlock()
}

// getOrCreateService returns the cached session.Service for the given
// session ID, creating one on demand. One Service per session.
func (a *App) getOrCreateService(sid string) *session.Service {
	a.mu.Lock()
	defer a.mu.Unlock()
	if svc, ok := a.services[sid]; ok {
		return svc
	}
	svc := session.NewService(sid, session.Dependencies{
		NewExecRunner: a.kernel.NewExecRunner,
		NewPtyRunner:  a.kernel.NewPtyRunner,
	})
	svc.SetSink(busSinkFor(a.kernel.Bus, sid))
	if a.services == nil {
		a.services = make(map[string]*session.Service)
	}
	a.services[sid] = svc
	return svc
}

// ——— session lifecycle ———

// StartSession creates a new session and initializes its wizard cursor.
// Returns the session ID.
func (a *App) StartSession(name string) (string, error) {
	if name == "" {
		name = "AgentUI Session"
	}
	summary, err := a.kernel.Store.CreateSession(a.ctx, name)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}
	if err := wizard.SaveCursor(a.ctx, a.kernel.MemStore, summary.ID, wizard.CursorPayload{
		CurrentStage: wizard.StageUserIntent,
	}); err != nil {
		return "", fmt.Errorf("init cursor: %w", err)
	}
	return summary.ID, nil
}

// ListSessions returns all known sessions, most recent first.
func (a *App) ListSessions() ([]data.SessionSummary, error) {
	return a.kernel.Store.ListSessions(a.ctx)
}

// ——— wizard stages 1-6 ———

func (a *App) SubmitUserIntent(sid, text string) error {
	if sid == "" {
		return fmt.Errorf("sid is required")
	}
	cwd, _ := os.Getwd()
	if err := wizard.WriteUserIntent(a.ctx, a.kernel.MemStore, sid, cwd, text); err != nil {
		return err
	}
	return wizard.Advance(a.ctx, a.kernel.MemStore, sid, wizard.StageUserIntent, wizard.StageProjectIntent)
}

func (a *App) LoadWizardState(sid string) (wizard.Snapshot, error) {
	if sid == "" {
		return wizard.Snapshot{}, fmt.Errorf("sid is required")
	}
	return wizard.LoadSnapshot(a.ctx, a.kernel.MemStore, sid)
}

func (a *App) SubmitProjectIntent(sid, prompt, userNote string) (intake.CognitiveProfile, error) {
	if sid == "" {
		return intake.CognitiveProfile{}, fmt.Errorf("sid is required")
	}
	cwd, _ := os.Getwd()
	profile, err := a.kernel.Intake.Run(a.ctx, sid, cwd, prompt)
	if err != nil {
		return profile, fmt.Errorf("intake: %w", err)
	}
	if err := wizard.WriteProjectIntent(a.ctx, a.kernel.MemStore, sid, cwd, prompt, userNote, profile); err != nil {
		return profile, err
	}
	if err := wizard.Advance(a.ctx, a.kernel.MemStore, sid, wizard.StageProjectIntent, wizard.StageUISpec); err != nil {
		return profile, err
	}
	return profile, nil
}

func (a *App) SubmitUISpec(sid string, components []wizard.UIComponentPayload, mappingPrompt string) error {
	if sid == "" {
		return fmt.Errorf("sid is required")
	}
	cwd, _ := os.Getwd()
	ids := make([]string, 0, len(components))
	for _, c := range components {
		if err := wizard.WriteUIComponent(a.ctx, a.kernel.MemStore, sid, cwd, c); err != nil {
			return err
		}
		ids = append(ids, c.ID)
	}
	if err := wizard.WriteUIPrompt(a.ctx, a.kernel.MemStore, sid, cwd, mappingPrompt, ids); err != nil {
		return err
	}
	return wizard.Advance(a.ctx, a.kernel.MemStore, sid, wizard.StageUISpec, wizard.StageTechPlan)
}

func (a *App) SubmitTechPlan(sid, draft, decision, adjusted string) error {
	if sid == "" {
		return fmt.Errorf("sid is required")
	}
	cwd, _ := os.Getwd()
	approved := decision == "accept" || decision == "adjust"
	payload := wizard.TechPlanPayload{
		Draft:        draft,
		Decision:     decision,
		AdjustedText: adjusted,
		Approved:     approved,
	}
	if err := wizard.WriteTechPlan(a.ctx, a.kernel.MemStore, sid, cwd, payload); err != nil {
		return err
	}
	return wizard.Advance(a.ctx, a.kernel.MemStore, sid, wizard.StageTechPlan, wizard.StagePermissions)
}

func (a *App) SubmitPermissions(sid string, p wizard.PermissionsPayload) error {
	if sid == "" {
		return fmt.Errorf("sid is required")
	}
	cwd, _ := os.Getwd()
	if err := wizard.WritePermissions(a.ctx, a.kernel.MemStore, sid, cwd, p); err != nil {
		return err
	}
	return wizard.Advance(a.ctx, a.kernel.MemStore, sid, wizard.StagePermissions, wizard.StageDecisionStyle)
}

func (a *App) SubmitDecisionStyle(sid string, style string) error {
	if sid == "" {
		return fmt.Errorf("sid is required")
	}
	cwd, _ := os.Getwd()
	ds := wizard.DecisionStyle(style)
	switch ds {
	case wizard.StyleStepByStep, wizard.StyleHybrid, wizard.StyleAutonomous:
	default:
		return fmt.Errorf("unknown decision style: %s", style)
	}
	if err := wizard.WriteDecisionStyle(a.ctx, a.kernel.MemStore, sid, cwd, ds); err != nil {
		return err
	}
	switch ds {
	case wizard.StyleStepByStep:
		_ = a.kernel.Vibe.UpdateProactivity(a.ctx, sid, vibe.ProactivityPassive)
	case wizard.StyleHybrid:
		_ = a.kernel.Vibe.UpdateProactivity(a.ctx, sid, vibe.ProactivityBalanced)
	case wizard.StyleAutonomous:
		_ = a.kernel.Vibe.UpdateProactivity(a.ctx, sid, vibe.ProactivityActive)
	}
	return wizard.Advance(a.ctx, a.kernel.MemStore, sid, wizard.StageDecisionStyle, wizard.StageExecuting)
}

// ——— Stage 7: execution ———

// StartExecution composes a prompt from the wizard snapshot and launches
// `claude` via session.Execute. Events flow through the kernel bus to the
// frontend channel "agent:<sid>". Non-blocking — runs in a goroutine.
func (a *App) StartExecution(sid string) error {
	if sid == "" {
		return fmt.Errorf("sid is required")
	}
	cwd, _ := os.Getwd()
	snap, err := wizard.LoadSnapshot(a.ctx, a.kernel.MemStore, sid)
	if err != nil {
		return fmt.Errorf("load snapshot: %w", err)
	}
	prompt := composeExecutionPrompt(snap)

	svc := a.getOrCreateService(sid)
	a.bridge.Attach(a.ctx, a.kernel.Bus, sid)

	go func() {
		req := session.ExecuteRequest{
			Command:      "claude",
			CWD:          cwd,
			Mode:         engine.ModePTY,
			InitialInput: prompt,
		}
		if err := svc.Execute(context.Background(), sid, req, busSinkFor(a.kernel.Bus, sid)); err != nil {
			wailsRuntime.LogErrorf(a.ctx, "execute: %v", err)
		}
	}()
	return nil
}

// SendChat sends free-text input to the running session. If no runtime is
// active for this session, SendInputOrResume will start one.
func (a *App) SendChat(sid, text string) error {
	if sid == "" {
		return fmt.Errorf("sid is required")
	}
	cwd, _ := os.Getwd()
	svc := a.getOrCreateService(sid)
	svc.RecordUserInput(text)
	a.bridge.Attach(a.ctx, a.kernel.Bus, sid)

	go func() {
		err := svc.SendInputOrResume(context.Background(), sid,
			session.ExecuteRequest{
				Command: "claude",
				CWD:     cwd,
				Mode:    engine.ModePTY,
			},
			session.InputRequest{Data: text + "\n"},
			busSinkFor(a.kernel.Bus, sid),
		)
		if err != nil {
			wailsRuntime.LogErrorf(a.ctx, "chat: %v", err)
		}
	}()
	return nil
}

// ApprovePermission resolves a pending permission request (e.g. tool-use
// confirmation) raised by the agent during execution.
func (a *App) ApprovePermission(sid, decision string) error {
	svc := a.getOrCreateService(sid)
	return svc.SendPermissionDecision(a.ctx, sid, decision, protocol.RuntimeMeta{}, busSinkFor(a.kernel.Bus, sid))
}

// ApproveReview resolves a pending review prompt.
func (a *App) ApproveReview(sid, decision string, reviewOnly bool) error {
	svc := a.getOrCreateService(sid)
	return svc.ReviewDecision(a.ctx, sid, session.ReviewDecisionRequest{
		Decision:     decision,
		IsReviewOnly: reviewOnly,
	}, busSinkFor(a.kernel.Bus, sid))
}

// ApprovePlan resolves a pending plan-mode decision.
func (a *App) ApprovePlan(sid, decision string) error {
	svc := a.getOrCreateService(sid)
	return svc.PlanDecision(a.ctx, sid, session.PlanDecisionRequest{
		Decision: decision,
	}, busSinkFor(a.kernel.Bus, sid))
}

// ——— evolve feedback (auxiliary; surfaces evolve's learning-derived
// suggestions in the GUI sidebar) ———

func (a *App) ListPendingFeedback() []feedback.Suggestion {
	return a.kernel.Feedback.Pending()
}

func (a *App) DecideFeedback(suggestionID, decision, adjusted string) error {
	_, err := a.kernel.Feedback.Decide(a.ctx, suggestionID, feedback.Decision(decision), adjusted)
	return err
}

// ——— helpers ———

// composeExecutionPrompt synthesizes the wizard products into a single
// prompt that the AI agent receives as its first message.
func composeExecutionPrompt(s wizard.Snapshot) string {
	var sb strings.Builder
	sb.WriteString("以下是用户在 AgentOS 桌面向导中提交的完整任务上下文。请严格按这些约定开始执行。\n\n")

	if s.UserIntent != nil && strings.TrimSpace(s.UserIntent.Text) != "" {
		sb.WriteString("【整体意图】\n")
		sb.WriteString(s.UserIntent.Text)
		sb.WriteString("\n\n")
	}
	if s.ProjectIntent != nil {
		sb.WriteString("【项目意图】\n")
		sb.WriteString("prompt: " + s.ProjectIntent.Prompt + "\n")
		if s.ProjectIntent.UserNote != "" {
			sb.WriteString("notes:  " + s.ProjectIntent.UserNote + "\n")
		}
		h := s.ProjectIntent.Profile.ProjectHint
		sb.WriteString(fmt.Sprintf("detected: lang=%s framework=%s build=%s\n\n",
			defaultStr(h.Language, "?"),
			defaultStr(h.Framework, "-"),
			defaultStr(h.BuildTool, "-")))
	}
	if s.UIPrompt != nil && strings.TrimSpace(s.UIPrompt.Prompt) != "" {
		sb.WriteString("【UI 元素映射】\n")
		sb.WriteString(s.UIPrompt.Prompt)
		sb.WriteString("\n\n")
	}
	if s.TechPlan != nil && s.TechPlan.Approved {
		text := s.TechPlan.Draft
		if strings.TrimSpace(s.TechPlan.AdjustedText) != "" {
			text = s.TechPlan.AdjustedText
		}
		sb.WriteString("【已批准的技术方案】\n")
		sb.WriteString(text)
		sb.WriteString("\n\n")
	}
	if s.Permissions != nil {
		sb.WriteString(fmt.Sprintf("【权限】allow=%v deny=%v mode=%s\n\n",
			s.Permissions.Allow, s.Permissions.Deny, defaultStr(s.Permissions.Mode, "(default)")))
	}
	if s.DecisionStyle != nil {
		sb.WriteString("【决策风格】" + string(s.DecisionStyle.Style) + "\n\n")
	}
	sb.WriteString("请开始执行。遇到决策点按上述决策风格处理:\n")
	sb.WriteString("  - step-by-step:每一步完成都暂停等用户在 GUI 确认\n")
	sb.WriteString("  - hybrid:仅在写文件/shell/依赖变更等高风险操作前请求确认\n")
	sb.WriteString("  - autonomous:全程自动,只在任务完成或失败时报告\n")
	return sb.String()
}

func defaultStr(s, fallback string) string {
	if strings.TrimSpace(s) == "" {
		return fallback
	}
	return s
}

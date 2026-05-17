package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"mobilevc/cognition/intake"
	"mobilevc/cognition/vibe"
	"mobilevc/data"
	"mobilevc/desktop/draft"
	"mobilevc/desktop/scan"
	"mobilevc/desktop/wizard"
	"mobilevc/engine"
	"mobilevc/evolution/feedback"
	"mobilevc/kernel"
	"mobilevc/protocol"
	"mobilevc/session"

	"mobilevc/cmd/agentui/llm"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// App is the root binding object exposed to the Wails JS runtime. Every
// exported method becomes window.go.main.App.<Method> on the frontend.
type App struct {
	ctx    context.Context
	kernel *kernel.Kernel
	bridge *eventBridge

	mu          sync.Mutex
	services    map[string]*session.Service
	projectDirs map[string]string // sid -> project root directory
}

// projectCWD returns the project root for a session, falling back to the
// process working directory when no explicit path was set.
func (a *App) projectCWD(sid string) string {
	a.mu.Lock()
	defer a.mu.Unlock()
	if d, ok := a.projectDirs[sid]; ok && d != "" {
		return d
	}
	d, _ := os.Getwd()
	return d
}

// SetSessionCWD binds a project root directory to a session.
func (a *App) SetSessionCWD(sid, path string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.projectDirs == nil {
		a.projectDirs = map[string]string{}
	}
	a.projectDirs[sid] = path
}

// PickFolder opens the native directory picker and returns the chosen path.
func (a *App) PickFolder(dft string) (string, error) {
	return wailsRuntime.OpenDirectoryDialog(a.ctx, wailsRuntime.OpenDialogOptions{
		Title:            "选择项目所在文件夹",
		DefaultDirectory: dft,
	})
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
	enrichPATH(ctx)
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
	cwd := a.projectCWD(sid)
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

func (a *App) SubmitProjectIntent(sid, prompt, userNote string, semantic *scan.Semantic) (intake.CognitiveProfile, error) {
	if sid == "" {
		return intake.CognitiveProfile{}, fmt.Errorf("sid is required")
	}
	cwd := a.projectCWD(sid)
	profile, err := a.kernel.Intake.Run(a.ctx, sid, cwd, prompt)
	if err != nil {
		return profile, fmt.Errorf("intake: %w", err)
	}
	if err := wizard.WriteProjectIntent(a.ctx, a.kernel.MemStore, sid, cwd, prompt, userNote, profile, semantic); err != nil {
		return profile, err
	}
	if err := wizard.Advance(a.ctx, a.kernel.MemStore, sid, wizard.StageProjectIntent, wizard.StageUISpec); err != nil {
		return profile, err
	}
	return profile, nil
}

// ScanPhysical returns the L1 file tree rooted at cwd (defaults to the GUI's
// working directory if empty). Fast (< 2s for typical projects); fully
// synchronous.
func (a *App) ScanPhysical(sid, cwd string) (*scan.TreeNode, error) {
	if strings.TrimSpace(cwd) == "" {
		cwd = a.projectCWD(sid)
	}
	if strings.TrimSpace(cwd) == "" {
		cwd, _ = os.Getwd()
	}
	return scan.WalkPhysical(cwd)
}

// ScanSemantic invokes `claude --print` to produce the L3 semantic summary
// for the given working directory. Non-blocking — runs in a goroutine and
// pushes lifecycle events on the frontend channel "scan:<sid>":
//
//	{phase: "thinking"}     immediately
//	{phase: "done",     semantic: Semantic}   on success
//	{phase: "error",    message: string, raw: string}  on failure
func (a *App) ScanSemantic(sid, cwd string) error {
	if sid == "" {
		return fmt.Errorf("sid is required")
	}
	if strings.TrimSpace(cwd) == "" {
		cwd = a.projectCWD(sid)
	}
	channel := "scan:" + sid
	wailsRuntime.EventsEmit(a.ctx, channel, map[string]any{"phase": "thinking"})

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
		defer cancel()
		sem, raw, err := scan.RunSemantic(ctx, cwd)
		if err != nil {
			wailsRuntime.LogErrorf(a.ctx, "scan semantic: %v", err)
			wailsRuntime.EventsEmit(a.ctx, channel, map[string]any{
				"phase":   "error",
				"message": err.Error(),
				"raw":     raw,
			})
			return
		}
		wailsRuntime.EventsEmit(a.ctx, channel, map[string]any{
			"phase":    "done",
			"semantic": sem,
		})
	}()
	return nil
}

// GetAPIKeyStatus describes whether the Anthropic API key is currently
// configured (env var or local config file). The GUI uses this to decide
// whether to show "configure key" UI before invoking AdjustUIWithAI.
func (a *App) GetAPIKeyStatus() llm.KeyStatus {
	return llm.Status()
}

// SetAPIConfig persists Anthropic API key + optional baseURL into
// ~/.mobilevc/agentui-config.json (0600). Pass empty strings to clear.
func (a *App) SetAPIConfig(key, baseURL string) error {
	return llm.SaveConfig(key, baseURL)
}

// AdjustUIWithAI calls Anthropic with a tool_use forcing apply_patches output,
// then returns the parsed patches. The frontend applies them locally without
// a follow-up round-trip.
func (a *App) AdjustUIWithAI(prompt, accent, templateName string, components []llm.Component) ([]llm.Patch, error) {
	key, src := llm.LoadKey()
	if key == "" || src == llm.KeyAbsent {
		return nil, fmt.Errorf("anthropic api key not configured")
	}
	baseURL := llm.LoadBaseURL()
	client := llm.NewClient(key, baseURL)
	ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
	defer cancel()
	return client.Adjust(ctx, llm.AdjustRequest{
		UserPrompt:   prompt,
		Components:   components,
		Accent:       accent,
		TemplateName: templateName,
	})
}

// SuggestUI picks a template + accent + font for the user based on Stage 1/2
// products. Called by the GUI when entering Stage 3 with no prior selection.
func (a *App) SuggestUI(sid string) (*llm.UISuggestion, error) {
	if sid == "" {
		return nil, fmt.Errorf("sid is required")
	}
	client, err := a.llmClient()
	if err != nil {
		return nil, err
	}
	snap, err := wizard.LoadSnapshot(a.ctx, a.kernel.MemStore, sid)
	if err != nil {
		return nil, fmt.Errorf("load snapshot: %w", err)
	}
	req := llm.SuggestUIRequest{}
	if snap.UserIntent != nil {
		req.UserIntent = snap.UserIntent.Text
	}
	if snap.ProjectIntent != nil {
		req.ProjectPrompt = snap.ProjectIntent.Prompt
		req.UserNote = snap.ProjectIntent.UserNote
		if sem := snap.ProjectIntent.Semantic; sem != nil {
			req.Language = sem.Language
			req.Summary = sem.Summary
			for _, m := range sem.Modules {
				req.ModuleNames = append(req.ModuleNames, m.Name)
			}
		}
	}
	ctx, cancel := context.WithTimeout(a.ctx, 60*time.Second)
	defer cancel()
	return client.SuggestUI(ctx, req)
}

// SuggestInteractionFlows generates 5-8 starter flows based on the user's
// intent and the UI components they've already locked in.
func (a *App) SuggestInteractionFlows(sid string) (*llm.FlowDraftSet, error) {
	if sid == "" {
		return nil, fmt.Errorf("sid is required")
	}
	client, err := a.llmClient()
	if err != nil {
		return nil, err
	}
	snap, err := wizard.LoadSnapshot(a.ctx, a.kernel.MemStore, sid)
	if err != nil {
		return nil, fmt.Errorf("load snapshot: %w", err)
	}
	req := llm.SuggestFlowsRequest{}
	if snap.UserIntent != nil {
		req.UserIntent = snap.UserIntent.Text
	}
	for _, c := range snap.UIComponents {
		req.Components = append(req.Components, llm.Component{
			ID:          c.ID,
			Name:        c.Name,
			Kind:        c.Kind,
			Description: c.Description,
		})
		if tn, ok := c.Props["templateName"].(string); ok && tn != "" && req.TemplateName == "" {
			req.TemplateName = tn
		}
		if tn, ok := c.Props["template"].(string); ok && tn != "" && req.TemplateName == "" {
			req.TemplateName = tn
		}
	}
	ctx, cancel := context.WithTimeout(a.ctx, 90*time.Second)
	defer cancel()
	return client.SuggestFlows(ctx, req)
}

// SuggestArchitecture proposes a starter Semantic for a greenfield project
// (the cwd has no readable files). The GUI calls this from Stage 2 when the
// physical tree is essentially empty.
func (a *App) SuggestArchitecture(sid string) (*scan.Semantic, error) {
	if sid == "" {
		return nil, fmt.Errorf("sid is required")
	}
	client, err := a.llmClient()
	if err != nil {
		return nil, err
	}
	snap, err := wizard.LoadSnapshot(a.ctx, a.kernel.MemStore, sid)
	if err != nil {
		return nil, fmt.Errorf("load snapshot: %w", err)
	}
	intent := ""
	if snap.UserIntent != nil {
		intent = snap.UserIntent.Text
	}
	if strings.TrimSpace(intent) == "" {
		return nil, fmt.Errorf("user intent is empty; cannot suggest architecture")
	}
	ctx, cancel := context.WithTimeout(a.ctx, 90*time.Second)
	defer cancel()
	arch, err := client.SuggestArchitecture(ctx, intent)
	if err != nil {
		return nil, err
	}
	return arch.ToSemantic(), nil
}

func (a *App) llmClient() (*llm.Client, error) {
	key, src := llm.LoadKey()
	if key == "" || src == llm.KeyAbsent {
		return nil, fmt.Errorf("anthropic api key not configured")
	}
	return llm.NewClient(key, llm.LoadBaseURL()), nil
}

func (a *App) SubmitUISpec(sid string, components []wizard.UIComponentPayload, mappingPrompt string) error {
	if sid == "" {
		return fmt.Errorf("sid is required")
	}
	cwd := a.projectCWD(sid)
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
	return wizard.Advance(a.ctx, a.kernel.MemStore, sid, wizard.StageUISpec, wizard.StageInteractionLogic)
}

// SubmitInteractionLogic persists Stage 3.5 — user-described interaction
// flows for the UI elements. Advances the wizard to the tech plan stage.
func (a *App) SubmitInteractionLogic(sid string, payload wizard.InteractionLogicPayload) error {
	if sid == "" {
		return fmt.Errorf("sid is required")
	}
	cwd := a.projectCWD(sid)
	if err := wizard.WriteInteractionLogic(a.ctx, a.kernel.MemStore, sid, cwd, payload); err != nil {
		return err
	}
	return wizard.Advance(a.ctx, a.kernel.MemStore, sid, wizard.StageInteractionLogic, wizard.StageTechPlan)
}

// DraftTechPlan kicks off async Stage 4 drafting. Loads the wizard snapshot,
// composes a context-aware prompt, then invokes `claude --print` and streams
// stdout bytes back through wailsRuntime.EventsEmit on channel "draft:<sid>":
//
//	{phase: "thinking"}              immediately
//	{phase: "chunk", text: string}   for each stdout chunk
//	{phase: "done", text: string}    on success (text is the full transcript)
//	{phase: "error", message: ...}   on failure
func (a *App) DraftTechPlan(sid string) error {
	if sid == "" {
		return fmt.Errorf("sid is required")
	}
	channel := "draft:" + sid
	snap, err := wizard.LoadSnapshot(a.ctx, a.kernel.MemStore, sid)
	if err != nil {
		return fmt.Errorf("load snapshot: %w", err)
	}
	prompt := draft.BuildPrompt(snap)
	cwd := a.projectCWD(sid)

	wailsRuntime.EventsEmit(a.ctx, channel, map[string]any{"phase": "thinking"})

	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
		defer cancel()
		text, err := draft.RunStream(ctx, cwd, prompt, func(chunk string) {
			wailsRuntime.EventsEmit(a.ctx, channel, map[string]any{
				"phase": "chunk",
				"text":  chunk,
			})
		})
		if err != nil {
			wailsRuntime.LogErrorf(a.ctx, "draft tech plan: %v", err)
			wailsRuntime.EventsEmit(a.ctx, channel, map[string]any{
				"phase":   "error",
				"message": err.Error(),
				"text":    text,
			})
			return
		}
		wailsRuntime.EventsEmit(a.ctx, channel, map[string]any{
			"phase": "done",
			"text":  text,
		})
	}()
	return nil
}

func (a *App) SubmitTechPlan(sid, draftText, decision, adjusted string) error {
	if sid == "" {
		return fmt.Errorf("sid is required")
	}
	cwd := a.projectCWD(sid)
	approved := decision == "accept" || decision == "adjust"
	payload := wizard.TechPlanPayload{
		Draft:        draftText,
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
	cwd := a.projectCWD(sid)
	if err := wizard.WritePermissions(a.ctx, a.kernel.MemStore, sid, cwd, p); err != nil {
		return err
	}
	return wizard.Advance(a.ctx, a.kernel.MemStore, sid, wizard.StagePermissions, wizard.StageDecisionStyle)
}

func (a *App) SubmitDecisionStyle(sid string, style string) error {
	if sid == "" {
		return fmt.Errorf("sid is required")
	}
	cwd := a.projectCWD(sid)
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
	cwd := a.projectCWD(sid)
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
	cwd := a.projectCWD(sid)
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
	if s.InteractionLogic != nil && len(s.InteractionLogic.Flows) > 0 {
		sb.WriteString("【交互逻辑】\n")
		for i, f := range s.InteractionLogic.Flows {
			sb.WriteString(fmt.Sprintf("  %d. %s → %s", i+1, f.Trigger, f.Action))
			if f.Description != "" {
				sb.WriteString("  // " + f.Description)
			}
			sb.WriteString("\n")
		}
		if s.InteractionLogic.Notes != "" {
			sb.WriteString("备注:" + s.InteractionLogic.Notes + "\n")
		}
		sb.WriteString("\n")
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

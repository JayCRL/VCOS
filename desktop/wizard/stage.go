// Package wizard defines the desktop GUI wizard's stage taxonomy and the
// Memory ID + Metadata schema used to persist every stage product. Every
// product is written as a memory.Entry through this package; no separate
// store or entity exists.
//
// Naming convention:
//   wizard-<stage>-<sessionID>             (single-payload stages)
//   wizard-ui-component-<sessionID>-<cid>  (per-component, stage = ui_spec)
//   wizard-ui-prompt-<sessionID>           (mapping prompt, stage = ui_prompt)
//   wizard-cursor-<sessionID>              (in-flight wizard cursor)
//
// Metadata common keys:
//   stage           string  (one of StageXxx)
//   wizardVersion   "v1"
//   createdBy       "agentui"
package wizard

import (
	"mobilevc/cognition/intake"
	"mobilevc/memory"
)

// Stage is the wizard stage identifier. Strings are the canonical values
// stored in memory.Entry.Metadata["stage"].
type Stage string

const (
	StageUserIntent     Stage = "user_intent"
	StageProjectIntent  Stage = "project_intent"
	StageUISpec         Stage = "ui_spec"
	StageUIPrompt       Stage = "ui_prompt"
	StageTechPlan       Stage = "tech_plan"
	StagePermissions    Stage = "permissions"
	StageDecisionStyle  Stage = "decision_style"
	StageExecuting      Stage = "executing"
	StageDone           Stage = "done"
	StageCursor         Stage = "cursor"
)

// AllStages is the canonical order shown in the GUI.
var AllStages = []Stage{
	StageUserIntent,
	StageProjectIntent,
	StageUISpec,
	StageTechPlan,
	StagePermissions,
	StageDecisionStyle,
	StageExecuting,
	StageDone,
}

// Metadata common keys.
const (
	MetaStage         = "stage"
	MetaWizardVersion = "wizardVersion"
	MetaCreatedBy     = "createdBy"

	WizardVersionV1 = "v1"
	CreatedByAgentUI = "agentui"

	IDPrefix = "wizard-"
)

// commonMeta returns the metadata map every wizard entry must carry.
func commonMeta(stage Stage, extras ...map[string]any) memory.Metadata {
	m := memory.Metadata{
		MetaStage:         string(stage),
		MetaWizardVersion: WizardVersionV1,
		MetaCreatedBy:     CreatedByAgentUI,
	}
	for _, e := range extras {
		for k, v := range e {
			m[k] = v
		}
	}
	return m
}

// ID builders.

func IDUserIntent(sid string) string     { return IDPrefix + "user-intent-" + sid }
func IDProjectIntent(sid string) string  { return IDPrefix + "project-intent-" + sid }
func IDUIComponent(sid, cid string) string {
	return IDPrefix + "ui-component-" + sid + "-" + cid
}
func IDUIPrompt(sid string) string       { return IDPrefix + "ui-prompt-" + sid }
func IDTechPlan(sid string) string       { return IDPrefix + "tech-plan-" + sid }
func IDPermissions(sid string) string    { return IDPrefix + "permissions-" + sid }
func IDDecisionStyle(sid string) string  { return IDPrefix + "decision-style-" + sid }
func IDCursor(sid string) string         { return IDPrefix + "cursor-" + sid }

// ——— Payload structs (JSON-encoded into memory.Entry.Content) ———

// UserIntentPayload — Stage 1.
type UserIntentPayload struct {
	Text string `json:"text"`
}

// ProjectIntentPayload — Stage 2. Wraps the intake-detected profile plus
// any free-text the user added in the wizard form.
type ProjectIntentPayload struct {
	Prompt   string                  `json:"prompt"`
	UserNote string                  `json:"userNote,omitempty"`
	Profile  intake.CognitiveProfile `json:"profile"`
}

// UIComponentPayload — Stage 3, one entry per component on the canvas.
type UIComponentPayload struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Kind        string         `json:"kind"`
	Description string         `json:"description,omitempty"`
	Props       map[string]any `json:"props,omitempty"`
	ChildrenIDs []string       `json:"childrenIds,omitempty"`
}

// UIPromptPayload — Stage 3, the synthesized mapping prompt that will be
// fed to the AI executor.
type UIPromptPayload struct {
	Prompt       string   `json:"prompt"`
	ComponentIDs []string `json:"componentIds"`
}

// TechPlanPayload — Stage 4. The AI-drafted plan plus the user's verdict.
type TechPlanPayload struct {
	Draft        string `json:"draft"`
	Decision     string `json:"decision,omitempty"`     // accept|reject|adjust
	AdjustedText string `json:"adjustedText,omitempty"`
	Approved     bool   `json:"approved"`
}

// PermissionsPayload — Stage 5. Tool capability whitelist/blacklist.
type PermissionsPayload struct {
	Allow []string `json:"allow"`
	Deny  []string `json:"deny"`
	Mode  string   `json:"mode,omitempty"` // mirrors protocol.RuntimeMeta.PermissionMode if useful
}

// DecisionStyle enumerates how the executor cooperates with the user.
type DecisionStyle string

const (
	StyleStepByStep DecisionStyle = "step-by-step"
	StyleHybrid     DecisionStyle = "hybrid"
	StyleAutonomous DecisionStyle = "autonomous"
)

// DecisionStylePayload — Stage 6.
type DecisionStylePayload struct {
	Style DecisionStyle `json:"style"`
}

// CursorPayload — wizard's in-flight position, written every time the GUI
// advances or resumes.
type CursorPayload struct {
	CurrentStage    Stage   `json:"currentStage"`
	CompletedStages []Stage `json:"completedStages"`
}

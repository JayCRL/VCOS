// Hand-rolled minimal Wails bindings declaration. Replaced by wails-generated
// bindings under wailsjs/ once `wails dev` has run, but we keep this here so
// `npm run build` works in isolation.

export type Stage =
  | "user_intent"
  | "project_intent"
  | "ui_spec"
  | "ui_prompt"
  | "interaction_logic"
  | "tech_plan"
  | "permissions"
  | "decision_style"
  | "executing"
  | "done"
  | "cursor";

export type DecisionStyle = "step-by-step" | "hybrid" | "autonomous";

export interface SessionSummary {
  id: string;
  title: string;
  createdAt: string;
  updatedAt: string;
}

export interface CursorPayload {
  currentStage: Stage;
  completedStages: Stage[];
}

export interface UserIntentPayload {
  text: string;
}

export interface ProjectHint {
  Language?: string;
  Framework?: string;
  BuildTool?: string;
  Files?: string[];
}

export interface CognitiveProfile {
  Goal: string;
  Role: string;
  Style: string;
  ProjectHint: ProjectHint;
}

export interface ProjectIntentPayload {
  prompt: string;
  userNote?: string;
  profile: CognitiveProfile;
  semantic?: Semantic;
}

export interface UIComponentPayload {
  id: string;
  name: string;
  kind: string;
  description?: string;
  props?: Record<string, unknown>;
  childrenIds?: string[];
}

export interface UIPromptPayload {
  prompt: string;
  componentIds: string[];
}

export interface InteractionFlow {
  id: string;
  trigger: string;
  componentId?: string;
  action: string;
  description?: string;
}

export interface InteractionLogicPayload {
  flows: InteractionFlow[];
  notes?: string;
}

// —— Stage 2 scan products ——

export interface TreeNode {
  name: string;
  path: string;
  isDir: boolean;
  size: number;
  modTime: string;
  children?: TreeNode[];
}

export interface EntryPoint {
  path: string;
  purpose: string;
}

export interface ScanModule {
  name: string;
  path: string;
  responsibility: string;
}

export interface DepEdge {
  from: string;
  to: string;
  kind: string;
}

export interface Hotspot {
  path: string;
  reason: string;
}

export interface Semantic {
  summary: string;
  language: string;
  entryPoints: EntryPoint[];
  modules: ScanModule[];
  deps: DepEdge[];
  hotspots: Hotspot[];
}

export interface ScanEvent {
  phase: "thinking" | "done" | "error";
  semantic?: Semantic;
  message?: string;
  raw?: string;
}

export interface DraftEvent {
  phase: "thinking" | "chunk" | "done" | "error";
  text?: string;
  message?: string;
}

export interface TechPlanPayload {
  draft: string;
  decision?: string;
  adjustedText?: string;
  approved: boolean;
}

export interface PermissionsPayload {
  allow: string[];
  deny: string[];
  mode?: string;
}

export interface DecisionStylePayload {
  style: DecisionStyle;
}

export interface WizardSnapshot {
  sessionId: string;
  cursor?: CursorPayload;
  userIntent?: UserIntentPayload;
  projectIntent?: ProjectIntentPayload;
  uiComponents?: UIComponentPayload[];
  uiPrompt?: UIPromptPayload;
  interactionLogic?: InteractionLogicPayload;
  techPlan?: TechPlanPayload;
  permissions?: PermissionsPayload;
  decisionStyle?: DecisionStylePayload;
}

export interface BusEvent {
  cursor: number;
  source: string;
  topic: string;
  sessionId: string;
  timestamp: string;
  payload: Record<string, unknown> | string | null;
}

export interface FeedbackSuggestion {
  ID: string;
  Title: string;
  Description?: string;
  Learnings?: string[];
  Confidence: number;
  Source: string;
  SessionID: string;
  CreatedAt: string;
}

// —— Stage 3 AI tweak ——

export interface APIKeyStatus {
  configured: boolean;
  source: "env" | "file" | "";
  tail?: string;
  baseURL?: string;
}

export type UIPatchOp = "rename" | "recolor" | "add" | "remove";

export interface UIPatch {
  componentId?: string;
  op: UIPatchOp;
  value?: Record<string, unknown>;
}

export interface LLMComponent {
  id: string;
  name: string;
  kind: string;
  description?: string;
}

export interface ComponentRename {
  componentId: string;
  name: string;
}

export interface UISuggestion {
  templateId: string;
  accent: string;
  font: string;
  rationale?: string;
  componentRenames?: ComponentRename[];
}

export interface FlowDraft {
  trigger: string;
  action: string;
  description?: string;
  componentId?: string;
}

export interface FlowDraftSet {
  flows: FlowDraft[];
  notes?: string;
}

declare global {
  interface Window {
    go: {
      main: {
        App: {
          StartSession(name: string): Promise<string>;
          ListSessions(): Promise<SessionSummary[]>;
          SubmitUserIntent(sid: string, text: string): Promise<void>;
          SubmitProjectIntent(
            sid: string,
            prompt: string,
            userNote: string,
            semantic: Semantic | null
          ): Promise<CognitiveProfile>;
          ScanPhysical(cwd: string): Promise<TreeNode>;
          ScanSemantic(sid: string, cwd: string): Promise<void>;
          DraftTechPlan(sid: string): Promise<void>;
          SubmitUISpec(
            sid: string,
            components: UIComponentPayload[],
            mappingPrompt: string
          ): Promise<void>;
          SubmitInteractionLogic(
            sid: string,
            payload: InteractionLogicPayload
          ): Promise<void>;
          SubmitTechPlan(
            sid: string,
            draft: string,
            decision: string,
            adjusted: string
          ): Promise<void>;
          SubmitPermissions(sid: string, p: PermissionsPayload): Promise<void>;
          SubmitDecisionStyle(sid: string, style: DecisionStyle): Promise<void>;
          LoadWizardState(sid: string): Promise<WizardSnapshot>;
          StartExecution(sid: string): Promise<void>;
          SendChat(sid: string, text: string): Promise<void>;
          ApprovePermission(sid: string, decision: string): Promise<void>;
          ApproveReview(
            sid: string,
            decision: string,
            reviewOnly: boolean
          ): Promise<void>;
          ApprovePlan(sid: string, decision: string): Promise<void>;
          ListPendingFeedback(): Promise<FeedbackSuggestion[]>;
          DecideFeedback(
            suggestionID: string,
            decision: string,
            adjusted: string
          ): Promise<void>;
          GetAPIKeyStatus(): Promise<APIKeyStatus>;
          SetAPIConfig(key: string, baseURL: string): Promise<void>;
          AdjustUIWithAI(
            prompt: string,
            accent: string,
            templateName: string,
            components: LLMComponent[]
          ): Promise<UIPatch[]>;
          SuggestUI(sid: string): Promise<UISuggestion>;
          SuggestInteractionFlows(sid: string): Promise<FlowDraftSet>;
          SuggestArchitecture(sid: string): Promise<Semantic>;
        };
      };
    };
    runtime: {
      EventsOn(name: string, cb: (e: BusEvent) => void): () => void;
      EventsOff(name: string): void;
    };
  }
}

export {};

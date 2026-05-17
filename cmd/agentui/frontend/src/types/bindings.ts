// Hand-rolled minimal Wails bindings declaration. Replaced by wails-generated
// bindings under wailsjs/ once `wails dev` has run, but we keep this here so
// `npm run build` works in isolation.

export type Stage =
  | "user_intent"
  | "project_intent"
  | "ui_spec"
  | "ui_prompt"
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
            userNote: string
          ): Promise<CognitiveProfile>;
          SubmitUISpec(
            sid: string,
            components: UIComponentPayload[],
            mappingPrompt: string
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

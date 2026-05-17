import type { BusEvent } from "../types/bindings";

export type NodeStatus = "pending" | "active" | "succeeded" | "failed";

export interface PipelineNode {
  id: string;
  label: string;
  icon: string;
  status: NodeStatus;
  detail: string;
}

export const DEFAULT_PIPELINE: PipelineNode[] = [
  { id: "understand", label: "理解任务", icon: "🧠", status: "pending", detail: "" },
  { id: "read", label: "读取项目", icon: "📂", status: "pending", detail: "" },
  { id: "draft", label: "起草改动", icon: "✍️", status: "pending", detail: "" },
  { id: "apply", label: "应用修改", icon: "🔧", status: "pending", detail: "" },
  { id: "build", label: "构建验证", icon: "🏗️", status: "pending", detail: "" },
  { id: "test", label: "运行测试", icon: "🧪", status: "pending", detail: "" },
  { id: "done", label: "完成", icon: "✅", status: "pending", detail: "" },
];

const KEYWORD_TO_NODE: { match: RegExp; nodeId: string }[] = [
  { match: /\b(plan|思考|analy[sz]ing|理解)/i, nodeId: "understand" },
  { match: /\b(read|读取|open file|scanning)/i, nodeId: "read" },
  { match: /\b(draft|propose|起草|generating)/i, nodeId: "draft" },
  { match: /\b(write|edit|patch|apply|应用)/i, nodeId: "apply" },
  { match: /\b(build|compile|tsc|cargo|go build)/i, nodeId: "build" },
  { match: /\b(test|pytest|jest|vitest)/i, nodeId: "test" },
];

const pickNodeFromText = (text: string): string | null => {
  for (const rule of KEYWORD_TO_NODE) {
    if (rule.match.test(text)) return rule.nodeId;
  }
  return null;
};

const stringFromPayload = (p: BusEvent["payload"]): string => {
  if (!p) return "";
  if (typeof p === "string") return p;
  const obj = p as Record<string, unknown>;
  const keys = ["Message", "message", "Title", "Stack", "Command", "Step", "Tool"];
  for (const k of keys) {
    const v = obj[k];
    if (typeof v === "string" && v) return v;
  }
  return "";
};

const stateFromPayload = (p: BusEvent["payload"]): string => {
  if (!p || typeof p !== "object") return "";
  const obj = p as Record<string, unknown>;
  return ((obj.State as string) ?? (obj.state as string) ?? "").toLowerCase();
};

/**
 * Apply a BusEvent to the current pipeline state and return the updated copy.
 * Pure function — easy to test and to drive from a useReducer.
 */
export const reducePipeline = (
  nodes: PipelineNode[],
  ev: BusEvent
): PipelineNode[] => {
  const topic = ev.topic?.toLowerCase() ?? "";
  const text = stringFromPayload(ev.payload);
  const state = stateFromPayload(ev.payload);

  // —— Lifecycle: pin global success/failure ——
  if (
    topic === "agent_state" &&
    ["done", "completed", "success", "finished"].includes(state)
  ) {
    return nodes.map((n) => ({
      ...n,
      status: n.status === "failed" ? "failed" : "succeeded",
    }));
  }
  if (
    (topic === "agent_state" && ["failed", "error"].includes(state)) ||
    topic.includes("error")
  ) {
    return nodes.map((n) => {
      if (n.status === "active") return { ...n, status: "failed", detail: text || n.detail };
      return n;
    });
  }

  // —— Pick a node from topic / text / state ——
  let targetId: string | null = null;
  if (topic === "session_state" && ["starting", "running"].includes(state)) {
    targetId = "understand";
  } else if (topic === "file_diff" || topic.includes("filediff")) {
    targetId = "apply";
  } else if (topic === "ai_status" || topic.includes("ai_status")) {
    targetId = "draft";
  } else if (topic === "step_update") {
    targetId = pickNodeFromText(text) ?? "draft";
  } else if (topic === "log") {
    targetId = pickNodeFromText(text);
  }

  // Fall back: look at the textual content.
  if (!targetId && text) {
    targetId = pickNodeFromText(text);
  }

  if (!targetId) return nodes;

  const idx = nodes.findIndex((n) => n.id === targetId);
  if (idx < 0) return nodes;

  // Mark every node before targetId as succeeded if it was active/pending,
  // mark target as active, keep nodes after target untouched.
  return nodes.map((n, i) => {
    if (i < idx && n.status !== "failed") {
      return { ...n, status: n.status === "succeeded" ? "succeeded" : "succeeded" };
    }
    if (i === idx) {
      return {
        ...n,
        status: n.status === "succeeded" ? "succeeded" : "active",
        detail: text || n.detail,
      };
    }
    return n;
  });
};

export const isAwaitingTopic = (topic: string): "permission" | "review" | "plan" | "interaction" | null => {
  const t = topic.toLowerCase();
  if (t.includes("permission")) return "permission";
  if (t.includes("interaction")) return "interaction";
  if (t.includes("review")) return "review";
  if (t.includes("plan_request") || t === "plan_request") return "plan";
  return null;
};

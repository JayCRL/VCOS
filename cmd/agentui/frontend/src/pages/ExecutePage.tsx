import { useCallback, useEffect, useReducer, useRef, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { PipelineStepper } from "../components/PipelineStepper";
import {
  PendingDecisionCard,
  type PendingDecision,
} from "../components/PendingDecisionCard";
import {
  DEFAULT_PIPELINE,
  isAwaitingTopic,
  reducePipeline,
  type PipelineNode,
} from "../lib/pipelineRouter";
import type { BusEvent, DecisionStyle } from "../types/bindings";

interface Props {
  sid: string;
  decisionStyle?: DecisionStyle;
}

interface State {
  nodes: PipelineNode[];
  events: BusEvent[];
}

type Action =
  | { type: "event"; ev: BusEvent }
  | { type: "reset" };

const reducer = (s: State, a: Action): State => {
  if (a.type === "reset") return { nodes: DEFAULT_PIPELINE.map((n) => ({ ...n })), events: [] };
  if (a.type === "event") {
    return {
      nodes: reducePipeline(s.nodes, a.ev),
      events: [...s.events.slice(-499), a.ev],
    };
  }
  return s;
};

const summarizeForCard = (ev: BusEvent): { title: string; detail?: string } => {
  const p = (ev.payload ?? {}) as Record<string, unknown>;
  const title =
    (p.Title as string) ||
    (p.Message as string) ||
    (p.message as string) ||
    ev.topic;
  const detail =
    (p.Step as string) ||
    (p.Command as string) ||
    (p.Description as string) ||
    undefined;
  return { title, detail };
};

export const ExecutePage = ({ sid, decisionStyle }: Props) => {
  const style: DecisionStyle = decisionStyle ?? "hybrid";
  const [state, dispatch] = useReducer(reducer, undefined, () => ({
    nodes: DEFAULT_PIPELINE.map((n) => ({ ...n })),
    events: [] as BusEvent[],
  }));
  const [pending, setPending] = useState<PendingDecision[]>([]);
  const [chat, setChat] = useState("");
  const [busy, setBusy] = useState(false);
  const [running, setRunning] = useState(false);
  const [showLogs, setShowLogs] = useState(false);
  const [err, setErr] = useState("");
  const logRef = useRef<HTMLDivElement>(null);

  // Subscribe to bus events for this session.
  const handleEvent = useCallback(
    async (ev: BusEvent) => {
      dispatch({ type: "event", ev });

      const kind = isAwaitingTopic(ev.topic ?? "");
      if (!kind) return;

      // autonomous: auto-accept
      if (style === "autonomous") {
        try {
          if (kind === "permission") await window.go.main.App.ApprovePermission(sid, "allow");
          else if (kind === "review") await window.go.main.App.ApproveReview(sid, "accept", false);
          else if (kind === "plan") await window.go.main.App.ApprovePlan(sid, "accept");
        } catch {
          /* surface as pending below */
        }
        return;
      }
      // hybrid: review/plan auto-accept; permission/interaction surface
      if (style === "hybrid" && (kind === "review" || kind === "plan")) {
        try {
          if (kind === "review") await window.go.main.App.ApproveReview(sid, "accept", false);
          else await window.go.main.App.ApprovePlan(sid, "accept");
          return;
        } catch {
          /* fall through */
        }
      }

      const s = summarizeForCard(ev);
      setPending((prev) => [
        ...prev,
        { id: ev.cursor, kind, title: s.title, detail: s.detail },
      ]);
    },
    [sid, style]
  );

  useEffect(() => {
    if (!sid) return;
    const off = window.runtime.EventsOn(`agent:${sid}`, handleEvent);
    return () => {
      try {
        off();
      } catch {
        window.runtime.EventsOff(`agent:${sid}`);
      }
    };
  }, [sid, handleEvent]);

  // Auto-scroll dev drawer.
  useEffect(() => {
    const el = logRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [state.events.length, showLogs]);

  const start = async () => {
    setBusy(true);
    setErr("");
    dispatch({ type: "reset" });
    try {
      await window.go.main.App.StartExecution(sid);
      setRunning(true);
    } catch (e) {
      setErr(`${(e as Error).message ?? e}`);
    } finally {
      setBusy(false);
    }
  };

  const send = async () => {
    if (!chat.trim()) return;
    setBusy(true);
    try {
      await window.go.main.App.SendChat(sid, chat.trim());
      setChat("");
      setRunning(true);
    } catch (e) {
      setErr(`${(e as Error).message ?? e}`);
    } finally {
      setBusy(false);
    }
  };

  const decide = async (p: PendingDecision, decision: "accept" | "reject" | "adjust") => {
    try {
      if (p.kind === "permission") await window.go.main.App.ApprovePermission(sid, decision);
      else if (p.kind === "review")
        await window.go.main.App.ApproveReview(sid, decision, false);
      else if (p.kind === "plan") await window.go.main.App.ApprovePlan(sid, decision);
      else await window.go.main.App.ApprovePermission(sid, decision);
      setPending((prev) => prev.filter((x) => x.id !== p.id));
    } catch (e) {
      setErr(`${(e as Error).message ?? e}`);
    }
  };

  const styleBadge = {
    "step-by-step": { label: "分步", cls: "exec-badge-warn" },
    hybrid: { label: "混合", cls: "exec-badge-info" },
    autonomous: { label: "全自动", cls: "exec-badge-ok" },
  } as const;
  const badge = styleBadge[style];

  return (
    <div className="exec-root">
      <div className="exec-toolbar">
        <h2 className="designer-h" style={{ margin: 0 }}>
          {running ? "正在执行" : "准备启动"}
        </h2>
        <span className={`exec-badge ${badge.cls}`}>{badge.label}</span>
        <span className="session-pill" style={{ marginLeft: 8 }}>{sid}</span>
        <div className="spacer" />
        <button className="btn" onClick={start} disabled={busy}>
          {running ? "重新启动" : "▶ 启动执行"}
        </button>
        <button className="btn btn-ghost" onClick={() => setShowLogs((v) => !v)}>
          {showLogs ? "收起开发者面板" : "开发者面板"}
        </button>
      </div>

      <div className="exec-canvas">
        <PipelineStepper nodes={state.nodes} />
        <PendingDecisionCard pending={pending} onDecide={decide} />
      </div>

      <AnimatePresence>
        {showLogs && (
          <motion.div
            className="dev-drawer"
            initial={{ height: 0, opacity: 0 }}
            animate={{ height: 220, opacity: 1 }}
            exit={{ height: 0, opacity: 0 }}
            transition={{ duration: 0.28, ease: [0.22, 1, 0.36, 1] }}
          >
            <div className="dev-drawer-head">事件流(开发者视图)</div>
            <div className="dev-drawer-log" ref={logRef}>
              {state.events.length === 0 ? (
                <span className="dev-drawer-empty">尚无事件</span>
              ) : (
                state.events.map((e) => (
                  <div className="dev-drawer-row" key={e.cursor}>
                    <span className="dev-cursor">#{e.cursor}</span>
                    <span className="dev-topic">{e.topic}</span>
                    <span className="dev-payload">
                      {summarizeForCard(e).title}
                    </span>
                  </div>
                ))
              )}
            </div>
          </motion.div>
        )}
      </AnimatePresence>

      <div className="exec-chat-bar">
        <input
          className="input"
          placeholder="想插一句话?这里说,直接发给 agent …"
          value={chat}
          onChange={(e) => setChat(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              send();
            }
          }}
          disabled={busy}
        />
        <button className="btn btn-primary" onClick={send} disabled={busy || !chat.trim()}>
          发送
        </button>
        {err && <span className="status-error" style={{ marginLeft: 8 }}>{err}</span>}
      </div>
    </div>
  );
};

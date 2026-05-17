import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { BusEvent, DecisionStyle } from "../types/bindings";

interface Props {
  sid: string;
  decisionStyle?: DecisionStyle;
}

const formatPayload = (p: BusEvent["payload"]): string => {
  if (!p) return "";
  if (typeof p === "string") return p;
  const obj = p as Record<string, unknown>;
  const candidates = ["Message", "message", "Title", "State", "Stack", "Text"];
  for (const k of candidates) {
    const v = obj[k];
    if (typeof v === "string" && v.trim()) return v;
  }
  try {
    return JSON.stringify(obj).slice(0, 240);
  } catch {
    return String(p);
  }
};

const isPermissionTopic = (topic: string) =>
  topic === "permission_request" || topic.toLowerCase().includes("permission");
const isInteractionTopic = (topic: string) =>
  topic === "interaction_request" || topic.toLowerCase().includes("interaction");
const isReviewTopic = (topic: string) =>
  topic === "review_state" || topic.toLowerCase().includes("review");
const isPlanTopic = (topic: string) =>
  topic === "plan_request" || topic.toLowerCase().includes("plan");

interface PendingDecision {
  cursor: number;
  topic: string;
  title: string;
  kind: "permission" | "review" | "plan" | "interaction";
}

export const ExecutePage = ({ sid, decisionStyle }: Props) => {
  const [events, setEvents] = useState<BusEvent[]>([]);
  const [pending, setPending] = useState<PendingDecision[]>([]);
  const [input, setInput] = useState("");
  const [busy, setBusy] = useState(false);
  const [running, setRunning] = useState(false);
  const [err, setErr] = useState("");
  const scrollerRef = useRef<HTMLDivElement>(null);

  const style: DecisionStyle = decisionStyle ?? "hybrid";

  const handleEvent = useCallback(
    async (e: BusEvent) => {
      setEvents((prev) => [...prev.slice(-499), e]);

      const payload = (e.payload ?? {}) as Record<string, unknown>;
      const title =
        (payload.Title as string) ||
        (payload.Message as string) ||
        e.topic;
      let kind: PendingDecision["kind"] | null = null;
      if (isPermissionTopic(e.topic)) kind = "permission";
      else if (isReviewTopic(e.topic)) kind = "review";
      else if (isPlanTopic(e.topic)) kind = "plan";
      else if (isInteractionTopic(e.topic)) kind = "interaction";
      if (!kind) return;

      // autonomous: auto-accept all
      if (style === "autonomous") {
        try {
          if (kind === "permission") await window.go.main.App.ApprovePermission(sid, "allow");
          else if (kind === "review") await window.go.main.App.ApproveReview(sid, "accept", false);
          else if (kind === "plan") await window.go.main.App.ApprovePlan(sid, "accept");
        } catch {
          // surface as pending if auto-accept failed
        }
        return;
      }
      // hybrid: only permission/interaction need user input; review/plan auto
      if (style === "hybrid" && (kind === "review" || kind === "plan")) {
        try {
          if (kind === "review") await window.go.main.App.ApproveReview(sid, "accept", false);
          else await window.go.main.App.ApprovePlan(sid, "accept");
          return;
        } catch {
          // fall through to surface
        }
      }
      // step-by-step or hybrid-permission/interaction: surface to user
      setPending((prev) => [
        ...prev,
        { cursor: e.cursor, topic: e.topic, title, kind },
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

  useEffect(() => {
    if (scrollerRef.current) {
      scrollerRef.current.scrollTop = scrollerRef.current.scrollHeight;
    }
  }, [events]);

  const start = async () => {
    setBusy(true);
    setErr("");
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
    if (!input.trim()) return;
    setBusy(true);
    try {
      await window.go.main.App.SendChat(sid, input.trim());
      setInput("");
      setRunning(true);
    } catch (e) {
      setErr(`${(e as Error).message ?? e}`);
    } finally {
      setBusy(false);
    }
  };

  const decide = async (p: PendingDecision, decision: string) => {
    try {
      if (p.kind === "permission") await window.go.main.App.ApprovePermission(sid, decision);
      else if (p.kind === "review") await window.go.main.App.ApproveReview(sid, decision, false);
      else if (p.kind === "plan") await window.go.main.App.ApprovePlan(sid, decision);
      else await window.go.main.App.ApprovePermission(sid, decision);
      setPending((prev) => prev.filter((x) => x.cursor !== p.cursor));
    } catch (e) {
      setErr(`${(e as Error).message ?? e}`);
    }
  };

  const styleBadge = useMemo(() => {
    const map: Record<DecisionStyle, { label: string; cls: string }> = {
      "step-by-step": { label: "分步", cls: "badge-warn" },
      hybrid: { label: "混合", cls: "badge-info" },
      autonomous: { label: "全自动", cls: "badge-ok" },
    };
    const m = map[style];
    return <span className={`badge ${m.cls}`}>{m.label}</span>;
  }, [style]);

  return (
    <section className="stage-box">
      <h2 className="stage-title">
        Stage 7 · 执行 {styleBadge}
      </h2>
      <p className="stage-hint">
        系统会把前面六步的产物拼成上下文 prompt,启动 <code>claude</code> PTY,
        所有事件流回此面板。决策风格控制要不要在每个决策点找你确认。
      </p>

      <div className="actions">
        <button onClick={start} disabled={busy}>
          {running ? "重新启动" : "启动执行"}
        </button>
        {err && <span className="status status-error">{err}</span>}
      </div>

      {pending.length > 0 && (
        <div className="pending-list">
          <div className="field-label">待审决策</div>
          {pending.map((p) => (
            <div className="pending-row" key={p.cursor}>
              <span className="pending-topic">[{p.kind}]</span>
              <span className="pending-title">{p.title}</span>
              <div className="pending-actions">
                <button onClick={() => decide(p, "accept")}>Accept</button>
                <button onClick={() => decide(p, "reject")}>Reject</button>
                {p.kind === "review" && (
                  <button onClick={() => decide(p, "adjust")}>Adjust</button>
                )}
              </div>
            </div>
          ))}
        </div>
      )}

      <div className="event-stream" ref={scrollerRef}>
        {events.length === 0 ? (
          <div className="event-empty">尚无事件 — 点击「启动执行」</div>
        ) : (
          events.map((e) => (
            <div className="event-row" key={e.cursor}>
              <span className="event-cursor">#{e.cursor}</span>
              <span className="event-topic">{e.topic}</span>
              <span className="event-payload">{formatPayload(e.payload)}</span>
            </div>
          ))
        )}
      </div>

      <div className="chat-row">
        <input
          className="comp-input"
          placeholder="给 agent 说点什么 …(回车发送)"
          value={input}
          onChange={(e) => setInput(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              send();
            }
          }}
          disabled={busy}
        />
        <button onClick={send} disabled={busy || !input.trim()}>
          发送
        </button>
      </div>
    </section>
  );
};

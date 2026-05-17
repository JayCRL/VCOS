import { useState } from "react";
import type { ConvTurn } from "../hooks/conversation.types";
import type { InteractionFlow, UIComponentPayload } from "../types/bindings";

const newId = () => `f-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 6)}`;

const FlowEditor = ({
  state,
  setState,
  done,
}: {
  state: Record<string, unknown>;
  setState: (patch: Record<string, unknown>) => void;
  done: (summary?: string) => void;
}) => {
  const flows = (state.flows as InteractionFlow[] | undefined) ?? [];
  const components = (state.components as UIComponentPayload[] | undefined) ?? [];
  const [trigger, setTrigger] = useState("");
  const [action, setAction] = useState("");
  const [desc, setDesc] = useState("");
  const [compId, setCompId] = useState<string>("");

  const add = () => {
    if (!trigger.trim() || !action.trim()) return;
    const f: InteractionFlow = {
      id: newId(),
      trigger: trigger.trim(),
      action: action.trim(),
      description: desc.trim() || undefined,
      componentId: compId || undefined,
    };
    setState({ flows: [...flows, f] });
    setTrigger("");
    setAction("");
    setDesc("");
    setCompId("");
  };
  const remove = (id: string) =>
    setState({ flows: flows.filter((f) => f.id !== id) });

  return (
    <div className="flow-editor">
      <div className="flow-list">
        {flows.length === 0 && (
          <div className="flow-empty">暂时还没有添加任何交互 — 用下面的卡片加一条吧。</div>
        )}
        {flows.map((f) => (
          <div className="flow-card" key={f.id}>
            <div className="flow-trigger">{f.trigger}</div>
            <div className="flow-arrow">→</div>
            <div className="flow-action">{f.action}</div>
            {f.description && <div className="flow-desc">{f.description}</div>}
            <button className="flow-remove" onClick={() => remove(f.id)} aria-label="删除">
              ×
            </button>
          </div>
        ))}
      </div>
      <div className="flow-form">
        <input
          className="input"
          placeholder="触发(如:点击登录按钮)"
          value={trigger}
          onChange={(e) => setTrigger(e.target.value)}
        />
        <input
          className="input"
          placeholder="动作(如:跳转到 /dashboard)"
          value={action}
          onChange={(e) => setAction(e.target.value)}
        />
        <input
          className="input"
          placeholder="补充说明(可选)"
          value={desc}
          onChange={(e) => setDesc(e.target.value)}
        />
        {components.length > 0 && (
          <select
            className="input"
            value={compId}
            onChange={(e) => setCompId(e.target.value)}
          >
            <option value="">(关联组件 · 可选)</option>
            {components.map((c) => (
              <option key={c.id} value={c.id}>
                {c.name}
              </option>
            ))}
          </select>
        )}
        <button className="btn" onClick={add} disabled={!trigger.trim() || !action.trim()}>
          + 添加
        </button>
      </div>
      <div className="flow-footer">
        <span className="flow-count">已添加 {flows.length} 条</span>
        <button
          className="btn btn-primary"
          onClick={() => done(`列了 ${flows.length} 条交互`)}
        >
          完成 →
        </button>
      </div>
    </div>
  );
};

export const interactionLogicTurns: ConvTurn[] = [
  {
    id: "kickoff",
    say:
      "好,先把刚才那些 UI 元素的「点击/输入/跳转」行为过一遍。我先帮你列几条交互草稿,你看哪些留、哪些改、再加点什么。",
  },
  {
    id: "prefill",
    effect: async (s) => {
      if ((s.flows as InteractionFlow[] | undefined)?.length) return; // already filled
      try {
        const set = await window.go.main.App.SuggestInteractionFlows(s.sid as string);
        const drafts: InteractionFlow[] = (set.flows ?? []).map((f) => ({
          id: newId(),
          trigger: f.trigger,
          action: f.action,
          description: f.description,
          componentId: f.componentId,
        }));
        return {
          flows: drafts,
          notes: (s.notes as string | undefined) ?? set.notes ?? "",
          aiNotice: drafts.length
            ? `AI 起了 ${drafts.length} 条草稿,自己挑。`
            : "AI 没给出草稿,你直接加吧。",
        };
      } catch (e) {
        return {
          flows: [],
          aiNotice: `AI 起草失败:${(e as Error).message ?? e}。你可以手动加。`,
        };
      }
    },
  },
  {
    id: "ainote",
    say: (s) => (s.aiNotice as string) ?? "",
  },
  {
    id: "flows",
    form: ({ state, setState, done }) => (
      <FlowEditor state={state} setState={setState} done={done} />
    ),
  },
  {
    id: "notesPrompt",
    say: "还有什么全局规则要告诉 agent 吗?比如未登录怎么处理、错误页长什么样。留空也行。",
  },
  {
    id: "notes",
    ask: { placeholder: "(可选)全局规则、约束、错误处理 …", multiline: true },
  },
  {
    id: "save",
    effect: async (s) => {
      const flows = (s.flows as InteractionFlow[]) ?? [];
      const notes = ((s.notes as string) ?? "").trim();
      await window.go.main.App.SubmitInteractionLogic(s.sid as string, {
        flows,
        notes: notes || undefined,
      });
    },
  },
  {
    id: "ready",
    say: (s) =>
      `好,${((s.flows as InteractionFlow[]) ?? []).length} 条交互已写入 Memory。接下来过技术方案。`,
    cta: "下一步 →",
  },
];

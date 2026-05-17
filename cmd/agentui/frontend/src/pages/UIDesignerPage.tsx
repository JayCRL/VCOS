import { useMemo, useState } from "react";
import type { UIComponentPayload, UIPromptPayload } from "../types/bindings";

interface Props {
  sid: string;
  initialComponents?: UIComponentPayload[];
  initialPrompt?: UIPromptPayload;
  onSaved: () => void;
}

const KIND_OPTIONS = [
  "screen",
  "card",
  "list",
  "form",
  "button",
  "input",
  "nav",
  "modal",
  "chart",
  "image",
  "other",
];

const newComponent = (): UIComponentPayload => ({
  id: `c-${Date.now().toString(36)}-${Math.random().toString(36).slice(2, 6)}`,
  name: "",
  kind: "card",
  description: "",
});

export const UIDesignerPage = ({
  sid,
  initialComponents,
  initialPrompt,
  onSaved,
}: Props) => {
  const [items, setItems] = useState<UIComponentPayload[]>(
    initialComponents && initialComponents.length > 0
      ? initialComponents
      : [newComponent()]
  );
  const [prompt, setPrompt] = useState(initialPrompt?.prompt ?? "");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  const autoPrompt = useMemo(() => {
    const lines = items
      .filter((c) => c.name.trim() || c.description?.trim())
      .map(
        (c, i) =>
          `${i + 1}. [${c.kind}] ${c.name || "(unnamed)"}${
            c.description ? " — " + c.description : ""
          }`
      );
    if (lines.length === 0) return "";
    return [
      "本次任务的 UI 元素映射如下,请按结构落实组件实现与样式约束:",
      ...lines,
    ].join("\n");
  }, [items]);

  const update = (idx: number, patch: Partial<UIComponentPayload>) => {
    setItems((prev) =>
      prev.map((c, i) => (i === idx ? { ...c, ...patch } : c))
    );
  };
  const remove = (idx: number) => setItems((prev) => prev.filter((_, i) => i !== idx));
  const add = () => setItems((prev) => [...prev, newComponent()]);

  const save = async () => {
    setBusy(true);
    setErr("");
    try {
      const clean = items.filter(
        (c) => c.name.trim() || c.description?.trim()
      );
      const finalPrompt = prompt.trim() || autoPrompt;
      await window.go.main.App.SubmitUISpec(sid, clean, finalPrompt);
      onSaved();
    } catch (e) {
      setErr(`${(e as Error).message ?? e}`);
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="stage-box">
      <h2 className="stage-title">Stage 3 · UI 元素特征</h2>
      <p className="stage-hint">
        列出本次要做的关键 UI 元素;每个组件的描述会被合成一条「映射提示词」写回项目记忆,后续 agent 会读到。
        (拖拽画布版本在 Step 3 接 reactflow 后接入。)
      </p>

      <div className="component-list">
        {items.map((c, i) => (
          <div className="component-row" key={c.id}>
            <input
              className="comp-input comp-input-name"
              placeholder="组件名"
              value={c.name}
              onChange={(e) => update(i, { name: e.target.value })}
              disabled={busy}
            />
            <select
              className="comp-input"
              value={c.kind}
              onChange={(e) => update(i, { kind: e.target.value })}
              disabled={busy}
            >
              {KIND_OPTIONS.map((k) => (
                <option key={k} value={k}>
                  {k}
                </option>
              ))}
            </select>
            <input
              className="comp-input comp-input-desc"
              placeholder="一句话描述外观或行为"
              value={c.description ?? ""}
              onChange={(e) => update(i, { description: e.target.value })}
              disabled={busy}
            />
            <button onClick={() => remove(i)} disabled={busy || items.length <= 1}>
              ×
            </button>
          </div>
        ))}
        <button onClick={add} disabled={busy}>
          + 添加组件
        </button>
      </div>

      <label className="field-label">
        映射提示词(留空则使用自动合成版本)
      </label>
      <textarea
        className="chat-input small"
        placeholder={autoPrompt || "(填了上面的组件后会自动生成)"}
        value={prompt}
        onChange={(e) => setPrompt(e.target.value)}
        disabled={busy}
      />
      {!prompt && autoPrompt && (
        <pre className="auto-prompt">{autoPrompt}</pre>
      )}

      <div className="actions">
        <button onClick={save} disabled={busy}>
          保存并进入下一步
        </button>
        {err && <span className="status status-error">{err}</span>}
      </div>
    </section>
  );
};

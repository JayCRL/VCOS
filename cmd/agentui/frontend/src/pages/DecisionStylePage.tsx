import { useState } from "react";
import type { DecisionStyle, DecisionStylePayload } from "../types/bindings";

const OPTIONS: { style: DecisionStyle; title: string; hint: string }[] = [
  {
    style: "step-by-step",
    title: "分步实现 · Step by Step",
    hint:
      "每完成一个阶段都暂停,等用户在 GUI 上确认才继续。最稳但最慢,适合首次合作或高风险改动。",
  },
  {
    style: "hybrid",
    title: "混合 · Hybrid",
    hint:
      "只在高风险动作(写文件、shell 执行、依赖变更)前请求确认;其余自动推进。日常默认。",
  },
  {
    style: "autonomous",
    title: "全自动 · Autonomous",
    hint:
      "全程不打扰,任务结束或失败时才推桌面通知。适合明确的小任务或夜间批量。",
  },
];

interface Props {
  sid: string;
  initial?: DecisionStylePayload;
  onSaved: () => void;
}

export const DecisionStylePage = ({ sid, initial, onSaved }: Props) => {
  const [picked, setPicked] = useState<DecisionStyle>(
    initial?.style ?? "hybrid"
  );
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  const save = async () => {
    setBusy(true);
    setErr("");
    try {
      await window.go.main.App.SubmitDecisionStyle(sid, picked);
      onSaved();
    } catch (e) {
      setErr(`${(e as Error).message ?? e}`);
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="stage-box">
      <h2 className="stage-title">Stage 6 · 决策风格</h2>
      <p className="stage-hint">
        告诉系统它在执行期间应该多频繁来打扰你。本选择会写入长期记忆,后续会话默认沿用,可随时改。
      </p>
      <div className="style-grid">
        {OPTIONS.map(({ style, title, hint }) => (
          <label
            key={style}
            className={`style-card ${picked === style ? "active" : ""}`}
          >
            <input
              type="radio"
              name="decision-style"
              checked={picked === style}
              onChange={() => setPicked(style)}
              disabled={busy}
            />
            <div className="style-card-title">{title}</div>
            <div className="style-card-hint">{hint}</div>
          </label>
        ))}
      </div>
      <div className="actions">
        <button onClick={save} disabled={busy}>
          保存并准备执行
        </button>
        {err && <span className="status status-error">{err}</span>}
      </div>
    </section>
  );
};

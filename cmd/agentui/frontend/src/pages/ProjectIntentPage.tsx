import { useState } from "react";
import type { CognitiveProfile, ProjectIntentPayload } from "../types/bindings";

interface Props {
  sid: string;
  initial?: ProjectIntentPayload;
  onSaved: () => void;
}

export const ProjectIntentPage = ({ sid, initial, onSaved }: Props) => {
  const [prompt, setPrompt] = useState(initial?.prompt ?? "");
  const [note, setNote] = useState(initial?.userNote ?? "");
  const [profile, setProfile] = useState<CognitiveProfile | undefined>(
    initial?.profile
  );
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  const save = async () => {
    if (!prompt.trim()) return;
    setBusy(true);
    setErr("");
    try {
      const p = await window.go.main.App.SubmitProjectIntent(
        sid,
        prompt.trim(),
        note.trim()
      );
      setProfile(p);
      onSaved();
    } catch (e) {
      setErr(`${(e as Error).message ?? e}`);
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="stage-box">
      <h2 className="stage-title">Stage 2 · 项目意图</h2>
      <p className="stage-hint">
        系统会基于当前工作目录自动探测项目类型(语言/框架/构建工具),然后把你写的 prompt 喂给 intake;
        在补充里写一些它探测不到的背景。
      </p>
      <label className="field-label">本阶段 prompt</label>
      <textarea
        className="chat-input"
        placeholder="这次具体想让 agent 做什么?目标产物 / 关键约束 / 接受标准 …"
        value={prompt}
        onChange={(e) => setPrompt(e.target.value)}
        disabled={busy}
      />
      <label className="field-label">补充说明(可选)</label>
      <textarea
        className="chat-input small"
        placeholder="项目背景、约定、敏感区域 …"
        value={note}
        onChange={(e) => setNote(e.target.value)}
        disabled={busy}
      />
      {profile && (
        <div className="info-box">
          <div>
            <strong>探测结果</strong>
          </div>
          <div>
            language={profile.ProjectHint.Language ?? "?"} · framework=
            {profile.ProjectHint.Framework ?? "-"} · build=
            {profile.ProjectHint.BuildTool ?? "-"}
          </div>
          <div>
            role={profile.Role} · style={profile.Style}
          </div>
          {profile.ProjectHint.Files && profile.ProjectHint.Files.length > 0 && (
            <div>files: {profile.ProjectHint.Files.join(", ")}</div>
          )}
        </div>
      )}
      <div className="actions">
        <button onClick={save} disabled={!prompt.trim() || busy}>
          运行 intake 并保存
        </button>
        {err && <span className="status status-error">{err}</span>}
      </div>
    </section>
  );
};

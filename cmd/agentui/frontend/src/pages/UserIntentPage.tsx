import { useState } from "react";

interface Props {
  sid: string;
  initialText?: string;
  onSaved: () => void;
}

export const UserIntentPage = ({ sid, initialText, onSaved }: Props) => {
  const [text, setText] = useState(initialText ?? "");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  const save = async () => {
    if (!text.trim()) return;
    setBusy(true);
    setErr("");
    try {
      await window.go.main.App.SubmitUserIntent(sid, text.trim());
      onSaved();
    } catch (e) {
      setErr(`${(e as Error).message ?? e}`);
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="stage-box">
      <h2 className="stage-title">Stage 1 · 用户意图</h2>
      <p className="stage-hint">
        告诉系统这一次的整体目标 — 写一两句话即可。下一步会基于工作目录自动探测项目并询问项目意图。
      </p>
      <textarea
        className="chat-input"
        placeholder="例如:把这个 Go 项目里的认证模块重构成 OAuth2 …"
        value={text}
        onChange={(e) => setText(e.target.value)}
        disabled={busy}
      />
      <div className="actions">
        <button onClick={save} disabled={!text.trim() || busy}>
          保存并进入下一步
        </button>
        {err && <span className="status status-error">{err}</span>}
      </div>
    </section>
  );
};

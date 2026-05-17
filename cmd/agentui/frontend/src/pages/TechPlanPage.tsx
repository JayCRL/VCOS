import { useState } from "react";
import type { TechPlanPayload } from "../types/bindings";

interface Props {
  sid: string;
  initial?: TechPlanPayload;
  onSaved: () => void;
}

export const TechPlanPage = ({ sid, initial, onSaved }: Props) => {
  const [draft, setDraft] = useState(initial?.draft ?? "");
  const [adjusted, setAdjusted] = useState(initial?.adjustedText ?? "");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  const decide = async (decision: "accept" | "reject" | "adjust") => {
    if (decision === "adjust" && !adjusted.trim()) {
      setErr("adjust 需要填写修正文本");
      return;
    }
    setBusy(true);
    setErr("");
    try {
      await window.go.main.App.SubmitTechPlan(
        sid,
        draft.trim(),
        decision,
        adjusted.trim()
      );
      onSaved();
    } catch (e) {
      setErr(`${(e as Error).message ?? e}`);
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="stage-box">
      <h2 className="stage-title">Stage 4 · 技术方案评审</h2>
      <p className="stage-hint">
        在这里写或粘贴技术方案草案,然后决定 accept / reject / adjust。
        Step 3 接入 AI 起草后,本页会变成「AI 起草 → 用户评审」双栏视图。
      </p>
      <label className="field-label">方案草案</label>
      <textarea
        className="chat-input"
        placeholder="目标 / 步骤 / 风险 / 验证手段 …"
        value={draft}
        onChange={(e) => setDraft(e.target.value)}
        disabled={busy}
      />
      <label className="field-label">adjust 时的修正文本(可选)</label>
      <textarea
        className="chat-input small"
        placeholder="对 AI 草案不完全满意?写下你的修正意见 …"
        value={adjusted}
        onChange={(e) => setAdjusted(e.target.value)}
        disabled={busy}
      />
      <div className="actions">
        <button onClick={() => decide("accept")} disabled={!draft.trim() || busy}>
          ✓ Accept
        </button>
        <button onClick={() => decide("adjust")} disabled={!draft.trim() || busy}>
          ✎ Adjust
        </button>
        <button onClick={() => decide("reject")} disabled={busy}>
          ✗ Reject
        </button>
        {err && <span className="status status-error">{err}</span>}
      </div>
    </section>
  );
};

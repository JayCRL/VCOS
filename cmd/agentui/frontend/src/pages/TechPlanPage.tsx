import { useCallback, useEffect, useState } from "react";
import { AnimatePresence, motion } from "framer-motion";
import { ChatBubble } from "../components/ChatBubble";
import { StreamingBubble } from "../components/StreamingBubble";

type Phase =
  | "intro"
  | "drafting"
  | "ready"
  | "adjust"
  | "submitting"
  | "done";

interface Props {
  sid: string;
  onSaved: () => void;
}

export const TechPlanPage = ({ sid, onSaved }: Props) => {
  const [phase, setPhase] = useState<Phase>("intro");
  const [draft, setDraft] = useState("");
  const [adjusted, setAdjusted] = useState("");
  const [err, setErr] = useState("");
  const [introTyped, setIntroTyped] = useState(false);
  const channel = `draft:${sid}`;

  // Kick off drafting after the intro bubble finishes typing.
  useEffect(() => {
    if (!introTyped || phase !== "intro") return;
    setPhase("drafting");
    (async () => {
      try {
        await window.go.main.App.DraftTechPlan(sid);
      } catch (e) {
        setErr(`触发起草失败:${(e as Error).message ?? e}`);
        setPhase("ready");
      }
    })();
  }, [introTyped, phase, sid]);

  const handleDone = useCallback((full: string) => {
    setDraft(full);
    setPhase("ready");
  }, []);

  const handleError = useCallback((msg: string) => {
    setErr(msg);
    setPhase("ready");
  }, []);

  const decide = useCallback(
    async (decision: "accept" | "reject" | "adjust") => {
      if (decision === "adjust" && !adjusted.trim()) {
        setPhase("adjust");
        return;
      }
      setPhase("submitting");
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
        setErr(`保存失败:${(e as Error).message ?? e}`);
        setPhase("ready");
      }
    },
    [adjusted, draft, sid, onSaved]
  );

  const retry = useCallback(async () => {
    setErr("");
    setDraft("");
    setPhase("drafting");
    try {
      await window.go.main.App.DraftTechPlan(sid);
    } catch (e) {
      setErr(`重试失败:${(e as Error).message ?? e}`);
      setPhase("ready");
    }
  }, [sid]);

  return (
    <div className="techplan-root">
      <div className="techplan-stream">
        <ChatBubble
          role="system"
          text="我去基于前面收集到的意图、项目背景、UI 规格和交互流,起草一份技术方案。Markdown 格式,稍等。"
          typewriter
          onTyped={() => setIntroTyped(true)}
        />
        {phase !== "intro" && (
          <StreamingBubble
            key={`draft-${sid}`}
            channel={channel}
            onDone={handleDone}
            onError={handleError}
          />
        )}
      </div>

      <AnimatePresence>
        {phase === "ready" && (
          <motion.div
            key="actions"
            className="techplan-actions"
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.24 }}
          >
            <div className="techplan-question">
              草案出来了,你怎么看?
            </div>
            <div className="techplan-buttons">
              <button
                className="btn btn-primary"
                disabled={!draft.trim()}
                onClick={() => decide("accept")}
              >
                ✓ 采纳
              </button>
              <button
                className="btn btn-ghost"
                disabled={!draft.trim()}
                onClick={() => setPhase("adjust")}
              >
                ✎ 调整
              </button>
              <button
                className="btn btn-ghost"
                onClick={() => decide("reject")}
              >
                ✗ 不行
              </button>
              <button className="btn btn-ghost" onClick={retry}>
                ↻ 再起草一次
              </button>
            </div>
            {err && <div className="status-error" style={{ marginTop: 8 }}>{err}</div>}
          </motion.div>
        )}

        {phase === "adjust" && (
          <motion.div
            key="adjust"
            className="techplan-actions"
            initial={{ opacity: 0, y: 10 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.24 }}
          >
            <div className="techplan-question">写下你的修正意见,会一起进 memory。</div>
            <textarea
              className="textarea"
              placeholder="比如:验证那部分加上 e2e 用例;步骤 2 改成先做后端契约…"
              value={adjusted}
              onChange={(e) => setAdjusted(e.target.value)}
              rows={4}
            />
            <div className="techplan-buttons">
              <button
                className="btn btn-primary"
                disabled={!adjusted.trim()}
                onClick={() => decide("adjust")}
              >
                提交修正
              </button>
              <button
                className="btn btn-ghost"
                onClick={() => setPhase("ready")}
              >
                取消
              </button>
            </div>
          </motion.div>
        )}

        {phase === "submitting" && (
          <motion.div
            key="submitting"
            className="techplan-actions"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
          >
            <div className="status-text">保存中…</div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
};

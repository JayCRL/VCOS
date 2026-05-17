import { useCallback, useEffect, useMemo, useState } from "react";
import { AnimatePresence, motion } from "framer-motion";
import { StageStepper } from "./components/StageStepper";
import { ConversationView } from "./components/ConversationView";
import { UIDesignerPage } from "./pages/UIDesignerPage";
import { ExecutePage } from "./pages/ExecutePage";
import { ProjectIntentPage } from "./pages/ProjectIntentPage";
import { TechPlanPage } from "./pages/TechPlanPage";
import { PermissionsPage } from "./pages/PermissionsPage";
import { DecisionStylePage } from "./pages/DecisionStylePage";
import { userIntentTurns } from "./scripts/userIntent";
import { interactionLogicTurns } from "./scripts/interactionLogic";
import type { Stage, WizardSnapshot } from "./types/bindings";

const PLACEHOLDER_META: Record<string, { title: string; hint: string }> = {
  ui_spec: {
    title: "选一套界面起点",
    hint: "(Step 3 落地)挑模板 → 实时渲染 → 改主色/字体/AI 微调。",
  },
  ui_prompt: {
    title: "选一套界面起点",
    hint: "(Step 3 落地)挑模板 → 实时渲染 → 改主色/字体/AI 微调。",
  },
  executing: {
    title: "正在执行 —— 进度面板",
    hint: "(Step 5 落地)流水线节点 + 待审飘卡 + 顶部 chat。",
  },
  done: { title: "全部完成", hint: "你可以开启下一个会话。" },
};

const App = () => {
  const [sid, setSid] = useState("");
  const [snapshot, setSnapshot] = useState<WizardSnapshot | null>(null);
  const [status, setStatus] = useState("");
  const [override, setOverride] = useState<Stage | null>(null);

  const refresh = useCallback(async (id: string) => {
    try {
      const snap = await window.go.main.App.LoadWizardState(id);
      setSnapshot(snap);
    } catch (e) {
      setStatus(`快照刷新失败: ${(e as Error).message ?? e}`);
    }
  }, []);

  useEffect(() => {
    (async () => {
      try {
        const sessions = await window.go.main.App.ListSessions();
        if (sessions.length > 0) {
          setSid(sessions[0].id);
          await refresh(sessions[0].id);
          setStatus("已恢复上次会话");
        } else {
          setStatus("点击 + 新会话开始");
        }
      } catch (e) {
        setStatus(`加载失败: ${(e as Error).message ?? e}`);
      }
    })();
  }, [refresh]);

  const newSession = async () => {
    try {
      const id = await window.go.main.App.StartSession("");
      setSid(id);
      setOverride(null);
      await refresh(id);
      setStatus("新建会话完成");
    } catch (e) {
      setStatus(`创建失败: ${(e as Error).message ?? e}`);
    }
  };

  const current: Stage =
    override ?? snapshot?.cursor?.currentStage ?? "user_intent";
  const completed: Stage[] = snapshot?.cursor?.completedStages ?? [];

  // 当 stage 变化时,清除 override 以让 effect 自然推进。
  useEffect(() => {
    if (
      override &&
      snapshot?.cursor?.currentStage &&
      override === snapshot.cursor.currentStage
    ) {
      setOverride(null);
    }
  }, [snapshot, override]);

  const onStageDone = useCallback(async () => {
    if (sid) await refresh(sid);
  }, [sid, refresh]);

  const stageNode = useMemo(() => {
    if (!sid) {
      return (
        <div className="placeholder-card">
          <div className="placeholder-title">还没有会话</div>
          <div className="placeholder-hint">
            点击「+ 新会话」开始向导。每一阶段的产物都会写到 Memory,
            后续执行时 MOE 会自动召回作为上下文。
          </div>
          <button className="btn btn-primary" onClick={newSession}>
            开始一个新会话
          </button>
        </div>
      );
    }
    switch (current) {
      case "user_intent":
        return (
          <ConversationView
            key={`uintent-${sid}`}
            turns={userIntentTurns}
            initial={{ sid }}
            onDone={onStageDone}
          />
        );
      case "project_intent":
        return (
          <ProjectIntentPage
            key={`pintent-${sid}`}
            sid={sid}
            onDone={onStageDone}
          />
        );
      case "ui_spec":
      case "ui_prompt":
        return (
          <UIDesignerPage
            key={`uispec-${sid}`}
            sid={sid}
            onDone={onStageDone}
          />
        );
      case "interaction_logic":
        return (
          <ConversationView
            key={`ilogic-${sid}`}
            turns={interactionLogicTurns}
            initial={{
              sid,
              components: snapshot?.uiComponents ?? [],
              flows: snapshot?.interactionLogic?.flows ?? [],
              notes: snapshot?.interactionLogic?.notes ?? "",
            }}
            onDone={onStageDone}
          />
        );
      case "tech_plan":
        return (
          <TechPlanPage
            key={`tplan-${sid}`}
            sid={sid}
            onSaved={onStageDone}
          />
        );
      case "permissions":
        return (
          <PermissionsPage
            key={`perm-${sid}`}
            sid={sid}
            initial={snapshot?.permissions}
            onSaved={onStageDone}
          />
        );
      case "decision_style":
        return (
          <DecisionStylePage
            key={`dstyle-${sid}`}
            sid={sid}
            initial={snapshot?.decisionStyle}
            onSaved={onStageDone}
          />
        );
      case "executing":
      case "done":
        return (
          <ExecutePage
            key={`exec-${sid}`}
            sid={sid}
            decisionStyle={snapshot?.decisionStyle?.style}
          />
        );
      default: {
        const m = PLACEHOLDER_META[current] ?? PLACEHOLDER_META.ui_spec;
        return (
          <div className="placeholder-card">
            <div className="placeholder-title">{m.title}</div>
            <div className="placeholder-hint">{m.hint}</div>
          </div>
        );
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sid, current, onStageDone]);

  return (
    <div className="layout">
      <header className="app-header">
        <div className="brand">
          <div className="brand-dot" />
          <span>AgentOS</span>
          <span style={{ color: "var(--text-tertiary)", fontWeight: 400 }}>
            · Desktop
          </span>
        </div>
        <div className="header-actions">
          {sid && <span className="session-pill">{sid}</span>}
          <button className="btn btn-ghost" onClick={newSession}>
            + 新会话
          </button>
        </div>
      </header>

      {sid && (
        <StageStepper
          current={current}
          completed={completed}
          onJump={(s) => setOverride(s)}
        />
      )}

      <main className="stage-main">
        <AnimatePresence mode="wait" initial={false}>
          <motion.div
            key={current}
            className="stage-canvas"
            initial={{ opacity: 0, x: 24, scale: 0.98 }}
            animate={{ opacity: 1, x: 0, scale: 1 }}
            exit={{ opacity: 0, x: -24, scale: 0.98 }}
            transition={{ duration: 0.32, ease: [0.22, 1, 0.36, 1] }}
          >
            {stageNode}
          </motion.div>
        </AnimatePresence>
      </main>

      <footer className="app-footer">
        <span className="status-text">{status}</span>
      </footer>
    </div>
  );
};

export default App;

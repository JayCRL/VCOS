import { useCallback, useEffect, useState } from "react";
import { StageStepper } from "./components/StageStepper";
import { UserIntentPage } from "./pages/UserIntentPage";
import { ProjectIntentPage } from "./pages/ProjectIntentPage";
import { UIDesignerPage } from "./pages/UIDesignerPage";
import { TechPlanPage } from "./pages/TechPlanPage";
import { PermissionsPage } from "./pages/PermissionsPage";
import { DecisionStylePage } from "./pages/DecisionStylePage";
import { ExecutePage } from "./pages/ExecutePage";
import type { Stage, WizardSnapshot } from "./types/bindings";

const App = () => {
  const [sid, setSid] = useState<string>("");
  const [snapshot, setSnapshot] = useState<WizardSnapshot | null>(null);
  const [status, setStatus] = useState<string>("");
  const [override, setOverride] = useState<Stage | null>(null);

  const refresh = useCallback(async (id: string) => {
    const snap = await window.go.main.App.LoadWizardState(id);
    setSnapshot(snap);
  }, []);

  useEffect(() => {
    (async () => {
      try {
        const sessions = await window.go.main.App.ListSessions();
        if (sessions.length > 0) {
          const latest = sessions[0];
          setSid(latest.id);
          await refresh(latest.id);
          setStatus(`resumed ${latest.id}`);
        } else {
          setStatus("点击 New Session 开始");
        }
      } catch (e) {
        setStatus(`load failed: ${(e as Error).message ?? e}`);
      }
    })();
  }, [refresh]);

  const newSession = async () => {
    try {
      const id = await window.go.main.App.StartSession("");
      setSid(id);
      setOverride(null);
      await refresh(id);
      setStatus(`new ${id}`);
    } catch (e) {
      setStatus(`create failed: ${(e as Error).message ?? e}`);
    }
  };

  const onSaved = async () => {
    if (sid) {
      setOverride(null);
      await refresh(sid);
      setStatus("saved");
    }
  };

  const current: Stage =
    override ?? snapshot?.cursor?.currentStage ?? "user_intent";
  const completed: Stage[] = snapshot?.cursor?.completedStages ?? [];

  const renderStage = () => {
    if (!sid) {
      return (
        <section className="stage-box">
          <h2 className="stage-title">还没有会话</h2>
          <p className="stage-hint">
            点击右上角 <strong>New Session</strong> 开始走向导。
          </p>
        </section>
      );
    }
    switch (current) {
      case "user_intent":
        return (
          <UserIntentPage
            sid={sid}
            initialText={snapshot?.userIntent?.text}
            onSaved={onSaved}
          />
        );
      case "project_intent":
        return (
          <ProjectIntentPage
            sid={sid}
            initial={snapshot?.projectIntent}
            onSaved={onSaved}
          />
        );
      case "ui_spec":
      case "ui_prompt":
        return (
          <UIDesignerPage
            sid={sid}
            initialComponents={snapshot?.uiComponents}
            initialPrompt={snapshot?.uiPrompt}
            onSaved={onSaved}
          />
        );
      case "tech_plan":
        return (
          <TechPlanPage
            sid={sid}
            initial={snapshot?.techPlan}
            onSaved={onSaved}
          />
        );
      case "permissions":
        return (
          <PermissionsPage
            sid={sid}
            initial={snapshot?.permissions}
            onSaved={onSaved}
          />
        );
      case "decision_style":
        return (
          <DecisionStylePage
            sid={sid}
            initial={snapshot?.decisionStyle}
            onSaved={onSaved}
          />
        );
      case "executing":
      case "done":
        return (
          <ExecutePage
            sid={sid}
            decisionStyle={snapshot?.decisionStyle?.style}
          />
        );
      default:
        return null;
    }
  };

  return (
    <div className="layout">
      <header className="header">
        <h1>AgentOS · Desktop</h1>
        <div className="header-right">
          <span className="meta">{sid ? `session ${sid}` : "no session"}</span>
          <button onClick={newSession}>New Session</button>
        </div>
      </header>

      {sid && (
        <StageStepper
          current={current}
          completed={completed}
          onJump={(s) => setOverride(s)}
        />
      )}

      {renderStage()}

      <footer className="footer">
        <span className="status">{status}</span>
      </footer>
    </div>
  );
};

export default App;

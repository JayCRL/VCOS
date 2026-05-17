import type { Stage } from "../types/bindings";

const ORDER: { stage: Stage; label: string }[] = [
  { stage: "user_intent", label: "1 · 用户意图" },
  { stage: "project_intent", label: "2 · 项目意图" },
  { stage: "ui_spec", label: "3 · UI 设计" },
  { stage: "tech_plan", label: "4 · 技术方案" },
  { stage: "permissions", label: "5 · 权限声明" },
  { stage: "decision_style", label: "6 · 决策风格" },
  { stage: "executing", label: "7 · 执行" },
];

interface Props {
  current: Stage;
  completed: Stage[];
  onJump?: (s: Stage) => void;
}

export const StageStepper = ({ current, completed, onJump }: Props) => {
  return (
    <div className="stepper">
      {ORDER.map(({ stage, label }, i) => {
        const isCurrent = stage === current;
        const isDone = completed.includes(stage);
        const cls = [
          "step",
          isCurrent ? "step-current" : "",
          isDone ? "step-done" : "",
        ]
          .filter(Boolean)
          .join(" ");
        return (
          <div
            key={stage}
            className={cls}
            onClick={() => isDone && onJump?.(stage)}
            role={isDone ? "button" : undefined}
          >
            <span className="step-bullet">{i + 1}</span>
            <span className="step-label">{label.split(" · ")[1]}</span>
          </div>
        );
      })}
    </div>
  );
};

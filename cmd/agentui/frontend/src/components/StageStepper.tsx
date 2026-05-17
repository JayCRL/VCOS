import type { Stage } from "../types/bindings";
import { motion } from "framer-motion";

const ORDER: { stage: Stage; label: string }[] = [
  { stage: "user_intent", label: "意图" },
  { stage: "project_intent", label: "项目" },
  { stage: "ui_spec", label: "UI" },
  { stage: "interaction_logic", label: "交互" },
  { stage: "tech_plan", label: "方案" },
  { stage: "permissions", label: "权限" },
  { stage: "decision_style", label: "风格" },
  { stage: "executing", label: "执行" },
];

interface Props {
  current: Stage;
  completed: Stage[];
  onJump?: (s: Stage) => void;
}

const indexOfStage = (s: Stage): number => {
  const i = ORDER.findIndex((x) => x.stage === s);
  if (i >= 0) return i;
  if (s === "ui_prompt") return 2;
  if (s === "done") return ORDER.length - 1;
  return 0;
};

export const StageStepper = ({ current, completed, onJump }: Props) => {
  const total = ORDER.length;
  const idx = indexOfStage(current);
  const progress = total <= 1 ? 1 : idx / (total - 1);

  return (
    <div className="stepper">
      <div className="stepper-track">
        <motion.div
          className="stepper-progress"
          initial={false}
          animate={{ scaleX: progress }}
          transition={{ type: "spring", stiffness: 220, damping: 28 }}
        />
        <div className="stepper-nodes">
          {ORDER.map(({ stage }, i) => {
            const state =
              i === idx
                ? "active"
                : i < idx || completed.includes(stage)
                ? "done"
                : "pending";
            const clickable = state === "done";
            return (
              <div
                key={stage}
                className="stepper-node"
                data-state={state}
                role={clickable ? "button" : undefined}
                onClick={() => clickable && onJump?.(stage)}
                style={{ cursor: clickable ? "pointer" : "default" }}
              />
            );
          })}
        </div>
      </div>
      <div className="stepper-labels">
        {ORDER.map(({ stage, label }, i) => {
          const state =
            i === idx
              ? "active"
              : i < idx || completed.includes(stage)
              ? "done"
              : "pending";
          return (
            <span key={stage} className="stepper-label" data-state={state}>
              {label}
            </span>
          );
        })}
      </div>
    </div>
  );
};

import { motion, AnimatePresence } from "framer-motion";

export interface PendingDecision {
  id: string | number;
  kind: "permission" | "review" | "plan" | "interaction";
  title: string;
  detail?: string;
}

interface Props {
  pending: PendingDecision[];
  onDecide: (p: PendingDecision, decision: "accept" | "reject" | "adjust") => void;
}

const KIND_LABEL: Record<PendingDecision["kind"], string> = {
  permission: "权限请求",
  interaction: "互动请求",
  review: "代码评审",
  plan: "计划评审",
};

export const PendingDecisionCard = ({ pending, onDecide }: Props) => (
  <AnimatePresence>
    {pending.length > 0 && (
      <motion.div
        className="pending-stack"
        initial={{ opacity: 0, y: 20, scale: 0.95 }}
        animate={{ opacity: 1, y: 0, scale: 1 }}
        exit={{ opacity: 0, y: 20, scale: 0.95 }}
        transition={{ type: "spring", stiffness: 280, damping: 26 }}
      >
        {pending.map((p) => (
          <motion.div
            key={String(p.id)}
            className="pending-card"
            layout
            initial={{ opacity: 0, y: 12, scale: 0.98 }}
            animate={{ opacity: 1, y: 0, scale: 1 }}
            exit={{ opacity: 0, x: 24, scale: 0.96 }}
            transition={{ type: "spring", stiffness: 320, damping: 26 }}
          >
            <div className="pending-card-head">
              <span className="pending-card-kind">{KIND_LABEL[p.kind]}</span>
            </div>
            <div className="pending-card-title">{p.title}</div>
            {p.detail && <div className="pending-card-detail">{p.detail}</div>}
            <div className="pending-card-actions">
              <button className="btn btn-ghost" onClick={() => onDecide(p, "reject")}>
                拒绝
              </button>
              {p.kind === "review" && (
                <button className="btn" onClick={() => onDecide(p, "adjust")}>
                  调整
                </button>
              )}
              <button className="btn btn-primary" onClick={() => onDecide(p, "accept")}>
                接受
              </button>
            </div>
          </motion.div>
        ))}
      </motion.div>
    )}
  </AnimatePresence>
);

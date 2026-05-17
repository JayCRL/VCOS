import { motion } from "framer-motion";
import type { PipelineNode } from "../lib/pipelineRouter";

interface Props {
  nodes: PipelineNode[];
}

export const PipelineStepper = ({ nodes }: Props) => {
  const activeIdx = nodes.findIndex((n) => n.status === "active");
  const detail =
    activeIdx >= 0
      ? nodes[activeIdx].detail
      : nodes.every((n) => n.status === "succeeded")
      ? "全部完成。"
      : "等待启动…";

  return (
    <div className="pipeline">
      <div className="pipeline-track">
        {nodes.map((n, i) => (
          <div key={n.id} className="pipeline-step">
            <motion.div
              className={`pipeline-node pipeline-node-${n.status}`}
              animate={
                n.status === "active"
                  ? { scale: [1, 1.08, 1] }
                  : { scale: 1 }
              }
              transition={
                n.status === "active"
                  ? { duration: 1.4, repeat: Infinity, ease: "easeInOut" }
                  : { duration: 0.24 }
              }
            >
              <span className="pipeline-node-icon">
                {n.status === "succeeded"
                  ? "✓"
                  : n.status === "failed"
                  ? "×"
                  : n.icon}
              </span>
            </motion.div>
            <div className="pipeline-label" data-state={n.status}>
              {n.label}
            </div>
            {i < nodes.length - 1 && (
              <div className={`pipeline-link pipeline-link-${nodes[i + 1].status}`} />
            )}
          </div>
        ))}
      </div>
      <motion.div
        className="pipeline-detail"
        key={detail}
        initial={{ opacity: 0, y: 6 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.24 }}
      >
        {detail}
      </motion.div>
    </div>
  );
};

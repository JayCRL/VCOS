import { useMemo } from "react";
import {
  Background,
  Controls,
  Handle,
  Position,
  ReactFlow,
  type NodeProps,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import { motion } from "framer-motion";
import { buildGraph, type GraphNodeData } from "../lib/graphLayout";
import type { Semantic, TreeNode } from "../types/bindings";

interface Props {
  tree?: TreeNode | null;
  semantic?: Semantic | null;
}

const KindNode = ({ data }: NodeProps) => {
  const d = data as GraphNodeData;
  const cls = `graph-node graph-node-${d.kind}`;
  return (
    <motion.div
      className={cls}
      initial={{ opacity: 0, scale: 0.9 }}
      animate={{ opacity: 1, scale: 1 }}
      transition={{ duration: 0.32, ease: [0.22, 1, 0.36, 1] }}
    >
      <Handle type="target" position={Position.Left} style={{ opacity: 0 }} />
      <div className="graph-node-label">{d.label}</div>
      {d.detail && <div className="graph-node-detail">{d.detail}</div>}
      {d.language && <div className="graph-node-tag">{d.language}</div>}
      <Handle type="source" position={Position.Right} style={{ opacity: 0 }} />
    </motion.div>
  );
};

const NODE_TYPES = { kindNode: KindNode } as const;

export const ProjectGraph = ({ tree, semantic }: Props) => {
  const { nodes, edges } = useMemo(
    () => buildGraph({ tree, semantic }),
    [tree, semantic]
  );

  return (
    <div className="project-graph">
      <svg width="0" height="0" style={{ position: "absolute" }}>
        <defs>
          <linearGradient id="gradEdge" x1="0%" y1="0%" x2="100%" y2="0%">
            <stop offset="0%" stopColor="#6366f1" />
            <stop offset="50%" stopColor="#a855f7" />
            <stop offset="100%" stopColor="#ec4899" />
          </linearGradient>
        </defs>
      </svg>
      <ReactFlow
        nodes={nodes}
        edges={edges}
        nodeTypes={NODE_TYPES}
        fitView
        fitViewOptions={{ padding: 0.18 }}
        proOptions={{ hideAttribution: true }}
        nodesDraggable={false}
        nodesConnectable={false}
        zoomOnDoubleClick={false}
      >
        <Background gap={32} color="rgba(15,17,21,0.04)" />
        <Controls
          position="bottom-right"
          showInteractive={false}
          style={{ background: "rgba(255,255,255,0.7)", borderRadius: 12 }}
        />
      </ReactFlow>
    </div>
  );
};

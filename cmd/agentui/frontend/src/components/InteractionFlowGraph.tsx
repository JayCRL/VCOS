import { useMemo } from "react";
import {
  Background,
  Controls,
  Handle,
  Position,
  ReactFlow,
  type EdgeProps,
  BaseEdge,
  EdgeLabelRenderer,
  getBezierPath,
} from "@xyflow/react";
import "@xyflow/react/dist/style.css";
import dagre from "@dagrejs/dagre";
import type { InteractionFlow, UIComponentPayload } from "../types/bindings";

interface Props {
  components: UIComponentPayload[];
  flows: InteractionFlow[];
  onDeleteFlow?: (id: string) => void;
}

// ── Edge with delete button ──────────────────────────────────────────

interface FlowEdgeData {
  onDelete?: (id: string) => void;
}

const FlowEdge = ({
  id,
  sourceX,
  sourceY,
  targetX,
  targetY,
  sourcePosition,
  targetPosition,
  label,
  data,
}: EdgeProps & { data?: FlowEdgeData }) => {
  const [edgePath, labelX, labelY] = getBezierPath({
    sourceX,
    sourceY,
    sourcePosition,
    targetX,
    targetY,
    targetPosition,
  });

  const handleDelete = data?.onDelete;

  return (
    <>
      <BaseEdge id={id} path={edgePath} />
      <EdgeLabelRenderer>
        <div
          className="flow-edge-label"
          style={{ position: "absolute", transform: `translate(-50%,-50%) translate(${labelX}px,${labelY}px)` }}
        >
          <span className="flow-edge-trigger">{typeof label === "string" ? label : ""}</span>
          {handleDelete && (
            <button className="flow-edge-del" onClick={() => handleDelete(id)} title="删除">
              ×
            </button>
          )}
        </div>
      </EdgeLabelRenderer>
    </>
  );
};

const EDGE_TYPES = { flowEdge: FlowEdge };

// ── Component node ───────────────────────────────────────────────────

const CompNode = ({ data }: { data: { label: string; kind: string } }) => {
  const cls = `graph-node graph-node-${data.kind === "button" ? "entryPoint" : data.kind === "input" ? "module" : data.kind === "card" ? "summary" : ""}`;
  return (
    <div className={cls || "graph-node"}>
      <Handle type="target" position={Position.Left} style={{ opacity: 0 }} />
      <div className="graph-node-label">{data.label}</div>
      <div className="graph-node-detail">{data.kind}</div>
      <Handle type="source" position={Position.Right} style={{ opacity: 0 }} />
    </div>
  );
};

const ActionNode = ({ data }: { data: { label: string; flowId: string } }) => (
  <div className="graph-node graph-node-hotspot">
    <Handle type="target" position={Position.Left} style={{ opacity: 0 }} />
    <div className="graph-node-label">{data.label}</div>
    <Handle type="source" position={Position.Right} style={{ opacity: 0 }} />
  </div>
);

const NODE_TYPES = { compNode: CompNode, actionNode: ActionNode };

// ── Layout logic ─────────────────────────────────────────────────────

interface GraphNode {
  id: string;
  label: string;
  kind: string;
  type: string;
}

export const InteractionFlowGraph = ({ components, flows, onDeleteFlow }: Props) => {
  const { nodes, edges } = useMemo(() => {
    const gNodes: GraphNode[] = [];
    const gEdges: any[] = [];

    // Component nodes
    for (const c of components) {
      gNodes.push({ id: `comp-${c.id}`, label: c.name, kind: c.kind, type: "compNode" });
    }

    // For each flow, create an action node and an edge from its component
    for (const f of flows) {
      const srcId = f.componentId ? `comp-${f.componentId}` : null;
      const actionId = `act-${f.id}`;

      if (srcId && components.some((c) => c.id === f.componentId)) {
        gNodes.push({ id: actionId, label: f.action, kind: "action", type: "actionNode" });
        gEdges.push({
          id: f.id,
          source: srcId,
          target: actionId,
          type: "flowEdge",
          label: f.trigger,
          animated: true,
          style: { stroke: "url(#gradEdge)", strokeWidth: 1.6 },
          data: { onDelete: onDeleteFlow },
        });
      } else {
        // Flow without a matched component — still create a node + phantom edge
        gNodes.push({ id: actionId, label: f.action, kind: "action", type: "actionNode" });
        gEdges.push({
          id: f.id,
          source: "global",
          target: actionId,
          type: "flowEdge",
          label: f.trigger,
          animated: true,
          style: { stroke: "rgba(239,68,68,0.5)", strokeWidth: 1.4, strokeDasharray: "5 3" },
          data: { onDelete: onDeleteFlow },
        });
      }
    }

    // Global head node (for flows without a component)
    const hasGlobal = flows.some((f) => !f.componentId || !components.some((c) => c.id === f.componentId));
    if (hasGlobal) {
      gNodes.push({ id: "global", label: "全局规则", kind: "root", type: "compNode" });
    }

    // dagre layout
    const g = new dagre.graphlib.Graph();
    g.setDefaultEdgeLabel(() => ({}));
    g.setGraph({ rankdir: "LR", ranksep: 120, nodesep: 36, marginx: 30, marginy: 30 });
    for (const n of gNodes) {
      g.setNode(n.id, { width: n.kind === "action" ? 180 : 150, height: n.kind === "action" ? 44 : 52 });
    }
    for (const e of gEdges) {
      g.setEdge(e.source, e.target);
    }
    dagre.layout(g);

    const rfNodes = gNodes.map((n) => {
      const pos = g.node(n.id);
      const w = n.kind === "action" ? 180 : 150;
      const h = n.kind === "action" ? 44 : 52;
      return {
        id: n.id,
        type: n.type,
        position: { x: pos ? pos.x - w / 2 : 0, y: pos ? pos.y - h / 2 : 0 },
        data: { label: n.label, kind: n.kind },
      };
    });

    return { nodes: rfNodes, edges: gEdges };
  }, [components, flows, onDeleteFlow]);

  if (components.length === 0) {
    return <div className="flowgraph-empty">还没有组件,请先回到上一步完成 UI 选择。</div>;
  }

  return (
    <div className="flowgraph-root">
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
        nodeTypes={NODE_TYPES as any}
        edgeTypes={EDGE_TYPES as any}
        fitView
        fitViewOptions={{ padding: 0.2 }}
        proOptions={{ hideAttribution: true }}
        nodesDraggable={false}
        nodesConnectable={false}
        zoomOnDoubleClick={false}
      >
        <Background gap={36} color="var(--border-hair)" />
        <Controls
          position="bottom-right"
          showInteractive={false}
          style={{ background: "var(--bg-elevated)", borderRadius: 12, border: "1px solid var(--border-hair)" }}
        />
      </ReactFlow>
    </div>
  );
};

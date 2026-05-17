import dagre from "@dagrejs/dagre";
import type { Edge, Node } from "@xyflow/react";
import type { Semantic, TreeNode } from "../types/bindings";

export type GraphNodeKind =
  | "root"
  | "directory"
  | "file"
  | "summary"
  | "module"
  | "entryPoint"
  | "hotspot";

export interface GraphNodeData extends Record<string, unknown> {
  label: string;
  kind: GraphNodeKind;
  detail?: string;
  language?: string;
  path?: string;
}

const KEY_FILE_RE =
  /^(readme|license|makefile|package\.json|tsconfig\.json|vite\.config|go\.mod|cargo\.toml|pyproject\.toml|requirements\.txt|dockerfile|docker-compose|cmakelists\.txt|claude\.md|agents\.md)/i;

const collectL1 = (
  root: TreeNode | null | undefined
): { dirs: TreeNode[]; files: TreeNode[] } => {
  if (!root || !root.children) return { dirs: [], files: [] };
  const dirs: TreeNode[] = [];
  const files: TreeNode[] = [];
  for (const c of root.children) {
    if (c.isDir) dirs.push(c);
    else if (KEY_FILE_RE.test(c.name)) files.push(c);
  }
  return { dirs, files };
};

interface BuildArgs {
  tree?: TreeNode | null;
  semantic?: Semantic | null;
}

export interface GraphBuildResult {
  nodes: Node<GraphNodeData>[];
  edges: Edge[];
}

export const buildGraph = ({ tree, semantic }: BuildArgs): GraphBuildResult => {
  const nodes: Node<GraphNodeData>[] = [];
  const edges: Edge[] = [];

  const pushNode = (id: string, data: GraphNodeData) => {
    nodes.push({
      id,
      data,
      position: { x: 0, y: 0 },
      type: "kindNode",
    });
  };

  // —— L1 ——
  const { dirs, files } = collectL1(tree);
  if (tree) {
    pushNode("root", { label: tree.name, kind: "root", detail: "项目根目录" });
  }
  for (const d of dirs) {
    const id = `l1-dir-${d.path}`;
    pushNode(id, { label: d.name, kind: "directory", path: d.path });
    if (tree) {
      edges.push({
        id: `e-root-${d.path}`,
        source: "root",
        target: id,
        type: "smoothstep",
        style: { stroke: "rgba(15,17,21,0.2)" },
      });
    }
  }
  for (const f of files) {
    const id = `l1-file-${f.path}`;
    pushNode(id, { label: f.name, kind: "file", path: f.path });
    if (tree) {
      edges.push({
        id: `e-root-f-${f.path}`,
        source: "root",
        target: id,
        type: "smoothstep",
        style: { stroke: "rgba(15,17,21,0.16)" },
      });
    }
  }

  // —— L3 ——
  if (semantic) {
    pushNode("summary", {
      label: semantic.summary || "项目摘要",
      kind: "summary",
      language: semantic.language,
    });
    if (tree) {
      edges.push({
        id: "e-root-summary",
        source: "root",
        target: "summary",
        style: { stroke: "rgba(99,102,241,0.4)", strokeWidth: 2 },
        type: "smoothstep",
      });
    }

    for (const m of semantic.modules ?? []) {
      const id = `mod-${m.name}`;
      pushNode(id, {
        label: m.name,
        kind: "module",
        detail: m.responsibility,
        path: m.path,
      });
      edges.push({
        id: `e-sum-${id}`,
        source: "summary",
        target: id,
        animated: true,
        type: "smoothstep",
        style: { stroke: "url(#gradEdge)", strokeWidth: 1.6 },
      });
      // Link module to matching L1 directory if path overlaps.
      const matchDir = dirs.find(
        (d) => m.path && (m.path === d.path || m.path.startsWith(d.path + "/"))
      );
      if (matchDir) {
        edges.push({
          id: `e-${id}-${matchDir.path}`,
          source: `l1-dir-${matchDir.path}`,
          target: id,
          style: { strokeDasharray: "4 4", stroke: "rgba(168,85,247,0.45)" },
          type: "straight",
        });
      }
    }
    for (const e of semantic.deps ?? []) {
      const fromId = `mod-${e.from}`;
      const toId = `mod-${e.to}`;
      const found = nodes.find((n) => n.id === fromId) && nodes.find((n) => n.id === toId);
      if (!found) continue;
      edges.push({
        id: `dep-${e.from}-${e.to}`,
        source: fromId,
        target: toId,
        animated: true,
        label: e.kind,
        type: "smoothstep",
        style: { stroke: "rgba(236,72,153,0.6)" },
        labelStyle: { fontSize: 10, fill: "#ec4899" },
      });
    }
    for (const ep of semantic.entryPoints ?? []) {
      const id = `ep-${ep.path}`;
      pushNode(id, {
        label: ep.path,
        kind: "entryPoint",
        detail: ep.purpose,
      });
      edges.push({
        id: `e-sum-${id}`,
        source: "summary",
        target: id,
        type: "smoothstep",
        style: { stroke: "rgba(59,130,246,0.5)" },
      });
    }
    for (const h of semantic.hotspots ?? []) {
      const id = `hot-${h.path}`;
      pushNode(id, {
        label: h.path,
        kind: "hotspot",
        detail: h.reason,
      });
      edges.push({
        id: `e-sum-${id}`,
        source: "summary",
        target: id,
        type: "smoothstep",
        style: { stroke: "rgba(239,68,68,0.5)" },
      });
    }
  }

  // —— dagre layout ——
  const g = new dagre.graphlib.Graph();
  g.setDefaultEdgeLabel(() => ({}));
  g.setGraph({
    rankdir: "LR",
    ranksep: 110,
    nodesep: 28,
    marginx: 30,
    marginy: 30,
  });

  const sizeOf = (k: GraphNodeKind): { w: number; h: number } => {
    if (k === "summary") return { w: 240, h: 80 };
    if (k === "root") return { w: 160, h: 60 };
    if (k === "module") return { w: 180, h: 64 };
    if (k === "entryPoint") return { w: 200, h: 56 };
    if (k === "hotspot") return { w: 220, h: 56 };
    if (k === "directory") return { w: 130, h: 44 };
    return { w: 130, h: 36 };
  };

  for (const n of nodes) {
    const { w, h } = sizeOf(n.data.kind);
    g.setNode(n.id, { width: w, height: h });
  }
  for (const e of edges) {
    g.setEdge(e.source, e.target);
  }
  dagre.layout(g);

  for (const n of nodes) {
    const p = g.node(n.id);
    if (p) {
      const { w, h } = sizeOf(n.data.kind);
      n.position = { x: p.x - w / 2, y: p.y - h / 2 };
    }
  }

  return { nodes, edges };
};

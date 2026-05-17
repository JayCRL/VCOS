import { useCallback, useEffect, useRef, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { ChatBubble } from "../components/ChatBubble";
import { ProjectGraph } from "../components/ProjectGraph";
import type { ScanEvent, Semantic, TreeNode } from "../types/bindings";

type Phase =
  | "greet"
  | "l1-scan"
  | "l3-thinking"
  | "l3-done"
  | "ask-note"
  | "submitting"
  | "done";

interface Bubble {
  id: string;
  role: "system" | "user";
  text: string;
  typing: boolean;
}

interface Props {
  sid: string;
  onDone: () => void;
}

export const ProjectIntentPage = ({ sid, onDone }: Props) => {
  const [phase, setPhase] = useState<Phase>("greet");
  const [tree, setTree] = useState<TreeNode | null>(null);
  const [semantic, setSemantic] = useState<Semantic | null>(null);
  const [bubbles, setBubbles] = useState<Bubble[]>([]);
  const [note, setNote] = useState("");
  const [err, setErr] = useState("");
  const [userIntent, setUserIntent] = useState("");
  const [thinkingText, setThinkingText] = useState("AI 正在阅读 README 和关键文件…");
  const seenBubbleIds = useRef<Set<string>>(new Set());
  const scrollerRef = useRef<HTMLDivElement>(null);

  const pushBubble = useCallback((id: string, role: Bubble["role"], text: string, typing = false) => {
    if (seenBubbleIds.current.has(id)) return;
    seenBubbleIds.current.add(id);
    setBubbles((b) => [...b, { id, role, text, typing }]);
  }, []);

  const finishBubble = useCallback((id: string) => {
    setBubbles((b) => b.map((x) => (x.id === id ? { ...x, typing: false } : x)));
  }, []);

  // Load userIntent (Stage 1 product) as the intake prompt source.
  useEffect(() => {
    (async () => {
      try {
        const snap = await window.go.main.App.LoadWizardState(sid);
        setUserIntent(snap.userIntent?.text ?? "");
      } catch {
        /* ignore */
      }
    })();
  }, [sid]);

  // Subscribe to ScanSemantic events.
  useEffect(() => {
    const off = window.runtime.EventsOn(`scan:${sid}`, (raw: unknown) => {
      const e = raw as ScanEvent;
      if (e.phase === "done" && e.semantic) {
        setSemantic(e.semantic);
        setPhase("l3-done");
      } else if (e.phase === "error") {
        setErr(e.message ?? "scan failed");
        setPhase("l3-done");
      }
    });
    return () => {
      try {
        off();
      } catch {
        window.runtime.EventsOff(`scan:${sid}`);
      }
    };
  }, [sid]);

  // Greet bubble on mount.
  useEffect(() => {
    pushBubble("greet", "system", "嗨,我先扫一下这个项目。物理结构几秒就好,然后 AI 会去读 README 和关键文件,把模块、入口、热点拎出来。", true);
  }, [pushBubble]);

  // L1 scan triggered when greet finishes typing.
  const startL1 = useCallback(async () => {
    if (phase !== "greet") return;
    setPhase("l1-scan");
    try {
      const t = await window.go.main.App.ScanPhysical(sid, "");
      setTree(t);
      const dirCount = t.children?.filter((c) => c.isDir).length ?? 0;
      const fileCount = countAllFiles(t);

      // Greenfield detection: fewer than 5 regular files and user has an intent
      // → ask LLM to propose a starter architecture instead of a file-tree scan.
      const emptyTree = fileCount < 5 && userIntent.trim().length > 0;

      pushBubble(
        "l1-done",
        "system",
        emptyTree
          ? `物理层就位 — 顶级 ${dirCount} 个目录,${fileCount} 个文件,项目几乎空着。让 AI 琢磨起步架构…`
          : `物理层就位 — 顶级 ${dirCount} 个目录,${fileCount} 个文件。现在让 AI 阅读关键内容…`,
        true
      );

      if (emptyTree) {
        setThinkingText("AI 正在构思起始代码架构…");
        setPhase("l3-thinking");
        try {
          const arch = await window.go.main.App.SuggestArchitecture(sid);
          setSemantic(arch);
          setPhase("l3-done");
        } catch (archErr) {
          // Fall back to a real scan — maybe there *is* something to read.
          setThinkingText("AI 正在阅读 README 和关键文件…");
          try {
            await window.go.main.App.ScanSemantic(sid, "");
          } catch {
            setErr(`架构推测 & 扫描均失败:${(archErr as Error).message ?? archErr}`);
            setPhase("l3-done");
          }
        }
      } else {
        setThinkingText("AI 正在阅读 README 和关键文件…");
        setPhase("l3-thinking");
        await window.go.main.App.ScanSemantic(sid, "");
      }
    } catch (e) {
      setErr(`扫描失败:${(e as Error).message ?? e}`);
    }
  }, [phase, pushBubble, sid, userIntent]);

  // When semantic arrives, emit the recap bubble.
  useEffect(() => {
    if (phase !== "l3-done") return;
    if (semantic) {
      const recap = `看起来这是一个 ${semantic.language || "(未识别)"} 项目 — ${semantic.summary}。我画了一张索引图:${semantic.modules.length} 个模块、${semantic.entryPoints.length} 个入口、${semantic.hotspots.length} 个热点。`;
      pushBubble("l3-recap", "system", recap, true);
    } else if (err) {
      pushBubble("l3-fallback", "system", `AI 扫描没拿到结果(${err}),不过物理结构已经在画布上了。继续往下走也行。`, true);
    }
  }, [phase, semantic, err, pushBubble]);

  // After recap finished typing, ask for the user's note.
  const askNote = useCallback(() => {
    pushBubble("ask-note", "system", "想补充什么项目背景吗?约定、痛点、敏感区都行,可以留空直接回车。", true);
    setPhase("ask-note");
  }, [pushBubble]);

  const submitNote = useCallback(async () => {
    setPhase("submitting");
    try {
      const prompt = userIntent.trim() || "(用户尚未填整体意图)";
      const cleanNote = note.trim();
      if (cleanNote) {
        pushBubble("user-note", "user", cleanNote);
      } else {
        pushBubble("user-note", "user", "(跳过补充)");
      }
      await window.go.main.App.SubmitProjectIntent(
        sid,
        prompt,
        cleanNote,
        semantic ?? null
      );
      onDone();
    } catch (e) {
      setErr(`保存失败:${(e as Error).message ?? e}`);
      setPhase("ask-note");
    }
  }, [userIntent, note, semantic, sid, onDone, pushBubble]);

  // Auto-scroll bubble column.
  useEffect(() => {
    if (scrollerRef.current) scrollerRef.current.scrollTop = scrollerRef.current.scrollHeight;
  }, [bubbles.length]);

  const onBubbleTyped = (id: string) => {
    finishBubble(id);
    if (id === "greet") startL1();
    if (id === "l3-recap" || id === "l3-fallback") askNote();
  };

  return (
    <div className="pintent-root">
      <div className="pintent-graph">
        <AnimatePresence>
          {(tree || phase === "l3-thinking") && (
            <motion.div
              key="graph-wrap"
              className="pintent-graph-wrap"
              initial={{ opacity: 0, scale: 0.96 }}
              animate={{ opacity: 1, scale: 1 }}
              transition={{ duration: 0.4, ease: [0.22, 1, 0.36, 1] }}
            >
              <ProjectGraph tree={tree} semantic={semantic} />
            </motion.div>
          )}
        </AnimatePresence>

        {phase === "l3-thinking" && (
          <motion.div
            className="pintent-thinking"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            transition={{ duration: 0.3 }}
          >
            <div className="thinking-pulse" />
            <span>{thinkingText}</span>
          </motion.div>
        )}

        {!tree && phase === "greet" && (
          <div className="pintent-graph-empty">
            <div className="brand-dot" style={{ width: 48, height: 48 }} />
            <span>准备扫描…</span>
          </div>
        )}
      </div>

      <div className="pintent-side">
        <div className="pintent-bubbles" ref={scrollerRef}>
          <AnimatePresence initial={false}>
            {bubbles.map((b) => (
              <ChatBubble
                key={b.id}
                role={b.role}
                text={b.text}
                typewriter={b.role === "system" && b.typing}
                onTyped={() => onBubbleTyped(b.id)}
              />
            ))}
          </AnimatePresence>
        </div>

        <AnimatePresence>
          {phase === "ask-note" && (
            <motion.div
              key="ask-note"
              className="pintent-input"
              initial={{ opacity: 0, y: 8 }}
              animate={{ opacity: 1, y: 0 }}
              exit={{ opacity: 0 }}
              transition={{ duration: 0.24 }}
            >
              <textarea
                className="textarea ask-input"
                placeholder="(可选)项目背景、约束、敏感区…"
                value={note}
                onChange={(e) => setNote(e.target.value)}
                onKeyDown={(e) => {
                  if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
                    e.preventDefault();
                    submitNote();
                  }
                }}
                rows={3}
              />
              <button className="btn btn-primary" onClick={submitNote}>
                {note.trim() ? "提交并继续" : "跳过 → 下一步"}
              </button>
            </motion.div>
          )}
          {phase === "submitting" && (
            <motion.div
              key="submitting"
              className="pintent-input"
              initial={{ opacity: 0 }}
              animate={{ opacity: 1 }}
            >
              <span className="status-text">保存中…</span>
            </motion.div>
          )}
        </AnimatePresence>

        {err && phase !== "ask-note" && (
          <div className="status-error" style={{ fontSize: 12 }}>{err}</div>
        )}
      </div>
    </div>
  );
};

const countAllFiles = (n: TreeNode | null): number => {
  if (!n) return 0;
  if (!n.isDir) return 1;
  let total = 0;
  for (const c of n.children ?? []) total += countAllFiles(c);
  return total;
};

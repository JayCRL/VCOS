import { useEffect, useMemo, useRef, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { TEMPLATES, getTemplate } from "../templates";
import {
  AccentPicker,
  FontPicker,
  ACCENT_PRESETS,
  FONT_PRESETS,
  type AccentPreset,
  type FontPreset,
} from "../components/AccentPicker";
import { APIKeyDialog } from "../components/APIKeyDialog";
import type {
  APIKeyStatus,
  LLMComponent,
  UIComponentPayload,
  UIPatch,
} from "../types/bindings";

interface Props {
  sid: string;
  initialTemplate?: string;
  initialAccent?: string;
  initialFont?: string;
  initialAdjustments?: string;
  onDone: () => void;
}

const SCALE_BASE = { width: 1100, height: 720 };

export const UIDesignerPage = ({
  sid,
  initialTemplate,
  initialAccent,
  initialFont,
  initialAdjustments,
  onDone,
}: Props) => {
  const [pickedId, setPickedId] = useState<string | null>(initialTemplate ?? null);
  const [accent, setAccent] = useState<AccentPreset>(
    ACCENT_PRESETS.find((p) => p.gradient === initialAccent) ?? ACCENT_PRESETS[0]
  );
  const [font, setFont] = useState<FontPreset>(
    FONT_PRESETS.find((p) => p.value === initialFont) ?? FONT_PRESETS[0]
  );
  const [adjustments, setAdjustments] = useState(initialAdjustments ?? "");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  // Local components — start from template, then mutated by AI patches.
  const [components, setComponents] = useState<UIComponentPayload[]>([]);
  const [aiBusy, setAIBusy] = useState(false);
  const [aiNote, setAINote] = useState("");
  const [keyOpen, setKeyOpen] = useState(false);
  const [keyStatus, setKeyStatus] = useState<APIKeyStatus | null>(null);

  // AI suggestion state — drives the auto-pick of template/accent/font when
  // the user enters Stage 3 with no prior selection.
  const [suggestBusy, setSuggestBusy] = useState(false);
  const [suggestErr, setSuggestErr] = useState("");
  const suggestRanRef = useRef(false);

  // canvas scaling — fit the 1100x720 base into the available area while preserving aspect.
  const wrapRef = useRef<HTMLDivElement>(null);
  const [scale, setScale] = useState(0.7);
  useEffect(() => {
    const recompute = () => {
      const el = wrapRef.current;
      if (!el) return;
      const r = el.getBoundingClientRect();
      const sx = (r.width - 16) / SCALE_BASE.width;
      const sy = (r.height - 16) / SCALE_BASE.height;
      setScale(Math.max(0.3, Math.min(1, Math.min(sx, sy))));
    };
    recompute();
    const obs = new ResizeObserver(recompute);
    if (wrapRef.current) obs.observe(wrapRef.current);
    return () => obs.disconnect();
  }, [pickedId]);

  const template = pickedId ? getTemplate(pickedId) : null;

  // Reset components when the template changes.
  useEffect(() => {
    if (!template) {
      setComponents([]);
      return;
    }
    setComponents(template.components.map((c) => ({ ...c })));
  }, [template?.id]);

  // Fetch key status once on mount.
  useEffect(() => {
    (async () => {
      try {
        const s = await window.go.main.App.GetAPIKeyStatus();
        setKeyStatus(s);
      } catch {
        /* ignore */
      }
    })();
  }, []);

  // Auto-suggest a template when the user lands on Stage 3 with no prior
  // selection. Skipped if no API key is configured or if they explicitly
  // backed out via "← 换个模板" (suggestRanRef prevents re-suggesting).
  useEffect(() => {
    if (suggestRanRef.current) return;
    if (pickedId) {
      suggestRanRef.current = true;
      return;
    }
    if (!keyStatus) return; // still loading
    if (!keyStatus.configured) return; // user can pick manually
    suggestRanRef.current = true;
    (async () => {
      setSuggestBusy(true);
      setSuggestErr("");
      try {
        const sug = await window.go.main.App.SuggestUI(sid);
        const tpl = TEMPLATES.find((t) => t.id === sug.templateId);
        if (!tpl) {
          throw new Error(`AI 推了未知的模板 id:${sug.templateId}`);
        }
        const matchedAccent = ACCENT_PRESETS.find(
          (p) => p.name.toLowerCase() === (sug.accent ?? "").toLowerCase()
        );
        const matchedFont = FONT_PRESETS.find((p) => p.name === sug.font);
        if (matchedAccent) setAccent(matchedAccent);
        if (matchedFont) setFont(matchedFont);
        // Apply rename overrides on the freshly-derived components.
        const renames = sug.componentRenames ?? [];
        const base: UIComponentPayload[] = tpl.components.map((c) => {
          const r = renames.find((x) => x.componentId === c.id);
          return r ? { ...c, name: r.name } : { ...c };
        });
        setComponents(base);
        setPickedId(sug.templateId);
        setAINote(
          `AI 替你选了「${tpl.name}」+ ${sug.accent} 主色${sug.rationale ? ` · ${sug.rationale}` : ""}。不喜欢的话点左上"换个模板"。`
        );
      } catch (e) {
        setSuggestErr(`${(e as Error).message ?? e}`);
      } finally {
        setSuggestBusy(false);
      }
    })();
  }, [keyStatus, pickedId, sid]);

  const llmComponents: LLMComponent[] = useMemo(
    () =>
      components.map((c) => ({
        id: c.id,
        name: c.name,
        kind: c.kind,
        description: c.description,
      })),
    [components]
  );

  const applyPatches = (patches: UIPatch[]) => {
    let next = components.slice();
    let nextAccent: AccentPreset | null = null;
    const notes: string[] = [];

    for (const p of patches) {
      switch (p.op) {
        case "recolor": {
          const name = String(p.value?.accent ?? "");
          const match = ACCENT_PRESETS.find(
            (a) => a.name.toLowerCase() === name.toLowerCase()
          );
          if (match) {
            nextAccent = match;
            notes.push(`主色 → ${match.name}`);
          } else {
            notes.push(`忽略未知主色:${name}`);
          }
          break;
        }
        case "rename": {
          const idx = next.findIndex((c) => c.id === p.componentId);
          const name = String(p.value?.name ?? "");
          if (idx >= 0 && name) {
            next = next.slice();
            next[idx] = { ...next[idx], name };
            notes.push(`重命名 ${p.componentId} → ${name}`);
          }
          break;
        }
        case "add": {
          const name = String(p.value?.name ?? "");
          const kind = String(p.value?.kind ?? "custom");
          const description = p.value?.description as string | undefined;
          if (!name) break;
          const id = `ai-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
          next = [...next, { id, name, kind, description }];
          notes.push(`新增 ${name}`);
          break;
        }
        case "remove": {
          if (p.componentId) {
            const before = next.length;
            next = next.filter((c) => c.id !== p.componentId);
            if (next.length < before) notes.push(`移除 ${p.componentId}`);
          }
          break;
        }
      }
    }

    setComponents(next);
    if (nextAccent) setAccent(nextAccent);
    setAINote(
      notes.length === 0
        ? "AI 没给出可应用的 patch(你的指令也许超出了主色/命名/增删的范围)"
        : `应用了 ${notes.length} 项调整:${notes.join(" · ")}`
    );
  };

  const runAIAdjust = async () => {
    const prompt = adjustments.trim();
    if (!prompt || !template) return;
    if (!keyStatus?.configured) {
      setKeyOpen(true);
      return;
    }
    setAIBusy(true);
    setAINote("");
    setErr("");
    try {
      const patches = await window.go.main.App.AdjustUIWithAI(
        prompt,
        accent.name,
        template.name,
        llmComponents
      );
      applyPatches(patches);
      setAdjustments("");
    } catch (e) {
      setErr(`AI 微调失败:${(e as Error).message ?? e}`);
    } finally {
      setAIBusy(false);
    }
  };

  const submit = async () => {
    if (!template) return;
    setBusy(true);
    setErr("");
    try {
      const enriched: UIComponentPayload[] = components.map((c) => ({
        ...c,
        props: { ...(c.props ?? {}), template: template.id, accent: accent.name, font: font.name },
      }));
      const mappingPrompt = composePrompt({
        templateName: template.name,
        templateDescription: template.description,
        accentName: accent.name,
        accentGradient: accent.gradient,
        fontName: font.name,
        adjustments,
        components: enriched,
      });
      await window.go.main.App.SubmitUISpec(sid, enriched, mappingPrompt);
      onDone();
    } catch (e) {
      setErr(`${(e as Error).message ?? e}`);
    } finally {
      setBusy(false);
    }
  };

  if (!template) {
    return (
      <div className="designer-root">
        {suggestBusy && (
          <motion.div
            className="designer-suggest-overlay"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
          >
            <div className="thinking-pulse" />
            <span>AI 正在替你挑模板…</span>
          </motion.div>
        )}
        <div className="designer-intro">
          <h2 className="designer-h">挑一套界面起点</h2>
          <p className="designer-sub">
            五套模板覆盖最常见的场景。选了之后会进入画布,你可以调主色、字体,
            或用自然语言告诉系统该如何微调 —— 这些信息会作为上下文写入项目记忆,
            后续 agent 会按你描述的方向实现。
          </p>
          {suggestErr && (
            <div className="status-error" style={{ marginTop: 8 }}>
              AI 推荐失败({suggestErr}),你可以手动挑一个。
            </div>
          )}
          {!keyStatus?.configured && !suggestBusy && (
            <div className="status-info" style={{ marginTop: 8 }}>
              想让 AI 帮你自动挑?
              <button className="link-btn" onClick={() => setKeyOpen(true)}>
                先配一下 API key
              </button>
            </div>
          )}
        </div>
        <div className="template-grid">
          {TEMPLATES.map((t) => {
            const Preview = t.Preview;
            return (
              <motion.button
                key={t.id}
                className="template-card"
                onClick={() => setPickedId(t.id)}
                whileHover={{ y: -4 }}
                transition={{ type: "spring", stiffness: 340, damping: 26 }}
              >
                <div className="tpl-thumb">
                  <div className="tpl-thumb-mini">
                    <Preview
                      accent={accent.gradient}
                      accentPrimary={accent.primary}
                      fontFamily={font.value}
                    />
                  </div>
                </div>
                <div className="template-meta">
                  <div className="template-name">{t.name}</div>
                  <div className="template-desc">{t.description}</div>
                </div>
              </motion.button>
            );
          })}
        </div>
      </div>
    );
  }

  const PreviewC = template.Preview;
  return (
    <div className="designer-root">
      <div className="designer-toolbar">
        <button className="btn btn-ghost" onClick={() => { suggestRanRef.current = true; setPickedId(null); }}>
          ← 换个模板
        </button>
        <div className="designer-meta">
          <strong>{template.name}</strong>
          <span style={{ color: "var(--text-tertiary)" }}>· {template.description}</span>
        </div>
        <div className="spacer" />
        <AccentPicker value={accent.gradient} onChange={setAccent} />
        <FontPicker value={font.value} onChange={setFont} />
        <button
          className="btn btn-ghost btn-icon"
          title={
            keyStatus?.configured
              ? `API key:${keyStatus.source === "env" ? "env 变量" : "已保存"}`
              : "未配置 API key"
          }
          onClick={() => setKeyOpen(true)}
        >
          {keyStatus?.configured ? "⚙" : "⚙!"}
        </button>
      </div>

      <div className="canvas-wrap" ref={wrapRef}>
        <AnimatePresence mode="wait">
          <motion.div
            key={`${template.id}-${accent.name}-${font.name}`}
            className="canvas-stage"
            style={{
              width: SCALE_BASE.width,
              height: SCALE_BASE.height,
              transform: `scale(${scale})`,
            }}
            initial={{ opacity: 0, scale: scale * 0.96 }}
            animate={{ opacity: 1, scale }}
            exit={{ opacity: 0 }}
            transition={{ duration: 0.28, ease: [0.22, 1, 0.36, 1] }}
          >
            <PreviewC
              accent={accent.gradient}
              accentPrimary={accent.primary}
              fontFamily={font.value}
            />
          </motion.div>
        </AnimatePresence>

        {aiBusy && (
          <motion.div
            className="designer-ai-overlay"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
          >
            <div className="thinking-pulse" />
            <span>AI 正在调整 UI…</span>
          </motion.div>
        )}
      </div>

      <div className="designer-footer">
        <input
          className="input designer-adjust"
          placeholder="想再微调?用一句话告诉 AI — 比如:把按钮换深紫色,加一个搜索框"
          value={adjustments}
          onChange={(e) => setAdjustments(e.target.value)}
          onKeyDown={(e) => {
            if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
              e.preventDefault();
              runAIAdjust();
            }
          }}
          disabled={busy || aiBusy}
        />
        <button
          className="btn btn-ghost"
          onClick={runAIAdjust}
          disabled={busy || aiBusy || !adjustments.trim()}
          title="按 ⌘/Ctrl + Enter 也行"
        >
          ✨ AI 应用
        </button>
        <button className="btn btn-primary" onClick={submit} disabled={busy || aiBusy}>
          {busy ? "提交中…" : "保存 · 下一步 →"}
        </button>
      </div>

      <AnimatePresence>
        {(aiNote || err) && (
          <motion.div
            className="designer-note"
            initial={{ opacity: 0, y: 6 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0 }}
          >
            {err ? (
              <span className="status-error">{err}</span>
            ) : (
              <span className="status-info">{aiNote}</span>
            )}
          </motion.div>
        )}
      </AnimatePresence>

      <APIKeyDialog
        open={keyOpen}
        onClose={() => setKeyOpen(false)}
        onSaved={(s) => setKeyStatus(s)}
      />
    </div>
  );
};

interface PromptArgs {
  templateName: string;
  templateDescription: string;
  accentName: string;
  accentGradient: string;
  fontName: string;
  adjustments: string;
  components: UIComponentPayload[];
}

const composePrompt = (a: PromptArgs): string => {
  const lines: string[] = [];
  lines.push(`用户选择的 UI 模板:${a.templateName}(${a.templateDescription})`);
  lines.push(`主色调:${a.accentName} · 渐变 ${a.accentGradient}`);
  lines.push(`字体偏好:${a.fontName}`);
  if (a.adjustments.trim()) {
    lines.push("");
    lines.push(`用户的微调说明:${a.adjustments.trim()}`);
  }
  lines.push("");
  lines.push("本次需要实现的关键 UI 组件:");
  a.components.forEach((c, i) => {
    lines.push(`  ${i + 1}. [${c.kind}] ${c.name}${c.description ? " — " + c.description : ""}`);
  });
  return lines.join("\n");
};

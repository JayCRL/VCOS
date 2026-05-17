import { useEffect, useState } from "react";
import { motion } from "framer-motion";
import type { APIKeyStatus } from "../types/bindings";

interface Props {
  open: boolean;
  onClose: () => void;
  onSaved?: (status: APIKeyStatus) => void;
}

export const APIKeyDialog = ({ open, onClose, onSaved }: Props) => {
  const [status, setStatus] = useState<APIKeyStatus | null>(null);
  const [draft, setDraft] = useState("");
  const [baseURL, setBaseURL] = useState("");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  useEffect(() => {
    if (!open) return;
    setErr("");
    setDraft("");
    (async () => {
      try {
        const s = await window.go.main.App.GetAPIKeyStatus();
        setStatus(s);
        setBaseURL(s.baseURL ?? "");
      } catch (e) {
        setErr(`读取状态失败:${(e as Error).message ?? e}`);
      }
    })();
  }, [open]);

  const save = async () => {
    const key = draft.trim();
    const url = baseURL.trim();
    if (key && !key.startsWith("sk-")) {
      setErr("API key 看起来不对(通常以 sk-ant- 或 sk- 开头)");
      return;
    }
    if (url && !/^https?:\/\//i.test(url)) {
      setErr("Base URL 需以 http:// 或 https:// 开头");
      return;
    }
    if (!key && !status?.configured) {
      setErr("请填入 API key");
      return;
    }
    setBusy(true);
    setErr("");
    try {
      // Keep existing key when user only edits baseURL — backend SaveConfig
      // clears empties, so we send empty key only when user actively cleared it.
      await window.go.main.App.SetAPIConfig(key || "", url);
      const next = await window.go.main.App.GetAPIKeyStatus();
      setStatus(next);
      setDraft("");
      onSaved?.(next);
    } catch (e) {
      setErr(`${(e as Error).message ?? e}`);
    } finally {
      setBusy(false);
    }
  };

  const clearKey = async () => {
    setBusy(true);
    setErr("");
    try {
      await window.go.main.App.SetAPIConfig("", baseURL.trim());
      const next = await window.go.main.App.GetAPIKeyStatus();
      setStatus(next);
      onSaved?.(next);
    } catch (e) {
      setErr(`${(e as Error).message ?? e}`);
    } finally {
      setBusy(false);
    }
  };

  if (!open) return null;

  return (
    <motion.div
      className="apikey-overlay"
      initial={{ opacity: 0 }}
      animate={{ opacity: 1 }}
      exit={{ opacity: 0 }}
      onClick={onClose}
    >
      <motion.div
        className="apikey-panel"
        initial={{ scale: 0.96, y: 10 }}
        animate={{ scale: 1, y: 0 }}
        transition={{ duration: 0.24, ease: [0.22, 1, 0.36, 1] }}
        onClick={(e) => e.stopPropagation()}
      >
        <div className="apikey-head">
          <div className="apikey-title">Anthropic API Key</div>
          <button className="btn btn-ghost btn-icon" onClick={onClose} aria-label="关闭">
            ×
          </button>
        </div>

        <div className="apikey-status">
          {status?.configured ? (
            <>
              <span className="status-dot status-dot-ok" />
              <span>
                已配置 ·{" "}
                {status.source === "env"
                  ? "通过环境变量 ANTHROPIC_API_KEY"
                  : "保存在 ~/.mobilevc/agentui-config.json"}
                {status.tail && ` · …${status.tail}`}
                {status.baseURL && (
                  <>
                    {" · "}
                    <span style={{ fontFamily: "var(--font-mono)", fontSize: 11 }}>
                      {status.baseURL}
                    </span>
                  </>
                )}
              </span>
            </>
          ) : (
            <>
              <span className="status-dot status-dot-warn" />
              <span>未配置 · AI 微调将无法使用</span>
            </>
          )}
        </div>

        <label className="field-label">输入新的 API key</label>
        <input
          className="input apikey-input"
          type="password"
          autoComplete="off"
          spellCheck={false}
          placeholder={status?.configured ? "留空则保持现有 key 不变" : "sk-ant-..."}
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
          disabled={busy}
        />

        <label className="field-label" style={{ marginTop: 10 }}>
          Base URL(可选 · 走代理 / 兼容网关时填)
        </label>
        <input
          className="input apikey-input"
          type="text"
          autoComplete="off"
          spellCheck={false}
          placeholder="https://api.anthropic.com  (留空使用官方)"
          value={baseURL}
          onChange={(e) => setBaseURL(e.target.value)}
          disabled={busy}
        />

        <div className="apikey-hint">
          保存到本地文件,权限 0600。环境变量 ANTHROPIC_API_KEY / ANTHROPIC_BASE_URL 优先级最高。
        </div>

        {err && <div className="status-error apikey-err">{err}</div>}

        <div className="apikey-buttons">
          <button
            className="btn btn-primary"
            disabled={busy || !draft.trim()}
            onClick={save}
          >
            {busy ? "保存中…" : "保存"}
          </button>
          {status?.configured && status.source === "file" && (
            <button
              className="btn btn-ghost"
              disabled={busy}
              onClick={clearKey}
            >
              清除已存的 key
            </button>
          )}
          <button className="btn btn-ghost" onClick={onClose} disabled={busy}>
            关闭
          </button>
        </div>
      </motion.div>
    </motion.div>
  );
};

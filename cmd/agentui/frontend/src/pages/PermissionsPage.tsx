import { useState } from "react";
import type { PermissionsPayload } from "../types/bindings";

const DEFAULT_TOOLS = [
  "read",
  "write",
  "shell",
  "git",
  "build",
  "test",
  "lint",
  "format",
  "network",
  "package_install",
  "delete",
  "shadow",
];

interface Props {
  sid: string;
  initial?: PermissionsPayload;
  onSaved: () => void;
}

export const PermissionsPage = ({ sid, initial, onSaved }: Props) => {
  const [decision, setDecision] = useState<Record<string, "allow" | "deny" | "ask">>(() => {
    const map: Record<string, "allow" | "deny" | "ask"> = {};
    const allow = new Set(initial?.allow ?? []);
    const deny = new Set(initial?.deny ?? []);
    for (const t of DEFAULT_TOOLS) {
      map[t] = allow.has(t) ? "allow" : deny.has(t) ? "deny" : "ask";
    }
    return map;
  });
  const [mode, setMode] = useState(initial?.mode ?? "");
  const [busy, setBusy] = useState(false);
  const [err, setErr] = useState("");

  const save = async () => {
    setBusy(true);
    setErr("");
    try {
      const payload: PermissionsPayload = {
        allow: Object.entries(decision)
          .filter(([, v]) => v === "allow")
          .map(([k]) => k),
        deny: Object.entries(decision)
          .filter(([, v]) => v === "deny")
          .map(([k]) => k),
        mode: mode.trim(),
      };
      await window.go.main.App.SubmitPermissions(sid, payload);
      onSaved();
    } catch (e) {
      setErr(`${(e as Error).message ?? e}`);
    } finally {
      setBusy(false);
    }
  };

  return (
    <section className="stage-box">
      <h2 className="stage-title">Stage 5 · 权限声明</h2>
      <p className="stage-hint">
        给定本次执行可使用的工具清单。「ask」表示每次使用前都要弹审(默认),「allow」自动放行,「deny」直接拒绝。
      </p>

      <div className="perm-grid">
        {DEFAULT_TOOLS.map((tool) => (
          <div key={tool} className="perm-row">
            <span className="perm-name">{tool}</span>
            {(["allow", "ask", "deny"] as const).map((v) => (
              <label key={v} className={`perm-chip perm-${v} ${decision[tool] === v ? "active" : ""}`}>
                <input
                  type="radio"
                  name={`perm-${tool}`}
                  checked={decision[tool] === v}
                  onChange={() => setDecision((prev) => ({ ...prev, [tool]: v }))}
                  disabled={busy}
                />
                {v}
              </label>
            ))}
          </div>
        ))}
      </div>

      <label className="field-label">
        protocol.RuntimeMeta.PermissionMode 备注(可选,如 "plan-only" / "review-first")
      </label>
      <input
        className="comp-input"
        value={mode}
        onChange={(e) => setMode(e.target.value)}
        disabled={busy}
        placeholder="不填则使用 agent 默认模式"
      />

      <div className="actions">
        <button onClick={save} disabled={busy}>
          保存并进入下一步
        </button>
        {err && <span className="status status-error">{err}</span>}
      </div>
    </section>
  );
};

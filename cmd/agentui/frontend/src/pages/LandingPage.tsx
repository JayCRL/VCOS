import { useEffect, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import type { SessionSummary } from "../types/bindings";

interface Props {
  onNewSession: () => Promise<void>;
  onOpenFolder: () => Promise<void>;
  onSelectSession: (id: string) => Promise<void>;
}

const fmtTime = (t: string) => {
  const d = new Date(t);
  if (isNaN(d.getTime())) return t;
  return d.toLocaleDateString(undefined, {
    month: "short",
    day: "numeric",
    hour: "2-digit",
    minute: "2-digit",
  });
};

export const LandingPage = ({ onNewSession, onOpenFolder, onSelectSession }: Props) => {
  const [allSessions, setAllSessions] = useState<SessionSummary[]>([]);
  const [showAll, setShowAll] = useState(false);

  useEffect(() => {
    (async () => {
      try {
        const list = await window.go.main.App.ListSessions();
        setAllSessions(list);
      } catch {
        /* ignore */
      }
    })();
  }, []);

  const recent = allSessions.slice(0, 2);

  return (
    <div className="landing-root">
      <motion.div
        className="landing-card"
        initial={{ opacity: 0, y: 24 }}
        animate={{ opacity: 1, y: 0 }}
        transition={{ duration: 0.4, ease: [0.22, 1, 0.36, 1] }}
      >
        <div className="landing-brand">
          <div className="brand-dot" style={{ width: 48, height: 48 }} />
          <h1>AgentOS</h1>
          <span className="landing-sub">Desktop · AI-Native 项目向导</span>
        </div>

        <div className="landing-actions">
          <button className="btn btn-primary landing-btn" onClick={onOpenFolder}>
            打开已有项目文件夹…
          </button>
          <button className="btn landing-btn landing-btn-new" onClick={onNewSession}>
            新建项目
          </button>
        </div>

        {recent.length > 0 && (
          <div className="landing-recent">
            <div className="landing-recent-title">最近</div>
            {recent.map((s) => (
              <button
                key={s.id}
                className="landing-recent-item"
                onClick={() => onSelectSession(s.id)}
              >
                <div className="landing-recent-name">{s.title}</div>
                <div className="landing-recent-time">{fmtTime(s.createdAt)}</div>
              </button>
            ))}
            {allSessions.length > 2 && (
              <button className="link-btn landing-more" onClick={() => setShowAll(true)}>
                查看全部({allSessions.length}) →
              </button>
            )}
          </div>
        )}
      </motion.div>

      {/* All sessions modal */}
      <AnimatePresence>
        {showAll && (
          <motion.div
            className="landing-overlay"
            initial={{ opacity: 0 }}
            animate={{ opacity: 1 }}
            exit={{ opacity: 0 }}
            onClick={() => setShowAll(false)}
          >
            <motion.div
              className="landing-modal"
              initial={{ scale: 0.96, y: 10 }}
              animate={{ scale: 1, y: 0 }}
              exit={{ scale: 0.96, y: 10 }}
              transition={{ duration: 0.24 }}
              onClick={(e) => e.stopPropagation()}
            >
              <div className="landing-modal-head">
                <span>全部会话</span>
                <button className="btn btn-ghost btn-icon" onClick={() => setShowAll(false)}>×</button>
              </div>
              <div className="landing-modal-list">
                {allSessions.map((s) => (
                  <button
                    key={s.id}
                    className="landing-recent-item"
                    onClick={() => { setShowAll(false); onSelectSession(s.id); }}
                  >
                    <div className="landing-recent-name">{s.title}</div>
                    <div className="landing-recent-time">{fmtTime(s.createdAt)}</div>
                  </button>
                ))}
              </div>
            </motion.div>
          </motion.div>
        )}
      </AnimatePresence>
    </div>
  );
};

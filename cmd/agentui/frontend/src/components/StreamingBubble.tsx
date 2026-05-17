import { useEffect, useRef, useState } from "react";
import { motion } from "framer-motion";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import type { DraftEvent } from "../types/bindings";

interface Props {
  channel: string;
  onDone?: (fullText: string) => void;
  onError?: (msg: string) => void;
  // Render an empty placeholder when not yet streaming.
  placeholder?: string;
}

// StreamingBubble listens on a Wails event channel that emits DraftEvent
// values and incrementally renders the accumulated text as GitHub-flavored
// markdown. On `done` it calls onDone(full); on `error` it calls onError(msg)
// but still renders whatever text arrived first.
export const StreamingBubble = ({
  channel,
  onDone,
  onError,
  placeholder = "AI 正在思考…",
}: Props) => {
  const [text, setText] = useState("");
  const [phase, setPhase] = useState<DraftEvent["phase"] | "idle">("idle");
  const scrollerRef = useRef<HTMLDivElement>(null);
  const accRef = useRef("");

  useEffect(() => {
    accRef.current = "";
    setText("");
    setPhase("idle");
    const off = window.runtime.EventsOn(channel, (raw: unknown) => {
      const e = raw as DraftEvent;
      if (e.phase === "thinking") {
        setPhase("thinking");
      } else if (e.phase === "chunk") {
        if (typeof e.text === "string") {
          accRef.current += e.text;
          setText(accRef.current);
          setPhase("chunk");
        }
      } else if (e.phase === "done") {
        const full = e.text ?? accRef.current;
        accRef.current = full;
        setText(full);
        setPhase("done");
        onDone?.(full);
      } else if (e.phase === "error") {
        setPhase("error");
        onError?.(e.message ?? "draft failed");
      }
    });
    return () => {
      try {
        off();
      } catch {
        window.runtime.EventsOff(channel);
      }
    };
  }, [channel, onDone, onError]);

  useEffect(() => {
    if (scrollerRef.current) {
      scrollerRef.current.scrollTop = scrollerRef.current.scrollHeight;
    }
  }, [text]);

  return (
    <motion.div
      className="stream-bubble"
      initial={{ opacity: 0, y: 6 }}
      animate={{ opacity: 1, y: 0 }}
      transition={{ duration: 0.28 }}
    >
      <div className="stream-bubble-head">
        <div className="thinking-pulse" />
        <span>
          {phase === "thinking"
            ? "AI 正在阅读上下文…"
            : phase === "chunk"
            ? "AI 正在起草…"
            : phase === "done"
            ? "草案已生成"
            : phase === "error"
            ? "草案出错(可重试)"
            : "等待启动"}
        </span>
      </div>
      <div className="stream-bubble-body" ref={scrollerRef}>
        {text ? (
          <ReactMarkdown remarkPlugins={[remarkGfm]}>{text}</ReactMarkdown>
        ) : (
          <div className="stream-bubble-empty">{placeholder}</div>
        )}
        {(phase === "thinking" || phase === "chunk") && (
          <span className="stream-cursor">▍</span>
        )}
      </div>
    </motion.div>
  );
};

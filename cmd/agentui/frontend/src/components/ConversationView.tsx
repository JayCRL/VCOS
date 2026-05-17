import { useEffect, useRef, useState } from "react";
import { motion, AnimatePresence } from "framer-motion";
import { ChatBubble } from "./ChatBubble";
import { useConversation } from "../hooks/useConversation";
import type { ConvState, ConvTurn } from "../hooks/conversation.types";

interface Props {
  turns: ConvTurn[];
  initial?: ConvState;
  onDone?: (state: ConvState) => void;
  /** Optional custom render for the form turn — given the turn and the ctx. */
}

export const ConversationView = ({ turns, initial, onDone }: Props) => {
  const conv = useConversation(turns, initial ?? {});
  const [draft, setDraft] = useState("");
  const scrollerRef = useRef<HTMLDivElement>(null);

  // Reset draft when a new ask turn opens.
  useEffect(() => {
    if (conv.awaitingAsk && conv.currentTurn?.ask) {
      const fromState = conv.currentTurn.ask.initialFromState;
      const f = conv.currentTurn.field || conv.currentTurn.id;
      const initial = fromState ?? (conv.state[f] as string | undefined) ?? "";
      setDraft(initial);
    }
  }, [conv.awaitingAsk, conv.currentTurn, conv.state]);

  // Notify on completion.
  useEffect(() => {
    if (conv.phase === "done" && onDone) {
      onDone(conv.state);
    }
  }, [conv.phase, conv.state, onDone]);

  // Auto-scroll to bottom on history change.
  useEffect(() => {
    const el = scrollerRef.current;
    if (el) el.scrollTop = el.scrollHeight;
  }, [conv.history.length, conv.phase]);

  const ask = conv.awaitingAsk ? conv.currentTurn?.ask : null;
  const form =
    conv.awaitingForm && conv.currentTurn?.form
      ? conv.currentTurn.form({
          state: conv.state,
          setState: conv.setState,
          done: conv.finishForm,
        })
      : null;
  const cta = conv.awaitingCTA ? conv.currentTurn?.cta : null;

  const submit = () => {
    if (!ask) return;
    if (!ask.multiline && !draft.trim()) return;
    conv.submit(draft);
    setDraft("");
  };

  return (
    <div className="conversation">
      <div className="bubbles" ref={scrollerRef}>
        <AnimatePresence initial={false}>
          {conv.history.map((h) => (
            <ChatBubble
              key={h.id}
              role={h.role}
              text={h.text}
              typewriter={h.role === "system" && h.typing}
              onTyped={() => conv.markTypingDone(h.id)}
            />
          ))}
        </AnimatePresence>
      </div>

      <AnimatePresence mode="wait">
        {form && (
          <motion.div
            key={`form-${conv.currentTurn?.id}`}
            className="form-zone"
            initial={{ opacity: 0, y: 12 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -8 }}
            transition={{ duration: 0.28, ease: [0.22, 1, 0.36, 1] }}
          >
            {form}
          </motion.div>
        )}

        {ask && (
          <motion.div
            key={`ask-${conv.currentTurn?.id}`}
            className="ask-zone"
            initial={{ opacity: 0, y: 12 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -8 }}
            transition={{ duration: 0.24, ease: [0.22, 1, 0.36, 1] }}
          >
            {ask.multiline ? (
              <textarea
                className="textarea ask-input"
                placeholder={ask.placeholder}
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                onKeyDown={(e) => {
                  if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
                    e.preventDefault();
                    submit();
                  }
                }}
                rows={3}
              />
            ) : (
              <input
                className="input ask-input"
                placeholder={ask.placeholder}
                value={draft}
                onChange={(e) => setDraft(e.target.value)}
                onKeyDown={(e) => {
                  if (e.key === "Enter") {
                    e.preventDefault();
                    submit();
                  }
                }}
              />
            )}
            <button
              className="btn btn-primary"
              onClick={submit}
              disabled={!draft.trim()}
            >
              {ask.submitLabel ?? (ask.multiline ? "提交 ⌘↵" : "提交 ↵")}
            </button>
          </motion.div>
        )}

        {cta && (
          <motion.div
            key={`cta-${conv.currentTurn?.id}`}
            className="cta-zone"
            initial={{ opacity: 0, y: 8 }}
            animate={{ opacity: 1, y: 0 }}
            exit={{ opacity: 0, y: -8 }}
            transition={{ duration: 0.24, ease: [0.22, 1, 0.36, 1] }}
          >
            <button className="btn btn-primary" onClick={conv.proceedCTA}>
              {cta}
            </button>
          </motion.div>
        )}
      </AnimatePresence>

      {conv.error && <div className="conversation-error">{conv.error}</div>}
    </div>
  );
};

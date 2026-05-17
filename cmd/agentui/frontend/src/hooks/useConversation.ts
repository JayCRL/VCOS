import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import type { ConvState, ConvTurn } from "./conversation.types";

interface HistoryItem {
  id: string;
  role: "system" | "user";
  text: string;
  typing?: boolean;
}

interface UseConversationResult {
  history: HistoryItem[];
  state: ConvState;
  setState: (patch: ConvState) => void;
  currentTurn: ConvTurn | null;
  phase: "typing" | "asking" | "form" | "effect" | "cta" | "done";
  awaitingAsk: boolean;
  awaitingForm: boolean;
  awaitingCTA: boolean;
  submit: (input?: string) => void;
  finishForm: (summary?: string) => void;
  proceedCTA: () => void;
  markTypingDone: (id: string) => void;
  error: string;
}

export const useConversation = (
  turns: ConvTurn[],
  initial: ConvState = {}
): UseConversationResult => {
  const [state, setStateInternal] = useState<ConvState>(initial);
  const [history, setHistory] = useState<HistoryItem[]>([]);
  const [cursor, setCursor] = useState(0);
  const [phase, setPhase] = useState<UseConversationResult["phase"]>("typing");
  const [error, setError] = useState("");
  const sayDoneRef = useRef<Set<string>>(new Set());
  const effectRanRef = useRef<Set<number>>(new Set());

  const setState = useCallback((patch: ConvState) => {
    setStateInternal((prev) => ({ ...prev, ...patch }));
  }, []);

  const currentTurn = cursor < turns.length ? turns[cursor] : null;

  // —— resolve the current turn's lifecycle ——
  useEffect(() => {
    if (!currentTurn) {
      setPhase("done");
      return;
    }
    const t = currentTurn;
    const sayText =
      typeof t.say === "function" ? (t.say as (s: ConvState) => string)(state) : t.say;
    const haveSay = !!sayText && sayText.length > 0;
    const sayKey = `s-${cursor}`;
    const saySettled = !haveSay || sayDoneRef.current.has(sayKey);

    // Step 1: ensure say bubble is added (typing).
    if (haveSay) {
      const exists = history.some((h) => h.id === sayKey);
      if (!exists) {
        setHistory((h) => [
          ...h,
          { id: sayKey, role: "system", text: sayText as string, typing: true },
        ]);
        setPhase("typing");
        return;
      }
    }
    if (!saySettled) {
      setPhase("typing");
      return;
    }

    // Step 2: run effect once if present.
    if (t.effect && !effectRanRef.current.has(cursor)) {
      effectRanRef.current.add(cursor);
      setPhase("effect");
      Promise.resolve(t.effect(state))
        .then((patch) => {
          if (patch && typeof patch === "object") setState(patch as ConvState);
          // After effect, re-evaluate by bumping a render cycle (state may change).
          // Continue to next sub-step (ask / form / cta / advance) in the next pass.
          setPhase((p) => (p === "effect" ? "typing" : p));
        })
        .catch((e) => setError(`${(e as Error).message ?? e}`));
      return;
    }

    // Step 3: ask / form / cta
    if (t.ask) {
      setPhase("asking");
      return;
    }
    if (t.form) {
      setPhase("form");
      return;
    }
    if (t.cta) {
      setPhase("cta");
      return;
    }
    // Nothing else to wait for — auto-advance.
    setCursor((c) => c + 1);
    setPhase("typing");
  }, [currentTurn, cursor, state, history, setState]);

  const markTypingDone = useCallback((id: string) => {
    sayDoneRef.current.add(id);
    setHistory((h) => h.map((it) => (it.id === id ? { ...it, typing: false } : it)));
    setPhase((p) => (p === "typing" ? "typing" : p)); // trigger re-eval via state change
    // bump cursor's evaluation
    setCursor((c) => c);
  }, []);

  const submit = useCallback(
    (input?: string) => {
      if (!currentTurn || !currentTurn.ask) return;
      const value = (input ?? "").trim();
      const field = currentTurn.field || currentTurn.id;
      const err = currentTurn.ask.validate?.(value, state) ?? null;
      if (err) {
        setError(err);
        return;
      }
      setError("");
      const userKey = `u-${cursor}`;
      setHistory((h) => [
        ...h,
        { id: userKey, role: "user", text: value, typing: false },
      ]);
      setState({ [field]: value });
      setCursor((c) => c + 1);
      setPhase("typing");
    },
    [currentTurn, cursor, state, setState]
  );

  const finishForm = useCallback(
    (summary?: string) => {
      if (!currentTurn || !currentTurn.form) return;
      if (summary) {
        const userKey = `u-${cursor}`;
        setHistory((h) => [
          ...h,
          { id: userKey, role: "user", text: summary, typing: false },
        ]);
      }
      setCursor((c) => c + 1);
      setPhase("typing");
    },
    [currentTurn, cursor]
  );

  const proceedCTA = useCallback(() => {
    if (!currentTurn || !currentTurn.cta) return;
    setCursor((c) => c + 1);
    setPhase("typing");
  }, [currentTurn]);

  return useMemo(
    () => ({
      history,
      state,
      setState,
      currentTurn,
      phase,
      awaitingAsk: phase === "asking",
      awaitingForm: phase === "form",
      awaitingCTA: phase === "cta",
      submit,
      finishForm,
      proceedCTA,
      markTypingDone,
      error,
    }),
    [
      history,
      state,
      setState,
      currentTurn,
      phase,
      submit,
      finishForm,
      proceedCTA,
      markTypingDone,
      error,
    ]
  );
};

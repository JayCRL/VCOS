import { useEffect, useRef, useState } from "react";

interface Props {
  text: string;
  speed?: number; // ms per grapheme
  onDone?: () => void;
  skip?: boolean;
}

const splitGraphemes = (s: string): string[] => {
  if (typeof Intl !== "undefined" && (Intl as any).Segmenter) {
    const seg = new (Intl as any).Segmenter("zh", { granularity: "grapheme" });
    return [...seg.segment(s)].map((x: any) => x.segment as string);
  }
  return Array.from(s);
};

export const Typewriter = ({ text, speed = 22, onDone, skip }: Props) => {
  const [shown, setShown] = useState(0);
  const charsRef = useRef<string[]>(splitGraphemes(text));
  const timerRef = useRef<number | null>(null);
  const lastTextRef = useRef(text);

  useEffect(() => {
    if (lastTextRef.current !== text) {
      lastTextRef.current = text;
      charsRef.current = splitGraphemes(text);
      setShown(0);
    }
  }, [text]);

  useEffect(() => {
    if (skip) {
      setShown(charsRef.current.length);
      onDone?.();
      return;
    }
    if (shown >= charsRef.current.length) {
      onDone?.();
      return;
    }
    timerRef.current = window.setTimeout(() => {
      setShown((s) => Math.min(s + 1, charsRef.current.length));
    }, speed);
    return () => {
      if (timerRef.current != null) window.clearTimeout(timerRef.current);
    };
  }, [shown, speed, onDone, skip]);

  const visible = charsRef.current.slice(0, shown).join("");
  const done = shown >= charsRef.current.length;
  return (
    <span>
      {visible}
      {!done && <span className="typewriter-cursor">▍</span>}
    </span>
  );
};

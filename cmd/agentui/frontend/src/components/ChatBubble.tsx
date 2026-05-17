import { motion } from "framer-motion";
import { ReactNode } from "react";
import { Typewriter } from "./Typewriter";

interface Props {
  role: "system" | "user";
  text?: string;
  children?: ReactNode;
  typewriter?: boolean;
  onTyped?: () => void;
}

export const ChatBubble = ({
  role,
  text,
  children,
  typewriter,
  onTyped,
}: Props) => {
  const isUser = role === "user";
  return (
    <motion.div
      className={`bubble-row ${isUser ? "bubble-row-user" : "bubble-row-system"}`}
      initial={{ opacity: 0, y: 8, scale: 0.98 }}
      animate={{ opacity: 1, y: 0, scale: 1 }}
      transition={{ duration: 0.28, ease: [0.22, 1, 0.36, 1] }}
    >
      {!isUser && <div className="bubble-avatar" />}
      <div className={`bubble ${isUser ? "bubble-user" : "bubble-system"}`}>
        {typewriter && text ? (
          <Typewriter text={text} onDone={onTyped} />
        ) : (
          children ?? text
        )}
      </div>
    </motion.div>
  );
};

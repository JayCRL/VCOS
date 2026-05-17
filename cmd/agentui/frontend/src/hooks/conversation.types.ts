import type { ReactNode } from "react";

export type ConvState = Record<string, unknown>;

export interface ConvTurnContext {
  state: ConvState;
  setState: (patch: ConvState) => void;
  done: (summary?: string) => void;
}

export interface ConvTurn {
  id: string;
  // 系统消息(可读 state 动态生成)。若返回空串则跳过 say 步骤。
  say?: string | ((s: ConvState) => string);
  // 文本输入。提交后写入 state[field || id]。
  ask?: {
    placeholder: string;
    multiline?: boolean;
    initialFromState?: string;
    validate?: (v: string, s: ConvState) => string | null;
    submitLabel?: string;
  };
  field?: string;
  // 嵌入控件(stage 主区);自带 done 回调,把 user-summary 加入对话历史。
  form?: (ctx: ConvTurnContext) => ReactNode;
  // 异步副作用,可写 state(通过返回的 patch)。
  effect?: (s: ConvState) => Promise<ConvState | void> | ConvState | void;
  // 末尾让用户点继续才往下走。
  cta?: string;
}

export type Script = (initial: ConvState) => ConvTurn[];

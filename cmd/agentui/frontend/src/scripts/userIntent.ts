import type { ConvTurn } from "../hooks/conversation.types";

export const userIntentTurns: ConvTurn[] = [
  {
    id: "greet",
    say: "嗨,我是 AgentOS。我们这次要做点什么?用一句话告诉我目标就好。",
  },
  {
    id: "goal",
    ask: {
      placeholder: "例如:把这个 Go 项目的认证模块重构成 OAuth2",
      multiline: true,
      validate: (v) => (v.trim().length < 3 ? "至少写一句话哈" : null),
    },
  },
  {
    id: "ack",
    say: (s) => `好的,「${(s.goal as string) ?? ""}」我记下了。`,
  },
  {
    id: "save",
    effect: async (s) => {
      await window.go.main.App.SubmitUserIntent(
        s.sid as string,
        s.goal as string
      );
      return { saved: true };
    },
  },
  {
    id: "ready",
    say: "准备好接着聊就点继续 — 下一步我会去探测项目结构。",
    cta: "继续 →",
  },
];

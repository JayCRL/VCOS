import type { ConvTurn } from "../hooks/conversation.types";
import type { DecisionStyle } from "../types/bindings";

const OPTIONS: {
  style: DecisionStyle;
  emoji: string;
  title: string;
  hint: string;
}[] = [
  {
    style: "step-by-step",
    emoji: "👋",
    title: "分步实现",
    hint: "每完成一阶段都暂停等我确认。最稳,适合首次合作或高风险改动。",
  },
  {
    style: "hybrid",
    emoji: "⚖️",
    title: "混合",
    hint: "只在写文件 / shell / 装依赖等高风险动作前来问我。日常默认。",
  },
  {
    style: "autonomous",
    emoji: "🚀",
    title: "全自动",
    hint: "全程不打扰,任务结束或失败时再推桌面通知。",
  },
];

export const decisionStyleTurns: ConvTurn[] = [
  {
    id: "kickoff",
    say:
      "最后一步 — 在执行期间你希望我多频繁来打扰你?这个选择会写入长期记忆,后续会话默认沿用。",
  },
  {
    id: "pick",
    form: ({ state, setState, done }) => {
      const picked = (state.style as DecisionStyle | undefined) ?? "hybrid";
      return (
        <div className="style-grid-v2">
          {OPTIONS.map((o) => (
            <button
              key={o.style}
              className="style-card-v2"
              data-active={picked === o.style}
              onClick={() => {
                setState({ style: o.style });
                done(`选了「${o.title}」`);
              }}
            >
              <div className="style-card-v2-icon">{o.emoji}</div>
              <div className="style-card-v2-title">{o.title}</div>
              <div className="style-card-v2-hint">{o.hint}</div>
            </button>
          ))}
        </div>
      );
    },
  },
  {
    id: "save",
    effect: async (s) => {
      const st = (s.style as DecisionStyle) ?? "hybrid";
      await window.go.main.App.SubmitDecisionStyle(s.sid as string, st);
      return { saved: true };
    },
  },
  {
    id: "ready",
    say: (s) => {
      const st = (s.style as DecisionStyle) ?? "hybrid";
      const map: Record<DecisionStyle, string> = {
        "step-by-step": "好的,分步走 — 每一步都会等你点头。",
        hybrid: "好的,混合策略 — 高风险动作前会找你确认。",
        autonomous: "好的,全自动 — 我会安静地把活做完,完成时叫你。",
      };
      return map[st];
    },
    cta: "开始执行 ▶︎",
  },
];

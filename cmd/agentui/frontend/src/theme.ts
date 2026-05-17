// 设计 token。同时在 styles.css 里以 CSS 变量声明一份。
// 这里的常量是给 framer-motion 等 JS 侧用的(动画曲线、时长)。

export const theme = {
  bg: {
    app: "#fafafb",
    surface: "#ffffff",
    elevated: "rgba(255,255,255,0.72)",
    appGradient:
      "radial-gradient(1200px 600px at 80% -10%, #f0eaff 0%, transparent 60%), radial-gradient(900px 500px at -10% 110%, #ffeaf6 0%, transparent 60%), #fafafb",
  },
  text: {
    primary: "#0e0f15",
    secondary: "#6b7185",
    tertiary: "#9aa0b2",
    inverted: "#ffffff",
  },
  accent: {
    primary: "#6366f1",
    secondary: "#a855f7",
    tertiary: "#ec4899",
    gradient:
      "linear-gradient(135deg, #6366f1 0%, #a855f7 50%, #ec4899 100%)",
    soft:
      "linear-gradient(135deg, #eef2ff 0%, #faf5ff 50%, #fdf2f8 100%)",
    hairlineGradient:
      "linear-gradient(90deg, #6366f1 0%, #a855f7 50%, #ec4899 100%)",
  },
  border: {
    hair: "rgba(15,17,21,0.06)",
    medium: "rgba(15,17,21,0.10)",
    accent: "rgba(99,102,241,0.32)",
  },
  shadow: {
    soft: "0 1px 2px rgba(15,17,21,0.04), 0 8px 24px rgba(15,17,21,0.06)",
    float:
      "0 4px 16px rgba(99,102,241,0.10), 0 24px 64px rgba(15,17,21,0.10)",
    glow: "0 0 0 4px rgba(99,102,241,0.12)",
  },
  radius: { sm: 8, md: 12, lg: 20, xl: 28, pill: 999 },
  motion: {
    // 平滑入场曲线(easeOutExpo 系)
    fadeEase: [0.22, 1, 0.36, 1] as [number, number, number, number],
    springSoft: { type: "spring" as const, stiffness: 260, damping: 28 },
    springQuick: { type: "spring" as const, stiffness: 340, damping: 26 },
    durationShort: 0.24,
    durationMed: 0.36,
    durationLong: 0.6,
  },
  layout: {
    headerHeight: 64,
    stepperHeight: 56,
    footerHeight: 76,
    contentPad: 32,
  },
} as const;

export type Theme = typeof theme;

import type { TemplatePreviewProps, TemplateSpec } from "./types";
import { cssVars } from "./types";

const Preview = ({ accent, accentPrimary, fontFamily, style }: TemplatePreviewProps) => (
  <div className="tpl-root" style={{ ...cssVars(accent, accentPrimary, fontFamily), ...style }}>
    <aside className="tpl-sidebar">
      <div className="tpl-logo" />
      <nav className="tpl-nav">
        <div className="tpl-nav-item active">概览</div>
        <div className="tpl-nav-item">项目</div>
        <div className="tpl-nav-item">任务</div>
        <div className="tpl-nav-item">分析</div>
        <div className="tpl-nav-item">设置</div>
      </nav>
    </aside>
    <section className="tpl-content">
      <div className="tpl-topbar">
        <h2 className="tpl-h1">Dashboard</h2>
        <button className="tpl-cta">+ New project</button>
      </div>
      <div className="tpl-kpis">
        <div className="tpl-kpi">
          <span className="tpl-kpi-label">Active</span>
          <span className="tpl-kpi-num">2,841</span>
          <span className="tpl-kpi-delta">+12.4%</span>
        </div>
        <div className="tpl-kpi">
          <span className="tpl-kpi-label">Revenue</span>
          <span className="tpl-kpi-num">¥48.2k</span>
          <span className="tpl-kpi-delta">+3.8%</span>
        </div>
        <div className="tpl-kpi">
          <span className="tpl-kpi-label">Pending</span>
          <span className="tpl-kpi-num">17</span>
          <span className="tpl-kpi-delta" style={{ color: "#dc2626" }}>−5</span>
        </div>
      </div>
      <div className="tpl-chart">
        <div style={{ display: "flex", justifyContent: "space-between", marginBottom: 8 }}>
          <h3 className="tpl-h2">Weekly activity</h3>
          <span style={{ fontSize: 11, color: "#9aa0b2" }}>7d</span>
        </div>
        <svg viewBox="0 0 320 110" style={{ width: "100%", height: "100%" }}>
          <defs>
            <linearGradient id="grad-dashboard" x1="0" x2="1" y1="0" y2="1">
              <stop offset="0%" stopColor="#6366f1" />
              <stop offset="50%" stopColor="#a855f7" />
              <stop offset="100%" stopColor="#ec4899" />
            </linearGradient>
          </defs>
          <path
            d="M0,80 C40,60 80,90 120,55 C160,30 200,70 240,40 C280,18 300,55 320,30"
            fill="none"
            stroke="url(#grad-dashboard)"
            strokeWidth="2.5"
            strokeLinecap="round"
          />
          <path
            d="M0,80 C40,60 80,90 120,55 C160,30 200,70 240,40 C280,18 300,55 320,30 L320,110 L0,110 Z"
            fill="url(#grad-dashboard)"
            opacity="0.12"
          />
        </svg>
      </div>
    </section>
  </div>
);

export const dashboardTemplate: TemplateSpec = {
  id: "dashboard",
  name: "Dashboard",
  description: "侧栏 · 顶栏 · 三 KPI · 趋势图",
  Preview,
  components: [
    { id: "sidebar", name: "Sidebar 导航", kind: "nav", description: "5 项一级菜单" },
    { id: "topbar", name: "Topbar 顶栏", kind: "nav", description: "标题 + 新建按钮" },
    { id: "kpi-active", name: "KPI · Active", kind: "card", description: "活跃用户数" },
    { id: "kpi-revenue", name: "KPI · Revenue", kind: "card", description: "营收总额" },
    { id: "kpi-pending", name: "KPI · Pending", kind: "card", description: "待办数" },
    { id: "chart-weekly", name: "Weekly Chart", kind: "chart", description: "周活动趋势图" },
  ],
};

import type { TemplatePreviewProps, TemplateSpec } from "./types";
import { cssVars } from "./types";

const rows = [
  ["Aurora 项目", "Anna Cohen", "进行中", "2 天前"],
  ["客户支持工单 #482", "Tom Mei", "已解决", "6 小时前"],
  ["Q3 财报分析", "Li Wei", "待审核", "1 天前"],
  ["Onboarding 重写", "Jules R.", "进行中", "3 小时前"],
  ["品牌指南 v3", "Anna Cohen", "已解决", "昨天"],
  ["合作伙伴沟通", "Tom Mei", "进行中", "今天"],
];

const statusColor = (s: string, accentPrimary: string) => {
  if (s === "已解决") return { bg: "rgba(22,163,74,0.12)", fg: "#16a34a" };
  if (s === "待审核") return { bg: "rgba(234,179,8,0.16)", fg: "#a16207" };
  return { bg: "rgba(99,102,241,0.12)", fg: accentPrimary };
};

const Preview = ({ accent, accentPrimary, fontFamily, style }: TemplatePreviewProps) => (
  <div className="tpl-root" style={{ ...cssVars(accent, accentPrimary, fontFamily), ...style }}>
    <section className="tpl-content">
      <div className="tpl-topbar">
        <h2 className="tpl-h1">任务列表</h2>
        <button className="tpl-cta">+ 新任务</button>
      </div>
      <div className="tpl-filter-bar">
        <input className="input" placeholder="🔍  搜索任务名 / 负责人 ..." />
        <button className="tpl-cta-soft">状态 ▾</button>
        <button className="tpl-cta-soft">时间 ▾</button>
      </div>
      <div className="tpl-table">
        <div className="tpl-table-head">
          <span>任务</span>
          <span>负责人</span>
          <span>状态</span>
          <span>更新</span>
          <span></span>
        </div>
        {rows.map(([name, owner, status, when], i) => {
          const c = statusColor(status, accentPrimary);
          return (
            <div className="tpl-table-row" key={i}>
              <span style={{ fontWeight: 500 }}>{name}</span>
              <span style={{ color: "#6b7185" }}>{owner}</span>
              <span>
                <span className="tpl-tag" style={{ background: c.bg, color: c.fg }}>
                  {status}
                </span>
              </span>
              <span style={{ color: "#9aa0b2" }}>{when}</span>
              <span style={{ color: "#9aa0b2", textAlign: "right" }}>···</span>
            </div>
          );
        })}
      </div>
    </section>
  </div>
);

export const tableTemplate: TemplateSpec = {
  id: "table",
  name: "数据表格",
  description: "搜索 · 筛选 · 行项目 · 状态徽章",
  Preview,
  components: [
    { id: "header", name: "标题 + 新建", kind: "nav", description: "页面顶栏" },
    { id: "search", name: "搜索框", kind: "input", description: "实时过滤" },
    { id: "filter-status", name: "状态筛选", kind: "button", description: "下拉" },
    { id: "filter-time", name: "时间筛选", kind: "button", description: "下拉" },
    { id: "table", name: "表格", kind: "list", description: "5 列任务列表" },
    { id: "status-tag", name: "状态徽章", kind: "card", description: "三色胶囊" },
  ],
};

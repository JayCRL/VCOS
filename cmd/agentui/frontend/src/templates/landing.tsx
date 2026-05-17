import type { TemplatePreviewProps, TemplateSpec } from "./types";
import { cssVars } from "./types";

const Preview = ({ accent, accentPrimary, fontFamily, style }: TemplatePreviewProps) => (
  <div className="tpl-root" style={{ ...cssVars(accent, accentPrimary, fontFamily), ...style, background: "#fff" }}>
    <div className="tpl-landing">
      <div className="tpl-hero">
        <span
          style={{
            fontSize: 11,
            padding: "4px 10px",
            border: "1px solid rgba(15,17,21,0.08)",
            borderRadius: 999,
            color: "#6b7185",
          }}
        >
          v2.0 · 全新发布
        </span>
        <h1>
          一个为团队设计的
          <br />
          <span className="accent">协作工作台</span>
        </h1>
        <p>
          把项目、任务、文档、聊天放到一个地方。十秒上手,无需培训,你的团队会感谢你。
        </p>
        <div style={{ display: "flex", gap: 10 }}>
          <button className="tpl-cta" style={{ padding: "10px 18px" }}>开始免费试用</button>
          <button className="tpl-cta-soft" style={{ padding: "10px 18px" }}>看 60 秒演示</button>
        </div>
      </div>

      <div className="tpl-features">
        <div className="tpl-feature">
          <div className="icon" />
          <div className="title">快速搭建</div>
          <div className="desc">拖拽组件,模板起步,5 分钟搭出生产可用页面。</div>
        </div>
        <div className="tpl-feature">
          <div className="icon" />
          <div className="title">无缝协作</div>
          <div className="desc">实时光标、评论、版本回滚,全员同步在一个画布。</div>
        </div>
        <div className="tpl-feature">
          <div className="icon" />
          <div className="title">数据安全</div>
          <div className="desc">SSO · 审计日志 · GDPR/SOC2 合规,企业级守护。</div>
        </div>
      </div>
    </div>
  </div>
);

export const landingTemplate: TemplateSpec = {
  id: "landing",
  name: "落地页",
  description: "Hero + 副标 + 双 CTA · 三栏特性",
  Preview,
  components: [
    { id: "tag", name: "版本徽章", kind: "text", description: "胶囊形 'v2.0 · 全新发布'" },
    { id: "headline", name: "主标题", kind: "text", description: "两行,关键词渐变" },
    { id: "subhead", name: "副标题", kind: "text", description: "简短价值描述" },
    { id: "cta-primary", name: "主 CTA", kind: "button", description: "开始免费试用" },
    { id: "cta-secondary", name: "次 CTA", kind: "button", description: "看演示" },
    { id: "feature-1", name: "特性 1", kind: "card", description: "快速搭建" },
    { id: "feature-2", name: "特性 2", kind: "card", description: "无缝协作" },
    { id: "feature-3", name: "特性 3", kind: "card", description: "数据安全" },
  ],
};

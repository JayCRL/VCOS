import type { TemplatePreviewProps, TemplateSpec } from "./types";
import { cssVars } from "./types";

const Preview = ({ accent, accentPrimary, fontFamily, style }: TemplatePreviewProps) => (
  <div className="tpl-root" style={{ ...cssVars(accent, accentPrimary, fontFamily), ...style }}>
    <div className="tpl-center">
      <div className="tpl-auth-card">
        <div className="logo" />
        <div className="tpl-auth-title">欢迎回来</div>
        <div className="tpl-auth-sub">用邮箱继续登录,或选择社交账号。</div>
        <span className="tpl-field-label">邮箱</span>
        <input className="tpl-field" placeholder="you@example.com" />
        <span className="tpl-field-label">密码</span>
        <input className="tpl-field" type="password" placeholder="••••••••" />
        <button className="tpl-cta" style={{ marginTop: 4, padding: "10px 16px" }}>
          登录
        </button>
        <button className="tpl-cta-soft">用 Google 账号继续</button>
        <div style={{ fontSize: 12, color: "#9aa0b2", textAlign: "center", marginTop: 6 }}>
          还没有账号? <a style={{ color: accentPrimary }}>注册</a>
        </div>
      </div>
    </div>
  </div>
);

export const authTemplate: TemplateSpec = {
  id: "auth",
  name: "登录",
  description: "居中渐变卡片 · 邮箱密码 + 社交",
  Preview,
  components: [
    { id: "logo", name: "Logo", kind: "image", description: "渐变方块 logo" },
    { id: "title", name: "标题", kind: "text", description: "欢迎语" },
    { id: "email-field", name: "邮箱输入", kind: "input", description: "邮箱字段" },
    { id: "password-field", name: "密码输入", kind: "input", description: "密码字段" },
    { id: "login-btn", name: "登录按钮", kind: "button", description: "主 CTA" },
    { id: "google-btn", name: "Google 按钮", kind: "button", description: "社交登录" },
    { id: "signup-link", name: "注册链接", kind: "text", description: "底部跳转" },
  ],
};

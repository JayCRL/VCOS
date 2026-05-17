import type { TemplatePreviewProps, TemplateSpec } from "./types";
import { cssVars } from "./types";

const Preview = ({ accent, accentPrimary, fontFamily, style }: TemplatePreviewProps) => (
  <div className="tpl-root" style={{ ...cssVars(accent, accentPrimary, fontFamily), ...style }}>
    <aside className="tpl-sidebar">
      <div className="tpl-logo" />
      <div className="tpl-chat-list">
        <div className="item active">
          <div className="name">Anna Cohen</div>
          <div className="preview">看到我刚才发的截图…</div>
        </div>
        <div className="item">
          <div className="name">Design Team</div>
          <div className="preview">@all 周三 review 时间</div>
        </div>
        <div className="item">
          <div className="name">Tom Mei</div>
          <div className="preview">那条 bug 我修了</div>
        </div>
        <div className="item">
          <div className="name">客户支持群</div>
          <div className="preview">需要个新模板</div>
        </div>
      </div>
    </aside>
    <section className="tpl-content" style={{ padding: 0 }}>
      <div className="tpl-chat-area">
        <div className="tpl-chat-msgs">
          <div className="tpl-chat-msg them">嘿 — 这张图你怎么看?</div>
          <div className="tpl-chat-msg them">需要再换一种配色吗?</div>
          <div className="tpl-chat-msg me">我觉得渐变这版可以,留住</div>
          <div className="tpl-chat-msg me">字体加大一档试试?</div>
          <div className="tpl-chat-msg them">好,我改 16 → 18</div>
        </div>
        <div className="tpl-chat-input">
          <input placeholder="输入消息…" />
          <button className="tpl-cta">发送</button>
        </div>
      </div>
    </section>
  </div>
);

export const chatTemplate: TemplateSpec = {
  id: "chat",
  name: "聊天",
  description: "会话列表 + 消息流 + 输入条",
  Preview,
  components: [
    { id: "conv-list", name: "会话列表", kind: "list", description: "近期对话" },
    { id: "msg-stream", name: "消息流", kind: "list", description: "气泡 me/them" },
    { id: "input-bar", name: "输入条", kind: "input", description: "文本 + 发送" },
  ],
};

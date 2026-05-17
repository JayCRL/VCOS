package llm

import (
	"context"
	"fmt"
	"strings"
)

// UISuggestion is what SuggestUI returns: a template id + accent + font name
// + optional component renames. Used to pre-fill Stage 3 so the user starts
// from an AI-curated baseline rather than a blank template grid.
type UISuggestion struct {
	TemplateID        string             `json:"templateId"`
	Accent            string             `json:"accent"`
	Font              string             `json:"font"`
	Rationale         string             `json:"rationale,omitempty"`
	ComponentRenames  []ComponentRename  `json:"componentRenames,omitempty"`
}

type ComponentRename struct {
	ComponentID string `json:"componentId"`
	Name        string `json:"name"`
}

// SuggestUIRequest is the context the model needs to pick a template.
type SuggestUIRequest struct {
	UserIntent      string
	ProjectPrompt   string
	UserNote        string
	Language        string
	Summary         string
	ModuleNames     []string
}

const suggestUISystem = `你是一个 UI 设计助手。基于用户的项目意图,从下列五套预制模板里挑一个最合适的起点,并给出推荐主色和字体。同时可以建议把模板里某些组件改个更贴近业务的名字。

模板:
- auth — 居中卡片,邮箱密码 + 社交登录,适合需要"登录/注册/找回密码"的场景
- chat — 会话列表 + 消息流 + 输入条,适合 IM、客服、AI 对话、协作工具
- dashboard — 左侧导航 + 顶部 KPI + 图表区,适合后台、数据可视化、监控面板
- landing — Hero + 副标 + 双 CTA + 三栏特性,适合官网/落地页/产品介绍
- table — 数据表格 + 筛选 + 操作栏,适合 CRUD、订单、库存、用户列表

主色(只能选这五个):Iris(紫粉)/ Sunset(橙红)/ Ocean(蓝青)/ Forest(绿松)/ Mono(灰黑)
字体:现代 / 等宽 / 衬线

输出严格遵循 suggest_ui 工具的字段。componentRenames 是可选的——如果你觉得模板里某个组件的名字不够贴近业务(比如 auth 的 "标题" 在协作工具语境下可以改成 "团队登录"),就重命名,否则留空数组。
`

// SuggestUI picks a template + accent + font based on the user's intent.
func (c *Client) SuggestUI(ctx context.Context, req SuggestUIRequest) (*UISuggestion, error) {
	properties := map[string]any{
		"templateId": map[string]any{
			"type": "string",
			"enum": []string{"auth", "chat", "dashboard", "landing", "table"},
		},
		"accent": map[string]any{
			"type": "string",
			"enum": []string{"Iris", "Sunset", "Ocean", "Forest", "Mono"},
		},
		"font": map[string]any{
			"type": "string",
			"enum": []string{"现代", "等宽", "衬线"},
		},
		"rationale": map[string]any{"type": "string"},
		"componentRenames": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"componentId": map[string]any{"type": "string"},
					"name":        map[string]any{"type": "string"},
				},
				"required": []string{"componentId", "name"},
			},
		},
	}
	var out UISuggestion
	if err := c.toolCall(ctx, suggestUISystem, buildSuggestUIBlock(req), "suggest_ui",
		properties, []string{"templateId", "accent", "font"}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func buildSuggestUIBlock(req SuggestUIRequest) string {
	var sb strings.Builder
	sb.WriteString("用户的整体意图:\n")
	if strings.TrimSpace(req.UserIntent) != "" {
		sb.WriteString(req.UserIntent)
	} else {
		sb.WriteString("(未提供)")
	}
	if strings.TrimSpace(req.ProjectPrompt) != "" {
		fmt.Fprintf(&sb, "\n\n项目信息:%s", req.ProjectPrompt)
	}
	if strings.TrimSpace(req.UserNote) != "" {
		fmt.Fprintf(&sb, "\n用户补充:%s", req.UserNote)
	}
	if strings.TrimSpace(req.Language) != "" {
		fmt.Fprintf(&sb, "\n项目语言:%s", req.Language)
	}
	if strings.TrimSpace(req.Summary) != "" {
		fmt.Fprintf(&sb, "\n项目摘要:%s", req.Summary)
	}
	if len(req.ModuleNames) > 0 {
		fmt.Fprintf(&sb, "\n现有模块:%s", strings.Join(req.ModuleNames, ", "))
	}
	sb.WriteString("\n\n请直接调用 suggest_ui。")
	return sb.String()
}

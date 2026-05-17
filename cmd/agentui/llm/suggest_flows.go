package llm

import (
	"context"
	"fmt"
	"strings"
)

// FlowDraft is a single AI-suggested interaction. Fields mirror
// wizard.InteractionFlow so the GUI can drop them straight in.
type FlowDraft struct {
	Trigger     string `json:"trigger"`
	Action      string `json:"action"`
	Description string `json:"description,omitempty"`
	ComponentID string `json:"componentId,omitempty"`
}

type FlowDraftSet struct {
	Flows []FlowDraft `json:"flows"`
	Notes string      `json:"notes,omitempty"`
}

// SuggestFlowsRequest is the context the model needs to generate flows.
type SuggestFlowsRequest struct {
	UserIntent    string
	TemplateName  string
	Components    []Component
}

const suggestFlowsSystem = `你是一个交互设计助手。基于用户的意图和已选的 UI 组件,产出 5-8 条最关键的交互流(trigger → action)。

要求:
- 覆盖主路径(登录/提交/导航/搜索/删除等)+ 边角(错误处理/空状态)
- trigger 用自然语言,如 "点击登录按钮" / "输入框获得焦点" / "未登录访问受保护路由"
- action 用 "跳转到 /X" / "弹出 X 模态框" / "提交表单到 /api/X" / "高亮错误信息" 这种简短指令
- componentId 引用用户提供的组件 id;如果某条 flow 是全局规则(如 "未登录跳登录页"),componentId 留空
- description 是可选的简短补充,只在 trigger/action 不够明确时填

整体 notes 字段写 1-2 条"全局约束"(比如 "所有按钮 hover 有放大动效")。如果没有,留空。

输出严格走 suggest_flows 工具。`

// SuggestFlows generates a starter set of interaction flows.
func (c *Client) SuggestFlows(ctx context.Context, req SuggestFlowsRequest) (*FlowDraftSet, error) {
	properties := map[string]any{
		"flows": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"trigger":     map[string]any{"type": "string"},
					"action":      map[string]any{"type": "string"},
					"description": map[string]any{"type": "string"},
					"componentId": map[string]any{"type": "string"},
				},
				"required": []string{"trigger", "action"},
			},
		},
		"notes": map[string]any{"type": "string"},
	}
	var out FlowDraftSet
	if err := c.toolCall(ctx, suggestFlowsSystem, buildSuggestFlowsBlock(req), "suggest_flows",
		properties, []string{"flows"}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func buildSuggestFlowsBlock(req SuggestFlowsRequest) string {
	var sb strings.Builder
	sb.WriteString("用户意图:\n")
	if strings.TrimSpace(req.UserIntent) != "" {
		sb.WriteString(req.UserIntent)
	} else {
		sb.WriteString("(未提供)")
	}
	if strings.TrimSpace(req.TemplateName) != "" {
		fmt.Fprintf(&sb, "\n\n已选模板:%s", req.TemplateName)
	}
	sb.WriteString("\n\nUI 组件:\n")
	if len(req.Components) == 0 {
		sb.WriteString("(无)\n")
	}
	for _, c := range req.Components {
		fmt.Fprintf(&sb, "- id=%s | %s (%s)", c.ID, c.Name, c.Kind)
		if c.Description != "" {
			fmt.Fprintf(&sb, " — %s", c.Description)
		}
		sb.WriteString("\n")
	}
	sb.WriteString("\n请直接调用 suggest_flows,5-8 条 flows。")
	return sb.String()
}

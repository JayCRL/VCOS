package draft

import (
	"fmt"
	"strings"

	"mobilevc/desktop/wizard"
)

const draftSystem = `你是一个资深架构师。基于以下用户的需求与项目背景,起草一份精炼的技术方案(Markdown 格式)。

要求:
- 控制在 600 字以内,不要废话
- 用一级标题分段:## 目标 / ## 主要步骤 / ## 风险 / ## 验证
- "主要步骤"用编号列表,每步包含简短的"输入 / 输出 / 验证"
- "风险"用 - bullet
- 不要寒暄、不要重复需求、不要 meta 描述,直接给方案
- 不要把整个回复包在代码块里
`

// BuildPrompt assembles the drafting prompt from a wizard snapshot.
// Empty sections are skipped so Claude isn't fed placeholders.
func BuildPrompt(snap wizard.Snapshot) string {
	var sb strings.Builder
	sb.WriteString(draftSystem)

	sb.WriteString("\n\n## 用户的整体意图\n")
	if snap.UserIntent != nil && strings.TrimSpace(snap.UserIntent.Text) != "" {
		sb.WriteString(snap.UserIntent.Text)
	} else {
		sb.WriteString("(未提供)")
	}

	if snap.ProjectIntent != nil {
		sb.WriteString("\n\n## 项目背景\n")
		if p := strings.TrimSpace(snap.ProjectIntent.Prompt); p != "" {
			sb.WriteString("- Prompt: ")
			sb.WriteString(p)
			sb.WriteString("\n")
		}
		if n := strings.TrimSpace(snap.ProjectIntent.UserNote); n != "" {
			sb.WriteString("- 用户补充: ")
			sb.WriteString(n)
			sb.WriteString("\n")
		}
		if sem := snap.ProjectIntent.Semantic; sem != nil {
			if sem.Language != "" {
				fmt.Fprintf(&sb, "- 语言: %s\n", sem.Language)
			}
			if sem.Summary != "" {
				fmt.Fprintf(&sb, "- 摘要: %s\n", sem.Summary)
			}
			if len(sem.Modules) > 0 {
				names := make([]string, 0, len(sem.Modules))
				for _, m := range sem.Modules {
					names = append(names, m.Name)
				}
				fmt.Fprintf(&sb, "- 模块: %s\n", strings.Join(names, ", "))
			}
			if len(sem.EntryPoints) > 0 {
				eps := make([]string, 0, len(sem.EntryPoints))
				for _, e := range sem.EntryPoints {
					eps = append(eps, e.Path)
				}
				fmt.Fprintf(&sb, "- 入口: %s\n", strings.Join(eps, ", "))
			}
		}
	}

	if len(snap.UIComponents) > 0 {
		fmt.Fprintf(&sb, "\n\n## UI 规格(%d 个组件)\n", len(snap.UIComponents))
		for _, c := range snap.UIComponents {
			fmt.Fprintf(&sb, "- %s (%s)", c.Name, c.Kind)
			if d := strings.TrimSpace(c.Description); d != "" {
				fmt.Fprintf(&sb, " — %s", d)
			}
			sb.WriteString("\n")
		}
	}
	if snap.UIPrompt != nil && strings.TrimSpace(snap.UIPrompt.Prompt) != "" {
		sb.WriteString("\n## 渲染映射 Prompt\n")
		sb.WriteString(snap.UIPrompt.Prompt)
		sb.WriteString("\n")
	}
	if snap.InteractionLogic != nil && len(snap.InteractionLogic.Flows) > 0 {
		fmt.Fprintf(&sb, "\n\n## 交互流(%d 条)\n", len(snap.InteractionLogic.Flows))
		for _, f := range snap.InteractionLogic.Flows {
			fmt.Fprintf(&sb, "- %s → %s", f.Trigger, f.Action)
			if f.ComponentID != "" {
				fmt.Fprintf(&sb, " (组件: %s)", f.ComponentID)
			}
			sb.WriteString("\n")
		}
		if n := strings.TrimSpace(snap.InteractionLogic.Notes); n != "" {
			sb.WriteString("- 补充: ")
			sb.WriteString(n)
			sb.WriteString("\n")
		}
	}

	sb.WriteString("\n\n现在请直接输出 Markdown 方案,无需任何前置说明。\n")
	return sb.String()
}

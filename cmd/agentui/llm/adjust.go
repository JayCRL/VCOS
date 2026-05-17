package llm

import (
	"context"
	"fmt"
	"strings"
)

// Patch is a single UI mutation produced by the model.
type Patch struct {
	ComponentID string         `json:"componentId,omitempty"`
	Op          string         `json:"op"` // rename | recolor | add | remove
	Value       map[string]any `json:"value,omitempty"`
}

type PatchSet struct {
	Patches []Patch `json:"patches"`
}

// Component is the lean view passed to the model as context.
type Component struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Kind        string `json:"kind"`
	Description string `json:"description,omitempty"`
}

type AdjustRequest struct {
	UserPrompt   string
	Components   []Component
	Accent       string
	TemplateName string
}

const adjustSystem = `你是一个 UI 设计助手。用户已经选好了一个 React 模板和主色,现在希望对界面做微调。
请基于用户的微调指令,产出一份 apply_patches 调用。每个 patch 描述一项修改:

- op=recolor:替换主色,value = { "accent": "Iris" | "Sunset" | "Ocean" | "Forest" | "Mono" }
  · Iris = 紫粉渐变;Sunset = 橙红;Ocean = 蓝青;Forest = 绿松;Mono = 灰黑
- op=rename:重命名某个已有组件,value = { "name": "新名字" }
- op=add:新增一个组件(只影响后续 prompt 不影响 JSX),value = { "name", "kind", "description" }
- op=remove:删除一个已有组件,只需 componentId

componentId 对应用户提供的现有 components 中的 id。
不要解释,直接调 apply_patches。如果用户的指令完全无法对应到上述四种 op,返回空的 patches: []。
`

// Adjust performs a single apply_patches tool_use turn.
func (c *Client) Adjust(ctx context.Context, req AdjustRequest) ([]Patch, error) {
	if strings.TrimSpace(req.UserPrompt) == "" {
		return nil, fmt.Errorf("empty prompt")
	}
	properties := map[string]any{
		"patches": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"componentId": map[string]any{"type": "string"},
					"op": map[string]any{
						"type": "string",
						"enum": []string{"rename", "recolor", "add", "remove"},
					},
					"value": map[string]any{"type": "object"},
				},
				"required": []string{"op"},
			},
		},
	}
	var pset PatchSet
	if err := c.toolCall(ctx, adjustSystem, buildAdjustUserBlock(req), "apply_patches",
		properties, []string{"patches"}, &pset); err != nil {
		return nil, err
	}
	return pset.Patches, nil
}

func buildAdjustUserBlock(req AdjustRequest) string {
	var sb strings.Builder
	sb.WriteString("当前模板:")
	if req.TemplateName != "" {
		sb.WriteString(req.TemplateName)
	} else {
		sb.WriteString("(未提供)")
	}
	sb.WriteString("\n当前主色:")
	if req.Accent != "" {
		sb.WriteString(req.Accent)
	} else {
		sb.WriteString("(未提供)")
	}
	sb.WriteString("\n现有组件:\n")
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
	sb.WriteString("\n用户指令:\n")
	sb.WriteString(req.UserPrompt)
	sb.WriteString("\n\n请直接调用 apply_patches。")
	return sb.String()
}

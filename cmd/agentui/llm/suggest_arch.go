package llm

import (
	"context"
	"strings"

	"mobilevc/desktop/scan"
)

// ArchSuggestion is a virtual Semantic emitted when the cwd is empty (a
// greenfield project). The GUI renders it through the same ProjectGraph that
// shows real scans, so the user immediately sees what a starting layout
// could look like.
type ArchSuggestion struct {
	Summary     string             `json:"summary"`
	Language    string             `json:"language"`
	EntryPoints []scan.EntryPoint  `json:"entryPoints"`
	Modules     []scan.Module      `json:"modules"`
	Hotspots    []scan.Hotspot     `json:"hotspots,omitempty"`
	Deps        []scan.DepEdge     `json:"deps,omitempty"`
	Rationale   string             `json:"rationale,omitempty"`
}

// ToSemantic converts the suggestion into a scan.Semantic so the existing
// rendering path doesn't need to know the difference.
func (a *ArchSuggestion) ToSemantic() *scan.Semantic {
	if a == nil {
		return nil
	}
	return &scan.Semantic{
		Summary:     a.Summary,
		Language:    a.Language,
		EntryPoints: a.EntryPoints,
		Modules:     a.Modules,
		Hotspots:    a.Hotspots,
		Deps:        a.Deps,
	}
}

const suggestArchSystem = `你是一个软件架构师。用户想做一个新项目(当前目录是空的),请基于他的意图,产出一个"起步架构"——3-6 个核心模块、2-3 个入口点、1-2 个潜在热点(易出问题的关键路径)、模块间主要依赖。

要求:
- summary:一句话项目摘要,30 字以内
- language:推荐技术栈语言("go" / "typescript" / "python" / "rust" / "kotlin/android" 等)
- modules[]:每个模块 name(英文 kebab-case,如 "user-auth")+ path(假想路径 "src/auth")+ responsibility(中文一句话职责)
- entryPoints[]:path("cmd/main.go" / "src/main.tsx" 等)+ purpose(用途)
- hotspots[]:用户最容易出问题的地方(并发、外部 API、状态管理等),path + reason
- deps[]:模块间依赖,from/to 指向上面 modules 的 name,kind = "imports" / "calls"

不要选过度复杂的栈;让用户能 30 分钟跑起来。
输出严格走 suggest_architecture 工具。`

// SuggestArchitecture proposes a starter layout for an empty project.
func (c *Client) SuggestArchitecture(ctx context.Context, userIntent string) (*ArchSuggestion, error) {
	properties := map[string]any{
		"summary":   map[string]any{"type": "string"},
		"language":  map[string]any{"type": "string"},
		"rationale": map[string]any{"type": "string"},
		"entryPoints": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":    map[string]any{"type": "string"},
					"purpose": map[string]any{"type": "string"},
				},
				"required": []string{"path", "purpose"},
			},
		},
		"modules": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"name":           map[string]any{"type": "string"},
					"path":           map[string]any{"type": "string"},
					"responsibility": map[string]any{"type": "string"},
				},
				"required": []string{"name", "path", "responsibility"},
			},
		},
		"hotspots": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"path":   map[string]any{"type": "string"},
					"reason": map[string]any{"type": "string"},
				},
				"required": []string{"path", "reason"},
			},
		},
		"deps": map[string]any{
			"type": "array",
			"items": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"from": map[string]any{"type": "string"},
					"to":   map[string]any{"type": "string"},
					"kind": map[string]any{"type": "string"},
				},
				"required": []string{"from", "to", "kind"},
			},
		},
	}
	userText := "用户意图:\n" + strings.TrimSpace(userIntent) +
		"\n\n请直接调用 suggest_architecture,3-6 个模块。"
	// Architecture payloads can run a bit larger than UI suggestions.
	cli := c.WithMaxTokens(2048)
	var out ArchSuggestion
	if err := cli.toolCall(ctx, suggestArchSystem, userText, "suggest_architecture",
		properties, []string{"summary", "language", "modules", "entryPoints"}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

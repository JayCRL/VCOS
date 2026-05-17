package llm

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// Client wraps an Anthropic client with a fixed model + max_tokens. Reused
// across adjust / suggest_ui / suggest_flows / suggest_arch.
type Client struct {
	api   anthropic.Client
	model anthropic.Model
	max   int64
}

// NewClient builds a Client using the supplied API key. A non-empty baseURL
// overrides the official Anthropic endpoint (useful for proxies / gateways).
func NewClient(apiKey, baseURL string) *Client {
	opts := []option.RequestOption{option.WithAPIKey(apiKey)}
	if strings.TrimSpace(baseURL) != "" {
		opts = append(opts, option.WithBaseURL(baseURL))
	}
	c := anthropic.NewClient(opts...)
	return &Client{
		api:   c,
		model: anthropic.ModelClaudeOpus4_7,
		max:   1024,
	}
}

// WithMaxTokens returns a shallow copy of the client with a different cap.
// Suggest endpoints that emit larger structured payloads can lift this.
func (c *Client) WithMaxTokens(n int64) *Client {
	cp := *c
	cp.max = n
	return &cp
}

// toolCall performs one Anthropic message turn that is forced to invoke a
// single tool. The tool's JSON input is unmarshaled into `out`.
//
// Both arrays-of-objects and plain-object schemas work — pass the JSON schema
// in `properties` / `required` exactly as Anthropic expects.
func (c *Client) toolCall(
	ctx context.Context,
	system, user string,
	toolName string,
	properties map[string]any,
	required []string,
	out any,
) error {
	tool := anthropic.ToolUnionParam{
		OfTool: &anthropic.ToolParam{
			Name: toolName,
			InputSchema: anthropic.ToolInputSchemaParam{
				Properties: properties,
				Required:   required,
			},
		},
	}
	resp, err := c.api.Messages.New(ctx, anthropic.MessageNewParams{
		Model:     c.model,
		MaxTokens: c.max,
		System: []anthropic.TextBlockParam{
			{Text: system},
		},
		Messages: []anthropic.MessageParam{
			anthropic.NewUserMessage(anthropic.NewTextBlock(user)),
		},
		Tools:      []anthropic.ToolUnionParam{tool},
		ToolChoice: anthropic.ToolChoiceUnionParam{OfTool: &anthropic.ToolChoiceToolParam{Name: toolName}},
	})
	if err != nil {
		return fmt.Errorf("anthropic %s: %w", toolName, err)
	}
	for _, block := range resp.Content {
		if block.Type != "tool_use" {
			continue
		}
		t := block.AsToolUse()
		if t.Name != toolName {
			continue
		}
		if err := json.Unmarshal(t.Input, out); err != nil {
			return fmt.Errorf("parse %s input: %w; raw=%s", toolName, err, string(t.Input))
		}
		return nil
	}
	return errors.New("model returned no tool_use block")
}

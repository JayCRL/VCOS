package scan

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
)

// RunSemantic invokes `claude --print` with SemanticPrompt at cwd and parses
// the JSON response into Semantic. The raw output is returned alongside for
// debugging (events / logs).
//
// Output format is "text" rather than "stream-json": the prompt is fixed and
// short, the GUI shows a thinking spinner during the wait, and parsing one
// complete JSON blob is much simpler than reconstructing it from a stream of
// assistant deltas.
func RunSemantic(ctx context.Context, cwd string) (*Semantic, string, error) {
	args := []string{
		"--print",
		"--output-format", "text",
		SemanticPrompt,
	}
	cmd := exec.CommandContext(ctx, "claude", args...)
	cmd.Dir = cwd
	out, err := cmd.CombinedOutput()
	raw := string(out)
	if err != nil {
		return nil, raw, fmt.Errorf("claude -p failed: %w; output: %s", err, snippet(raw, 240))
	}
	jsonStr := extractJSON(raw)
	if jsonStr == "" {
		return nil, raw, fmt.Errorf("no JSON object found in claude output; got %q", snippet(raw, 240))
	}
	var s Semantic
	if err := json.Unmarshal([]byte(jsonStr), &s); err != nil {
		return nil, raw, fmt.Errorf("parse claude JSON: %w; payload was %s", err, snippet(jsonStr, 240))
	}
	return &s, raw, nil
}

// extractJSON pulls the first balanced { ... } block out of s. Claude
// occasionally prefixes its JSON with stray text despite the prompt's
// instructions, so we tolerate that.
func extractJSON(s string) string {
	start := strings.Index(s, "{")
	if start < 0 {
		return ""
	}
	depth := 0
	inStr := false
	escape := false
	for i := start; i < len(s); i++ {
		c := s[i]
		if escape {
			escape = false
			continue
		}
		if c == '\\' {
			escape = true
			continue
		}
		if c == '"' {
			inStr = !inStr
			continue
		}
		if inStr {
			continue
		}
		switch c {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return s[start : i+1]
			}
		}
	}
	return ""
}

func snippet(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

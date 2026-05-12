package kernel

import (
	"strings"
	"time"

	"mobilevc/data"
	"mobilevc/engine"
	"mobilevc/protocol"
	"mobilevc/session"
)

// ── String utilities ──

func firstNonEmptyString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func fallback(value, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

// ── Command detection ──

func commandHead(command string) string {
	fields := strings.Fields(strings.TrimSpace(command))
	if len(fields) == 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(fields[0]))
}

func isAISessionCommandLike(command string) bool {
	head := commandHead(command)
	switch {
	case head == "claude", strings.HasSuffix(head, "/claude"), strings.HasSuffix(head, `\\claude`), head == "claude.exe":
		return true
	case head == "codex", strings.HasSuffix(head, "/codex"), strings.HasSuffix(head, `\\codex`), head == "codex.exe":
		return true
	case head == "gemini", strings.HasSuffix(head, "/gemini"), strings.HasSuffix(head, `\\gemini`), head == "gemini.exe":
		return true
	default:
		return false
	}
}

func isCodexRuntime(runtime data.SessionRuntime) bool {
	return strings.EqualFold(strings.TrimSpace(runtime.Engine), "codex") ||
		isCodexCommandHead(strings.TrimSpace(runtime.Command))
}

func isClaudeRuntime(runtime data.SessionRuntime) bool {
	return strings.EqualFold(strings.TrimSpace(runtime.Engine), "claude") ||
		isClaudeCommandHead(strings.TrimSpace(runtime.Command))
}

func isCodexCommandHead(command string) bool {
	head := commandHead(command)
	return head == "codex" || strings.HasSuffix(head, "/codex") || strings.HasSuffix(head, `\\codex`) || head == "codex.exe"
}

func isClaudeCommandHead(command string) bool {
	head := commandHead(command)
	return head == "claude" || strings.HasSuffix(head, "/claude") || strings.HasSuffix(head, `\\claude`) || head == "claude.exe"
}

func shouldTreatInputAsAICommand(data string) bool {
	trimmed := strings.TrimSpace(strings.ToLower(data))
	if trimmed == "" || strings.ContainsAny(trimmed, " \n\t") {
		return false
	}
	return isAISessionCommandLike(trimmed)
}

// ── Shell helpers ──

func shellQuote(value string) string {
	if strings.ContainsAny(value, " \t\n\"'$`\\|&;<>(){}[]#*?!~") {
		return `"` + strings.ReplaceAll(value, `"`, `\"`) + `"`
	}
	return value
}

func commandHasFlag(fields []string, flags ...string) bool {
	for _, f := range fields {
		lower := strings.ToLower(f)
		for _, flag := range flags {
			if lower == strings.ToLower(flag) {
				return true
			}
		}
	}
	return false
}

func commandHasCodexReasoningEffort(fields []string) bool {
	for i, f := range fields {
		if strings.ToLower(f) == "--config" && i+1 < len(fields) {
			if strings.HasPrefix(strings.ToLower(fields[i+1]), "model_reasoning_effort=") {
				return true
			}
		}
	}
	return false
}

// ── AI command preferences ──

func defaultAICommandFromEngine(values ...string) string {
	for _, v := range values {
		switch strings.TrimSpace(strings.ToLower(v)) {
		case "codex":
			return "codex"
		case "gemini":
			return "gemini"
		case "claude":
			return "claude"
		}
	}
	return "claude"
}

func applyAICommandPreferences(command, engine, model, reasoningEffort string) string {
	command = strings.TrimSpace(command)
	if command == "" {
		command = defaultAICommandFromEngine(engine)
	}
	isClaude := isClaudeCommandHead(command)
	isCodex := isCodexCommandHead(command)
	fields := strings.Fields(command)
	head := ""
	if len(fields) > 0 {
		head = fields[0]
	}
	if model != "" && !commandHasFlag(fields, "-m", "--model") {
		command += " --model " + shellQuote(model)
		fields = strings.Fields(command)
	}
	if engine == "codex" || isCodex {
		if reasoningEffort != "" && !commandHasCodexReasoningEffort(fields) {
			command += " --config model_reasoning_effort=" + shellQuote(reasoningEffort)
		}
	}
	_ = isClaude
	_ = head
	return command
}

func shouldInjectEnabledSkillsForInput(command string, engines ...string) bool {
	if isAISessionCommandLike(command) {
		return true
	}
	for _, e := range engines {
		switch strings.TrimSpace(strings.ToLower(e)) {
		case "claude", "codex", "gemini":
			return true
		}
	}
	return false
}

// ── Path / runtime detection ──

func isExternalCodexSummary(item data.SessionSummary) bool {
	return item.External && strings.EqualFold(strings.TrimSpace(item.Runtime.Engine), "codex")
}

func summaryCodexThreadID(item data.SessionSummary) string {
	return strings.TrimSpace(item.Runtime.ResumeSessionID)
}

func isExternalClaudeSummary(item data.SessionSummary) bool {
	return item.External && isClaudeRuntime(item.Runtime)
}

func summaryClaudeSessionID(item data.SessionSummary) string {
	return strings.TrimSpace(item.ClaudeSessionUUID)
}

func filterStoreSessionsByCWD(items []data.SessionSummary, filterCWD string) []data.SessionSummary {
	if strings.TrimSpace(filterCWD) == "" {
		return items
	}
	filtered := make([]data.SessionSummary, 0, len(items))
	for _, item := range items {
		if strings.HasPrefix(strings.TrimSpace(item.Runtime.CWD), filterCWD) ||
			strings.HasPrefix(strings.TrimSpace(item.Runtime.CWD), filterCWD+"/") {
			filtered = append(filtered, item)
		}
	}
	if len(filtered) == 0 {
		return items
	}
	return filtered
}

// ── Parser utilities ──

func parseMarkdownFrontMatter(content string) (map[string]string, string) {
	lines := strings.Split(content, "\n")
	if len(lines) < 3 || strings.TrimSpace(lines[0]) != "---" {
		return nil, content
	}
	front := make(map[string]string)
	endIdx := -1
	for i := 1; i < len(lines); i++ {
		if strings.TrimSpace(lines[i]) == "---" {
			endIdx = i
			break
		}
		parts := strings.SplitN(lines[i], ":", 2)
		if len(parts) == 2 {
			front[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
		}
	}
	if endIdx < 0 {
		return nil, content
	}
	return front, strings.Join(lines[endIdx+1:], "\n")
}

func extractMarkdownTitle(content string) string {
	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "# ") {
			return strings.TrimSpace(strings.TrimPrefix(trimmed, "# "))
		}
	}
	return ""
}

// ── Misc ──

func laterNonZeroTime(values ...time.Time) time.Time {
	var latest time.Time
	for _, v := range values {
		if !v.IsZero() && v.After(latest) {
			latest = v
		}
	}
	return latest
}

func firstNonNilInt(values ...*int) *int {
	for _, v := range values {
		if v != nil {
			return v
		}
	}
	return nil
}

func hasBinaryContent(content []byte) bool {
	for _, b := range content {
		if b == 0 {
			return true
		}
	}
	return false
}

func detectLangFromPath(path string) string {
	lower := strings.ToLower(path)
	switch {
	case strings.HasSuffix(lower, ".go"):
		return "go"
	case strings.HasSuffix(lower, ".py"):
		return "python"
	case strings.HasSuffix(lower, ".js"), strings.HasSuffix(lower, ".jsx"):
		return "javascript"
	case strings.HasSuffix(lower, ".ts"), strings.HasSuffix(lower, ".tsx"):
		return "typescript"
	case strings.HasSuffix(lower, ".rs"):
		return "rust"
	case strings.HasSuffix(lower, ".java"):
		return "java"
	case strings.HasSuffix(lower, ".kt"), strings.HasSuffix(lower, ".kts"):
		return "kotlin"
	case strings.HasSuffix(lower, ".swift"):
		return "swift"
	case strings.HasSuffix(lower, ".dart"):
		return "dart"
	case strings.HasSuffix(lower, ".c"), strings.HasSuffix(lower, ".h"):
		return "c"
	case strings.HasSuffix(lower, ".cpp"), strings.HasSuffix(lower, ".cc"), strings.HasSuffix(lower, ".cxx"), strings.HasSuffix(lower, ".hpp"):
		return "cpp"
	case strings.HasSuffix(lower, ".rb"):
		return "ruby"
	case strings.HasSuffix(lower, ".php"):
		return "php"
	case strings.HasSuffix(lower, ".sh"), strings.HasSuffix(lower, ".bash"), strings.HasSuffix(lower, ".zsh"):
		return "shell"
	case strings.HasSuffix(lower, ".yaml"), strings.HasSuffix(lower, ".yml"):
		return "yaml"
	case strings.HasSuffix(lower, ".json"):
		return "json"
	case strings.HasSuffix(lower, ".xml"):
		return "xml"
	case strings.HasSuffix(lower, ".md"), strings.HasSuffix(lower, ".markdown"):
		return "markdown"
	case strings.HasSuffix(lower, ".sql"):
		return "sql"
	case strings.HasSuffix(lower, ".css"), strings.HasSuffix(lower, ".scss"), strings.HasSuffix(lower, ".less"):
		return "css"
	case strings.HasSuffix(lower, ".html"), strings.HasSuffix(lower, ".htm"):
		return "html"
	case strings.HasSuffix(lower, ".toml"):
		return "toml"
	case strings.HasSuffix(lower, ".dockerfile"), strings.Contains(lower, "dockerfile"):
		return "dockerfile"
	case strings.HasSuffix(lower, ".makefile"), strings.Contains(lower, "makefile"):
		return "makefile"
	default:
		return ""
	}
}

// ── Projection helpers ──

func taskCursorSnapshot(sessionRuntime *RuntimeSession) session.TaskCursorSnapshot {
	if sessionRuntime == nil {
		return session.TaskCursorSnapshot{}
	}
	return session.TaskCursorSnapshot{
		LatestCursor: sessionRuntime.LatestCursor(),
		LastOutputAt: sessionRuntime.LastOutputTime(),
	}
}

func deltaCursorSnapshot(sessionRuntime *RuntimeSession) session.DeltaCursorSnapshot {
	if sessionRuntime == nil {
		return session.DeltaCursorSnapshot{}
	}
	return session.DeltaCursorSnapshot{
		LatestCursor: sessionRuntime.LatestCursor(),
	}
}

func prepareSessionEventForResume(sessionRuntime *RuntimeSession, sessionID string, event any) any {
	if sessionRuntime == nil {
		return event
	}
	event = protocol.ApplyEventCursor(event, sessionRuntime.LatestCursor())
	sessionRuntime.AppendPending(event)
	return event
}

func emitReviewStateFromProjection(sink EventSink, sessionID string, projection data.ProjectionSnapshot) {
	sink(session.ReviewStateEventFromProjection(sessionID, projection))
}

func restoredAgentStateEventFromRecord(record data.SessionRecord, hasActiveRunner bool) *protocol.AgentStateEvent {
	if !hasActiveRunner && isExternalNativeActiveRecord(record) {
		return nil
	}
	return session.RestoredAgentStateEventFromRecord(record, hasActiveRunner, isExternalNativeActiveRecord(record))
}

func isExternalNativeActiveRecord(record data.SessionRecord) bool {
	return record.Summary.External && strings.TrimSpace(record.Projection.Runtime.Command) != ""
}

func sessionRecordRuntimeAlive(record data.SessionRecord, svc *session.Service) bool {
	return session.SessionRecordRuntimeAlive(record, svc, false)
}

// ── Codex thread dedup ──

func preferCodexThreadSummary(current, candidate data.SessionSummary) data.SessionSummary {
	if candidate.UpdatedAt.After(current.UpdatedAt) {
		return candidate
	}
	if candidate.UpdatedAt.Equal(current.UpdatedAt) && candidate.EntryCount > current.EntryCount {
		return candidate
	}
	return current
}

func dedupeCodexThreadSummaries(items []data.SessionSummary) []data.SessionSummary {
	type key struct {
		threadID string
		cwd      string
	}
	seen := make(map[key]data.SessionSummary)
	for _, item := range items {
		if !isExternalCodexSummary(item) || summaryCodexThreadID(item) == "" {
			continue
		}
		k := key{threadID: summaryCodexThreadID(item), cwd: strings.TrimSpace(item.Runtime.CWD)}
		if existing, ok := seen[k]; ok {
			seen[k] = preferCodexThreadSummary(existing, item)
		} else {
			seen[k] = item
		}
	}
	result := make([]data.SessionSummary, 0, len(items))
	for _, item := range items {
		if isExternalCodexSummary(item) && summaryCodexThreadID(item) != "" {
			k := key{threadID: summaryCodexThreadID(item), cwd: strings.TrimSpace(item.Runtime.CWD)}
			if best, ok := seen[k]; ok {
				result = append(result, best)
				delete(seen, k)
				continue
			}
		}
		result = append(result, item)
	}
	return result
}

// ── Mode parsing ──

func parseMode(raw string) (engine.Mode, error) {
	return session.ParseMode(raw)
}

// ── normalizePermissionModeForClaude ──

func normalizePermissionModeForClaude(mode string) string {
	return session.NormalizeClaudePermissionMode(mode)
}

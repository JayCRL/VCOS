package semantic

import "context"

// FlowQuery describes an L3 deep-analysis query — data-flow, control-flow,
// taint, or reachability — addressed to a starting symbol or file.
type FlowQuery struct {
	ProjectRoot string
	StartSymbol string // qualified symbol name e.g. "pkg.Func"
	Path        string // alternative starting point: file path
	Kind        string // "data-flow" | "control-flow" | "taint" | "reachability"
	Limit       int    // 0 means impl-default
}

// FlowFinding is a single result from a deep-analysis query.
type FlowFinding struct {
	Title    string
	Severity string // "info" | "warn" | "critical"
	Path     string
	Line     int
	Detail   string
}

// FlowAnalyzer is the L3 architecture interface — pulled in on-demand only,
// never resident (the target backend is Joern; cf. §4.1 of the design doc).
// Implementations are expected to be slow and process-spawning, so callers
// should pass a cancellable context.
type FlowAnalyzer interface {
	Analyze(ctx context.Context, q FlowQuery) ([]FlowFinding, error)
}

// NoopFlowAnalyzer returns no findings, ever. Wire this when L3 is not
// configured so the rest of the stack can run without nil checks.
type NoopFlowAnalyzer struct{}

func (NoopFlowAnalyzer) Analyze(context.Context, FlowQuery) ([]FlowFinding, error) {
	return []FlowFinding{}, nil
}

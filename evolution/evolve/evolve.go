// Package evolve is the "进化引擎" — it accepts code-change proposals, applies
// them in an isolated shadow workspace, runs verification checks (lint, build,
// test), and distills the results into learnings that are written back to the
// Memory MOE for future retrieval.
//
// This closes the loop: propose → verify in shadow → learn → persist to memory.
package evolve

import (
	"context"
	"fmt"
	"strings"
	"time"

	"mobilevc/memory"
	"mobilevc/evolution/shadow"
)

// CheckType enumerates verification steps.
type CheckType string

const (
	CheckBuild  CheckType = "build"
	CheckTest   CheckType = "test"
	CheckLint   CheckType = "lint"
	CheckFormat CheckType = "format"
)

// Proposal is a code change to evaluate.
type Proposal struct {
	Title       string
	Description string
	CWD         string                // project root
	Changes     []shadow.FileChange  // file diffs to apply
	Checks      []CheckType          // which checks to run (empty = all)
}

// CheckResult is the outcome of a single verification step.
type CheckResult struct {
	Type     CheckType `json:"type"`
	Passed   bool      `json:"passed"`
	ExitCode int       `json:"exitCode"`
	Stdout   string    `json:"stdout,omitempty"`
	Stderr   string    `json:"stderr,omitempty"`
	Duration string    `json:"duration,omitempty"`
}

// Learning is a distilled finding from evolution.
type Learning struct {
	Pattern    string  `json:"pattern"`    // what was learned
	Confidence float64 `json:"confidence"` // 0-1
	Domain     string  `json:"domain"`     // code / dialogue / task
	Outcome    string  `json:"outcome"`    // passed / failed / partial
}

// Result is the full evolution report.
type Result struct {
	Title      string        `json:"title"`
	Passed     bool          `json:"passed"`
	Checks     []CheckResult `json:"checks"`
	Learnings  []Learning    `json:"learnings"`
	Summary    string        `json:"summary"`
	Duration   time.Duration `json:"duration"`
	StartedAt  time.Time     `json:"startedAt"`
	FinishedAt time.Time     `json:"finishedAt"`
}

// Evolver evaluates proposals in a shadow workspace.
type Evolver struct {
	Shadow *shadow.Manager
	Store  memory.Store // for persisting learnings as memory entries
}

// New creates an Evolver backed by the given shadow manager and memory store.
func New(shadowMgr *shadow.Manager, store memory.Store) *Evolver {
	return &Evolver{Shadow: shadowMgr, Store: store}
}

// Evaluate applies the proposal in a shadow workspace, runs checks, and returns
// the result with extracted learnings.
func (e *Evolver) Evaluate(ctx context.Context, p Proposal) (*Result, error) {
	startedAt := time.Now()
	r := &Result{
		Title:     p.Title,
		StartedAt: startedAt,
	}

	// Create shadow workspace.
	ws, err := e.Shadow.CreateWorkspace(ctx, p.CWD)
	if err != nil {
		return r, fmt.Errorf("evolve: create workspace: %w", err)
	}
	defer ws.Cleanup()

	// Apply changes.
	if len(p.Changes) > 0 {
		if err := ws.ApplyDiff(p.Changes); err != nil {
			return r, fmt.Errorf("evolve: apply diff: %w", err)
		}
	}

	// Determine checks.
	checks := p.Checks
	if len(checks) == 0 {
		checks = []CheckType{CheckLint, CheckBuild, CheckTest}
	}

	// Run each check.
	caps := []shadow.Capability{shadow.CapRead, shadow.CapWrite, shadow.CapBuild, shadow.CapTest, shadow.CapLint, shadow.CapFormat}
	allPassed := true
	for _, ct := range checks {
		cResult := e.runCheck(ctx, ws, caps, ct)
		r.Checks = append(r.Checks, cResult)
		if !cResult.Passed {
			allPassed = false
		}
	}

	r.Passed = allPassed
	r.Learnings = e.extractLearnings(r)
	r.Summary = e.buildSummary(r)
	r.FinishedAt = time.Now()
	r.Duration = r.FinishedAt.Sub(r.StartedAt)

	// Persist learnings to memory.
	if e.Store != nil {
		for _, l := range r.Learnings {
			_ = e.Store.Upsert(ctx, memory.Entry{
				ID:        "evolve-" + p.Title + "-" + l.Pattern,
				Type:      memory.TypeLongTerm,
				Domain:    memory.DomainCode,
				Title:     l.Pattern,
				Content:   fmt.Sprintf("outcome=%s confidence=%.2f", l.Outcome, l.Confidence),
				Source:    memory.SourceKernel,
				Metadata:  map[string]any{"pattern": l.Pattern, "confidence": l.Confidence},
				CreatedAt: time.Now().UTC(),
				UpdatedAt: time.Now().UTC(),
			})
		}
	}

	return r, nil
}

func (e *Evolver) runCheck(ctx context.Context, ws *shadow.Workspace, caps []shadow.Capability, ct CheckType) CheckResult {
	cr := CheckResult{Type: ct, Passed: true}

	var cmd string
	var args []string

	switch ct {
	case CheckBuild:
		cmd, args = "go", []string{"build", "./..."}
	case CheckTest:
		cmd, args = "go", []string{"test", "./..."}
	case CheckLint:
		cmd, args = "go", []string{"vet", "./..."}
	case CheckFormat:
		cmd, args = "gofmt", []string{"-l", "."}
	default:
		cr.Passed = true
		return cr
	}

	result, err := ws.Run(ctx, caps, cmd, args...)
	if err != nil {
		cr.Passed = false
		if result != nil {
			cr.ExitCode = result.ExitCode
			cr.Stdout = result.Stdout
			cr.Stderr = result.Stderr
			cr.Duration = result.Duration.String()
		} else {
			cr.Stderr = err.Error()
		}
		return cr
	}
	cr.ExitCode = result.ExitCode
	cr.Stdout = result.Stdout
	cr.Stderr = result.Stderr
	cr.Duration = result.Duration.String()
	if result.ExitCode != 0 {
		cr.Passed = false
	}
	return cr
}

func (e *Evolver) extractLearnings(r *Result) []Learning {
	var ls []Learning
	for _, c := range r.Checks {
		if c.Passed {
			ls = append(ls, Learning{
				Pattern:    string(c.Type) + "_passed",
				Confidence: 0.9,
				Domain:     "code",
				Outcome:    "passed",
			})
		} else {
			// Extract error pattern from stderr.
			pattern := extractErrorPattern(c.Stderr)
			if pattern == "" {
				pattern = string(c.Type) + "_failed"
			}
			ls = append(ls, Learning{
				Pattern:    pattern,
				Confidence: 0.7,
				Domain:     "code",
				Outcome:    "failed",
			})
		}
	}
	return ls
}

func (e *Evolver) buildSummary(r *Result) string {
	passed := 0
	for _, c := range r.Checks {
		if c.Passed {
			passed++
		}
	}
	status := "PASSED"
	if !r.Passed {
		status = "FAILED"
	}
	return fmt.Sprintf("%s: %s (%d/%d checks passed)", r.Title, status, passed, len(r.Checks))
}

func extractErrorPattern(stderr string) string {
	lines := strings.Split(stderr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Look for common Go error patterns.
		if idx := strings.Index(line, ": "); idx > 0 {
			// "file.go:10:2: undefined: foo" → take the message part.
			parts := strings.SplitN(line, ": ", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
		return line
	}
	return ""
}

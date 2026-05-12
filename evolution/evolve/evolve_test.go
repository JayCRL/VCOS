package evolve

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"mobilevc/memory"
	"mobilevc/evolution/shadow"
)

func TestEvaluateBuildPass(t *testing.T) {
	src := setupGoProject(t, "package main\n\nfunc main() {}\n")

	store := memory.NewMemStore()
	ev := New(shadow.NewManager(t.TempDir()), store)

	result, err := ev.Evaluate(context.Background(), Proposal{
		Title:  "test-build",
		CWD:    src,
		Checks: []CheckType{CheckBuild},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !result.Passed {
		t.Fatalf("build should pass: %s", result.Summary)
	}
	if len(result.Learnings) == 0 {
		t.Fatal("expected learnings")
	}

	entries, _ := store.Query(context.Background(), memory.Filter{Types: []memory.Type{memory.TypeLongTerm}})
	if len(entries) == 0 {
		t.Fatal("no learnings persisted to memory")
	}
}

func TestEvaluateBuildFail(t *testing.T) {
	src := setupGoProject(t, "package main\n\nfunc main() {\n\tundefinedVar\n}\n")

	store := memory.NewMemStore()
	ev := New(shadow.NewManager(t.TempDir()), store)

	result, err := ev.Evaluate(context.Background(), Proposal{
		Title:  "test-build-fail",
		CWD:    src,
		Checks: []CheckType{CheckBuild},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if result.Passed {
		t.Fatal("build should fail")
	}
	if len(result.Checks) != 1 || result.Checks[0].Passed {
		t.Fatalf("expected failed check: %+v", result.Checks)
	}
}

func TestEvaluateWithDiff(t *testing.T) {
	src := setupGoProject(t, "package main\n") // invalid: no main func

	store := memory.NewMemStore()
	ev := New(shadow.NewManager(t.TempDir()), store)

	result, err := ev.Evaluate(context.Background(), Proposal{
		Title: "test-diff",
		CWD:   src,
		Changes: []shadow.FileChange{
			{Path: "main.go", NewContent: "package main\n\nfunc main() {}\n"},
		},
		Checks: []CheckType{CheckBuild},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if !result.Passed {
		t.Fatalf("diff+eval failed: %s", result.Summary)
	}
}

func TestExtractErrorPattern(t *testing.T) {
	stderr := "main.go:3:2: undefined: undefinedVar"
	got := extractErrorPattern(stderr)
	if !strings.Contains(got, "undefined: undefinedVar") {
		t.Fatalf("got %q", got)
	}
	if extractErrorPattern("") != "" {
		t.Fatal("expected empty")
	}
}

func setupGoProject(t *testing.T, mainContent string) string {
	t.Helper()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n\ngo 1.25\n"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "main.go"), []byte(mainContent), 0644)

	// Init git so shadow can use worktree.
	runGit(t, dir, "init")
	runGit(t, dir, "config", "user.email", "test@test")
	runGit(t, dir, "config", "user.name", "test")
	runGit(t, dir, "add", ".")
	runGit(t, dir, "commit", "-m", "init")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("git %v: %v (%s)", args, err, strings.TrimSpace(string(out)))
	}
}

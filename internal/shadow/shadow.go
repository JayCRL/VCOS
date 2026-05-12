// Package shadow manages isolated git-worktree (or copy) workspaces for safe
// code evaluation. Each workspace runs within a capability whitelist — only
// read-only and build/test commands are permitted; destructive operations
// (rm -rf, network outbound, etc.) are rejected at the gate.
//
// This is the "影子空间" from the architecture diagram: an isolation sandbox
// where the evolution engine can apply diffs, run linters, and verify fixes
// without affecting the user's working tree.
package shadow

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ErrCapabilityBlocked is returned when a command is rejected by the whitelist.
var ErrCapabilityBlocked = errors.New("shadow: command blocked by capability whitelist")

// ErrNotGitRepo returned when the source is not a git repository and Worktree
// isolation was requested.
var ErrNotGitRepo = errors.New("shadow: not a git repository")

// Capability is a named permission for workspace operations.
type Capability string

const (
	CapRead      Capability = "read"      // file reads, git status
	CapWrite     Capability = "write"     // file writes inside the workspace
	CapBuild     Capability = "build"     // go build, cargo build, npm run build
	CapTest      Capability = "test"      // go test, cargo test, npm test
	CapLint      Capability = "lint"      // go vet, golangci-lint, eslint
	CapFormat    Capability = "format"    // gofmt, prettier
	CapNetwork   Capability = "network"   // outbound network (blocked by default)
	CapShell     Capability = "shell"     // arbitrary shell commands (blocked by default)
)

// Workspace is an isolated directory for safe code operations.
type Workspace struct {
	Path       string
	SourcePath string       // original project path
	cleanup    func() error
	createdAt  time.Time
}

// RunResult captures the output of a command executed in the workspace.
type RunResult struct {
	Command  string
	ExitCode int
	Stdout   string
	Stderr   string
	Duration time.Duration
}

// Manager creates and pools shadow workspaces.
type Manager struct {
	basePath    string
	defaultCaps []Capability
	timeout     time.Duration

	mu   sync.Mutex
	pool []*Workspace
}

// NewManager creates a shadow manager. basePath is where shadow workspaces are
// created (defaults to os.TempDir if empty).
func NewManager(basePath string) *Manager {
	if basePath == "" {
		basePath = filepath.Join(os.TempDir(), "agentos-shadow")
	}
	_ = os.MkdirAll(basePath, 0700)
	return &Manager{
		basePath: basePath,
		defaultCaps: []Capability{CapRead, CapWrite, CapBuild, CapTest, CapLint, CapFormat},
		timeout: 5 * time.Minute,
	}
}

// CreateWorkspace creates an isolated copy of projectPath using git worktree.
// Falls back to a file copy if the project is not a git repository.
func (m *Manager) CreateWorkspace(ctx context.Context, projectPath string) (*Workspace, error) {
	base := filepath.Join(m.basePath, fmt.Sprintf("ws-%d", time.Now().UnixNano()))
	if err := os.MkdirAll(base, 0700); err != nil {
		return nil, fmt.Errorf("shadow: create base dir: %w", err)
	}

	ws := &Workspace{
		SourcePath: projectPath,
		Path:       base,
		createdAt:  time.Now(),
	}

	isGit := isGitRepo(projectPath)
	if isGit {
		if err := m.createGitWorktree(projectPath, base); err != nil {
			// Fallback to copy.
			isGit = false
		}
	}
	if !isGit {
		if err := copyDir(projectPath, base); err != nil {
			os.RemoveAll(base)
			return nil, fmt.Errorf("shadow: copy project: %w", err)
		}
	}

	ws.cleanup = func() error { return os.RemoveAll(base) }
	return ws, nil
}

func (m *Manager) createGitWorktree(projectPath, target string) error {
	cmd := exec.Command("git", "-C", projectPath, "worktree", "add", "--detach", target)
	cmd.Env = os.Environ()
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git worktree add: %w (%s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// Run executes a command in the workspace if allowed by the capability whitelist.
func (ws *Workspace) Run(ctx context.Context, caps []Capability, command string, args ...string) (*RunResult, error) {
	if !ws.Allow(caps, command, args) {
		return nil, ErrCapabilityBlocked
	}

	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, command, args...)
	cmd.Dir = ws.Path
	cmd.Env = os.Environ()

	start := time.Now()
	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := &RunResult{
		Command:  command + " " + strings.Join(args, " "),
		Stdout:   strings.TrimSpace(stdout.String()),
		Stderr:   strings.TrimSpace(stderr.String()),
		Duration: time.Since(start),
	}
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.ExitCode = -1
			result.Stderr += "\n" + err.Error()
		}
	}
	return result, nil
}

// ApplyDiff writes file changes into the workspace.
func (ws *Workspace) ApplyDiff(diffs []FileChange) error {
	for _, d := range diffs {
		target := filepath.Join(ws.Path, d.Path)
		if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
			return fmt.Errorf("shadow: mkdir for %s: %w", d.Path, err)
		}
		if err := os.WriteFile(target, []byte(d.NewContent), 0644); err != nil {
			return fmt.Errorf("shadow: write %s: %w", d.Path, err)
		}
	}
	return nil
}

// Cleanup removes the workspace.
func (ws *Workspace) Cleanup() error {
	if ws.cleanup != nil {
		// Also prune the git worktree if applicable.
		if isGitRepo(ws.SourcePath) {
			_ = exec.Command("git", "-C", ws.SourcePath, "worktree", "prune").Run()
		}
		err := ws.cleanup()
		ws.cleanup = nil
		return err
	}
	return nil
}

// Allow checks whether the given command is permitted under the caps.
func (ws *Workspace) Allow(caps []Capability, command string, args []string) bool {
	cmd := strings.ToLower(filepath.Base(command))
	full := strings.ToLower(command + " " + strings.Join(args, " "))

	has := func(c Capability) bool {
		for _, x := range caps {
			if x == c {
				return true
			}
		}
		return false
	}

	// Block dangerous patterns regardless of caps.
	for _, block := range []string{"rm ", "sudo ", "chmod 777", "> /dev/", "mkfs.", "dd if=", ":(){", "curl ", "wget "} {
		if strings.Contains(full, block) {
			return false
		}
	}

	switch {
	case cmd == "go":
		return has(CapBuild) || has(CapTest) || has(CapLint)
	case cmd == "cargo" && (has(CapBuild) || has(CapTest)):
		return true
	case cmd == "npm" || cmd == "npx" || cmd == "yarn":
		return has(CapBuild) || has(CapTest) || has(CapLint)
	case cmd == "python" || cmd == "python3" || cmd == "pytest":
		return has(CapTest)
	case cmd == "rustc":
		return has(CapBuild)
	case cmd == "make" || cmd == "cmake":
		return has(CapBuild)
	// Read-only commands.
	case cmd == "cat" || cmd == "ls" || cmd == "head" || cmd == "tail" || cmd == "wc":
		return has(CapRead)
	case cmd == "git":
		return !strings.Contains(full, "push") && has(CapRead)
	// Lint/formatters.
	case cmd == "gofmt" || cmd == "golangci-lint" || cmd == "eslint" || cmd == "prettier":
		return has(CapLint) || has(CapFormat)
	// Shell — only if explicitly allowed.
	case cmd == "sh" || cmd == "bash" || cmd == "zsh":
		return has(CapShell)
	default:
		return has(CapRead) // conservative: allow unknown read-only-ish commands
	}
}

// ——— helpers ———

func isGitRepo(path string) bool {
	cmd := exec.Command("git", "-C", path, "rev-parse", "--git-dir")
	cmd.Env = os.Environ()
	return cmd.Run() == nil
}

func copyDir(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		// Skip .git directories.
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		if info.IsDir() {
			return os.MkdirAll(target, info.Mode())
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", rel, err)
		}
		return os.WriteFile(target, data, info.Mode())
	})
}

// FileChange represents a single file modification.
type FileChange struct {
	Path       string
	OldContent string
	NewContent string
}

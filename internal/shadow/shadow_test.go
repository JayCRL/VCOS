package shadow

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestCreateWorkspaceGit(t *testing.T) {
	// Create a temp git repo.
	src := t.TempDir()
	mustRun(t, src, "git", "init")
	mustRun(t, src, "git", "config", "user.email", "test@test")
	mustRun(t, src, "git", "config", "user.name", "test")
	_ = os.WriteFile(filepath.Join(src, "README.md"), []byte("# test"), 0644)
	mustRun(t, src, "git", "add", ".")
	mustRun(t, src, "git", "commit", "-m", "init")

	m := NewManager(t.TempDir())
	ws, err := m.CreateWorkspace(context.Background(), src)
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	defer ws.Cleanup()

	// Verify README.md exists in shadow.
	data, err := os.ReadFile(filepath.Join(ws.Path, "README.md"))
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if string(data) != "# test" {
		t.Fatalf("content=%q", string(data))
	}
}

func TestCreateWorkspaceCopyFallback(t *testing.T) {
	src := t.TempDir()
	_ = os.WriteFile(filepath.Join(src, "hello.txt"), []byte("hello"), 0644)

	m := NewManager(t.TempDir())
	ws, err := m.CreateWorkspace(context.Background(), src)
	if err != nil {
		t.Fatalf("CreateWorkspace (non-git): %v", err)
	}
	defer ws.Cleanup()

	data, _ := os.ReadFile(filepath.Join(ws.Path, "hello.txt"))
	if string(data) != "hello" {
		t.Fatalf("copy failed")
	}
}

func TestRunAllowedCommand(t *testing.T) {
	m := NewManager(t.TempDir())
	ws, err := m.CreateWorkspace(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Cleanup()

	caps := []Capability{CapRead}
	result, err := ws.Run(context.Background(), caps, "ls")
	if err != nil {
		t.Fatalf("Run ls: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("ls exit=%d", result.ExitCode)
	}
}

func TestRunBlockedCommand(t *testing.T) {
	m := NewManager(t.TempDir())
	ws, err := m.CreateWorkspace(context.Background(), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	defer ws.Cleanup()

	_, err = ws.Run(context.Background(), []Capability{CapRead}, "rm", "-rf", "/")
	if err != ErrCapabilityBlocked {
		t.Fatalf("expected ErrCapabilityBlocked, got %v", err)
	}
}

func TestApplyDiff(t *testing.T) {
	src := t.TempDir()
	_ = os.WriteFile(filepath.Join(src, "main.go"), []byte("package main"), 0644)

	m := NewManager(t.TempDir())
	ws, _ := m.CreateWorkspace(context.Background(), src)
	defer ws.Cleanup()

	err := ws.ApplyDiff([]FileChange{
		{Path: "main.go", OldContent: "package main", NewContent: "package main\n\nimport \"fmt\""},
	})
	if err != nil {
		t.Fatalf("ApplyDiff: %v", err)
	}
	data, _ := os.ReadFile(filepath.Join(ws.Path, "main.go"))
	if !strings.Contains(string(data), "import \"fmt\"") {
		t.Fatalf("diff not applied: %s", string(data))
	}
}

func TestCapabilityWhitelist(t *testing.T) {
	ws := &Workspace{Path: "/tmp"}

	tests := []struct {
		cmd  string
		args []string
		caps []Capability
		ok   bool
	}{
		{"go", []string{"build", "./..."}, []Capability{CapBuild}, true},
		{"go", []string{"test", "./..."}, []Capability{CapTest}, true},
		{"go", []string{"build", "./..."}, []Capability{CapRead}, false},
		{"rm", []string{"-rf", "."}, []Capability{CapShell}, false},
		{"curl", []string{"http://evil"}, []Capability{CapNetwork}, false},
		{"sudo", []string{"make"}, []Capability{CapShell}, false},
		{"git", []string{"status"}, []Capability{CapRead}, true},
		{"git", []string{"push"}, []Capability{CapRead}, false},
		{"ls", nil, []Capability{CapRead}, true},
		{"cat", []string{"README.md"}, []Capability{CapRead}, true},
		{"npm", []string{"run", "build"}, []Capability{CapBuild}, true},
	}

	for _, tt := range tests {
		got := ws.Allow(tt.caps, tt.cmd, tt.args)
		if got != tt.ok {
			t.Errorf("Allow(%v, %s %v)=%v want %v", tt.caps, tt.cmd, tt.args, got, tt.ok)
		}
	}
}

func mustRun(t *testing.T, dir, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %v: %v (%s)", name, args, err, strings.TrimSpace(string(out)))
	}
}

func TestManagerTimeout(t *testing.T) {
	m := NewManager(t.TempDir())
	if m.timeout != 5*time.Minute {
		t.Fatalf("timeout=%v", m.timeout)
	}
}

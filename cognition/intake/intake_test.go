package intake

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"mobilevc/memory"
)

func TestDetectProjectGo(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0644)

	h := detectProject(dir)
	if h.Language != "go" {
		t.Fatalf("Language=%q want go", h.Language)
	}
	if h.BuildTool != "go mod" {
		t.Fatalf("BuildTool=%q", h.BuildTool)
	}
}

func TestDetectProjectMulti(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "package.json"), []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "tsconfig.json"), []byte("{}"), 0644)
	_ = os.WriteFile(filepath.Join(dir, "vite.config.ts"), []byte(""), 0644)

	h := detectProject(dir)
	if h.Language != "javascript/typescript" {
		t.Fatalf("Language=%q", h.Language)
	}
	if h.Framework != "vite" {
		t.Fatalf("Framework=%q want vite", h.Framework)
	}
}

func TestInferRole(t *testing.T) {
	if r := inferRole("please review this PR"); r != "reviewer" {
		t.Fatalf("got %q", r)
	}
	if r := inferRole("design a new API architecture"); r != "architect" {
		t.Fatalf("got %q", r)
	}
	if r := inferRole("fix the null pointer bug"); r != "debugger" {
		t.Fatalf("got %q", r)
	}
	if r := inferRole("add a login endpoint"); r != "developer" {
		t.Fatalf("got %q", r)
	}
}

func TestInferStyle(t *testing.T) {
	if s := inferStyle("give me a concise answer"); s != "concise" {
		t.Fatalf("got %q", s)
	}
	if s := inferStyle("explain this in detail"); s != "verbose" {
		t.Fatalf("got %q", s)
	}
	if s := inferStyle("hello"); s != "balanced" {
		t.Fatalf("got %q", s)
	}
}

func TestAnalyzeAndBootstrap(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module test\n"), 0644)

	store := memory.NewMemStore()
	s := New(store)

	profile := s.Analyze(dir, "please review the auth module")
	if profile.ProjectHint.Language != "go" {
		t.Fatalf("lang=%q", profile.ProjectHint.Language)
	}
	if profile.Role != "reviewer" {
		t.Fatalf("role=%q", profile.Role)
	}

	err := s.Bootstrap(context.Background(), "sess-intake", dir, profile)
	if err != nil {
		t.Fatalf("Bootstrap: %v", err)
	}

	entries, _ := store.Query(context.Background(), memory.Filter{SessionID: "sess-intake"})
	if len(entries) < 2 {
		t.Fatalf("expected >=2 entries, got %d", len(entries))
	}
}

func TestRun(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "Cargo.toml"), []byte("[package]\nname=\"test\""), 0644)

	store := memory.NewMemStore()
	s := New(store)

	profile, err := s.Run(context.Background(), "sess-run", dir, "design a new parser")
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	if profile.ProjectHint.Language != "rust" {
		t.Fatalf("lang=%q", profile.ProjectHint.Language)
	}
	if profile.Role != "architect" {
		t.Fatalf("role=%q", profile.Role)
	}
}

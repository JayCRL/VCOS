// Package intake performs session-level cognitive construction — it detects
// project context from the working directory, classifies the user's intent
// and role, and seeds the initial Memory entries that bootstrap the Memory
// MOE for a new session.
//
// This is the "初始诉求 & 认知构建" block from the architecture diagram.
package intake

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mobilevc/memory"
)

// ProjectHint describes a detected project context.
type ProjectHint struct {
	Language string   // go, python, js, rust, etc.
	Framework string  // inferred from deps
	BuildTool string  // go mod, npm, pip, cargo
	Files     []string // key project files found
}

// CognitiveProfile is the output of session intake.
type CognitiveProfile struct {
	Goal        string      // what the user wants to do (from initial prompt)
	Role        string      // inferred user role: developer, reviewer, architect
	Style       string      // preferred interaction style hint
	ProjectHint ProjectHint // project context, if any
}

// Store is the subset of memory.Store needed by intake.
type Store interface {
	Upsert(ctx context.Context, entry memory.Entry) error
}

// SessionIntake analyses a new session and seeds initial memories.
type SessionIntake struct {
	Store Store
}

// New creates a SessionIntake backed by the given memory store.
func New(store Store) *SessionIntake { return &SessionIntake{Store: store} }

// Analyze detects project context and returns a cognitive profile.
func (s *SessionIntake) Analyze(cwd string, initialPrompt string) CognitiveProfile {
	p := CognitiveProfile{Goal: initialPrompt}
	if cwd == "" {
		return p
	}
	hint := detectProject(cwd)
	p.ProjectHint = hint
	p.Role = inferRole(initialPrompt)
	p.Style = inferStyle(initialPrompt)
	return p
}

// Bootstrap seeds initial memory entries for a new session.
func (s *SessionIntake) Bootstrap(ctx context.Context, sessionID, cwd string, profile CognitiveProfile) error {
	if s.Store == nil {
		return nil
	}
	now := time.Now().UTC()

	// Project context memory
	if profile.ProjectHint.Language != "" {
		_ = s.Store.Upsert(ctx, memory.Entry{
			ID: "intake-project-" + sessionID, Type: memory.TypeProject, Domain: memory.DomainCode,
			Title: "project context", SessionID: sessionID, CWD: cwd,
			Content:   formatProjectHint(profile.ProjectHint),
			Source:    memory.SourceKernel,
			CreatedAt: now, UpdatedAt: now,
		})
	}

	// Goal memory
	if profile.Goal != "" {
		_ = s.Store.Upsert(ctx, memory.Entry{
			ID: "intake-goal-" + sessionID, Type: memory.TypeShortTerm, Domain: memory.DomainTask,
			Title: "session goal", SessionID: sessionID, CWD: cwd,
			Content:   profile.Goal,
			Source:    memory.SourceUser,
			CreatedAt: now, UpdatedAt: now,
		})
	}

	// Role/style preference (long-term, survives session)
	if profile.Role != "" || profile.Style != "" {
		_ = s.Store.Upsert(ctx, memory.Entry{
			ID: "intake-pref-" + sessionID, Type: memory.TypeLongTerm, Domain: memory.DomainDialogue,
			Title: "user preference", SessionID: sessionID, CWD: cwd,
			Content:   "role=" + profile.Role + " style=" + profile.Style,
			Source:    memory.SourceKernel,
			CreatedAt: now, UpdatedAt: now,
		})
	}
	return nil
}

// Run is a convenience: Analyze + Bootstrap.
func (s *SessionIntake) Run(ctx context.Context, sessionID, cwd, initialPrompt string) (CognitiveProfile, error) {
	profile := s.Analyze(cwd, initialPrompt)
	return profile, s.Bootstrap(ctx, sessionID, cwd, profile)
}

// ——— project detection ———

type projectSig struct {
	File     string
	Language string
	Tool     string
}

var projectSignatures = []projectSig{
	{"go.mod", "go", "go mod"},
	{"package.json", "javascript/typescript", "npm/yarn"},
	{"tsconfig.json", "typescript", "tsc"},
	{"Cargo.toml", "rust", "cargo"},
	{"requirements.txt", "python", "pip"},
	{"pyproject.toml", "python", "pip/poetry"},
	{"setup.py", "python", "setuptools"},
	{"Pipfile", "python", "pipenv"},
	{"Gemfile", "ruby", "bundler"},
	{"pom.xml", "java", "maven"},
	{"build.gradle", "java/kotlin", "gradle"},
	{"CMakeLists.txt", "c/c++", "cmake"},
	{"Makefile", "c/c++", "make"},
	{"Dockerfile", "container", "docker"},
	{"docker-compose.yml", "container", "docker compose"},
	{".git", "any", "git"},
}

func detectProject(cwd string) ProjectHint {
	h := ProjectHint{}
	for _, sig := range projectSignatures {
		path := filepath.Join(cwd, sig.File)
		if info, err := os.Stat(path); err == nil && !info.IsDir() {
			if h.Language == "" {
				h.Language = sig.Language
			}
			if h.BuildTool == "" {
				h.BuildTool = sig.Tool
			}
			h.Files = append(h.Files, sig.File)
		}
	}
	// Detect framework hints
	if hasFile(cwd, "next.config.js") || hasFile(cwd, "next.config.ts") {
		h.Framework = "next.js"
	} else if hasFile(cwd, "tailwind.config.js") || hasFile(cwd, "tailwind.config.ts") {
		h.Framework = "tailwind"
	} else if hasFile(cwd, "vite.config.js") || hasFile(cwd, "vite.config.ts") {
		h.Framework = "vite"
	}
	return h
}

func hasFile(cwd, name string) bool {
	_, err := os.Stat(filepath.Join(cwd, name))
	return err == nil
}

func formatProjectHint(h ProjectHint) string {
	parts := []string{"language=" + h.Language}
	if h.Framework != "" {
		parts = append(parts, "framework="+h.Framework)
	}
	if h.BuildTool != "" {
		parts = append(parts, "build="+h.BuildTool)
	}
	return strings.Join(parts, " ")
}

// ——— role / style inference ———

func inferRole(prompt string) string {
	lower := strings.ToLower(prompt)
	switch {
	case strings.Contains(lower, "review") || strings.Contains(lower, "audit") || strings.Contains(lower, "security"):
		return "reviewer"
	case strings.Contains(lower, "architect") || strings.Contains(lower, "design") || strings.Contains(lower, "plan"):
		return "architect"
	case strings.Contains(lower, "debug") || strings.Contains(lower, "fix") || strings.Contains(lower, "bug"):
		return "debugger"
	default:
		return "developer"
	}
}

func inferStyle(prompt string) string {
	lower := strings.ToLower(prompt)
	switch {
	case strings.Contains(lower, "concise") || strings.Contains(lower, "brief") || strings.Contains(lower, "short"):
		return "concise"
	case strings.Contains(lower, "detailed") || strings.Contains(lower, "verbose") || strings.Contains(lower, "explain"):
		return "verbose"
	default:
		return "balanced"
	}
}

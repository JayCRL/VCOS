package main

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	wailsRuntime "github.com/wailsapp/wails/v2/pkg/runtime"
)

// enrichPATH augments the process PATH so that downstream `claude` /
// `npm` / `node` invocations succeed even when the .app bundle was launched
// from Finder/Dock (where PATH is just /usr/bin:/bin and ignores the user's
// shell rc files).
//
// Strategy: ask the user's login shell what PATH it would set, then union
// that with a hardcoded list of common toolchain bin dirs.
func enrichPATH(ctx context.Context) {
	candidates := []string{os.Getenv("SHELL"), "/bin/zsh", "/bin/bash"}
	var shellPATH string
	for _, sh := range candidates {
		if sh == "" {
			continue
		}
		out, err := exec.Command(sh, "-l", "-i", "-c", "echo $PATH").Output()
		if err != nil {
			out, err = exec.Command(sh, "-l", "-c", "echo $PATH").Output()
		}
		if err == nil {
			shellPATH = strings.TrimSpace(string(out))
			if shellPATH != "" {
				break
			}
		}
	}

	home, _ := os.UserHomeDir()
	fallback := []string{
		"/opt/homebrew/bin",
		"/opt/homebrew/sbin",
		"/usr/local/bin",
		"/usr/local/sbin",
		filepath.Join(home, ".npm-global/bin"),
		filepath.Join(home, ".local/bin"),
		filepath.Join(home, ".bun/bin"),
		filepath.Join(home, ".volta/bin"),
		filepath.Join(home, ".cargo/bin"),
		filepath.Join(home, ".yarn/bin"),
		filepath.Join(home, ".deno/bin"),
	}

	parts := []string{os.Getenv("PATH")}
	if shellPATH != "" {
		parts = append(parts, shellPATH)
	}
	parts = append(parts, fallback...)
	merged := dedupePath(strings.Join(parts, string(os.PathListSeparator)))
	os.Setenv("PATH", merged)
	wailsRuntime.LogInfof(ctx, "PATH enriched (%d entries)", strings.Count(merged, string(os.PathListSeparator))+1)
}

func dedupePath(p string) string {
	seen := map[string]struct{}{}
	out := []string{}
	for _, s := range strings.Split(p, string(os.PathListSeparator)) {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if _, ok := seen[s]; ok {
			continue
		}
		seen[s] = struct{}{}
		out = append(out, s)
	}
	return strings.Join(out, string(os.PathListSeparator))
}

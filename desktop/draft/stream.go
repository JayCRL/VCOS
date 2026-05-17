package draft

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
)

// ChunkCallback receives bytes as they arrive from claude stdout.
// Called from the streaming goroutine; must not block long.
type ChunkCallback func(chunk string)

// RunStream invokes `claude --print` with the given prompt at cwd and streams
// stdout bytes through cb. Returns the full accumulated text and any error.
//
// We use --output-format text rather than stream-json because text mode is
// already line-streamed by claude CLI (no buffering between tokens), and the
// raw text is what we want to render — no need to reconstruct it from
// stream-json deltas.
func RunStream(ctx context.Context, cwd, prompt string, cb ChunkCallback) (string, error) {
	args := []string{"--print", "--output-format", "text", prompt}
	cmd := exec.CommandContext(ctx, "claude", args...)
	if cwd != "" {
		cmd.Dir = cwd
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return "", fmt.Errorf("stdout pipe: %w", err)
	}
	var stderrBuf strings.Builder
	cmd.Stderr = &stderrCapture{w: &stderrBuf}

	if err := cmd.Start(); err != nil {
		return "", fmt.Errorf("claude start: %w", err)
	}

	var collected strings.Builder
	reader := bufio.NewReader(stdout)
	buf := make([]byte, 512)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			s := string(buf[:n])
			collected.WriteString(s)
			if cb != nil {
				cb(s)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			_ = cmd.Wait()
			return collected.String(), fmt.Errorf("read stdout: %w", readErr)
		}
	}

	if err := cmd.Wait(); err != nil {
		return collected.String(), fmt.Errorf("claude exit: %w; stderr=%s", err, snippet(stderrBuf.String(), 240))
	}
	return collected.String(), nil
}

type stderrCapture struct{ w *strings.Builder }

func (c *stderrCapture) Write(p []byte) (int, error) {
	c.w.Write(p)
	return len(p), nil
}

func snippet(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
}

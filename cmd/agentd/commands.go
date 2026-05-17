package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"mobilevc/cognition/dashboard"
	"mobilevc/cognition/vibe"
	"mobilevc/evolution/evolve"
	"mobilevc/evolution/feedback"
	"mobilevc/evolution/shadow"
	"mobilevc/kernel"
	"mobilevc/kernel/scheduler"
	"mobilevc/memory"
)

// cliState owns the per-process resources that slash commands touch.
type cliState struct {
	k          *kernel.Kernel
	sessionID  string
	cwd        string
	shadowWS   *shadow.Workspace
	dashAddr   string
	dashServer *http.Server
}

// handleSlash dispatches a "/..." input line. Returns (handled, shouldExit).
// Non-slash input is returned as not handled so the caller forwards it to the AI.
func (c *cliState) handleSlash(ctx context.Context, line string) (handled, exit bool) {
	if !strings.HasPrefix(line, "/") {
		return false, false
	}
	args := strings.Fields(line)
	if len(args) == 0 {
		return true, false
	}
	cmd, args := args[0], args[1:]
	switch cmd {
	case "/exit", "/quit":
		return true, true
	case "/help":
		c.printHelp()
	case "/vibe":
		c.cmdVibe(ctx, args)
	case "/intake":
		c.cmdIntake(ctx, args)
	case "/mem":
		c.cmdMem(ctx, args)
	case "/shadow":
		c.cmdShadow(ctx, args)
	case "/evolve":
		c.cmdEvolve(ctx, args)
	case "/fb":
		c.cmdFeedback(ctx, args)
	case "/state":
		c.cmdState()
	case "/dash":
		c.cmdDash(args)
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s (try /help)\n", cmd)
	}
	return true, false
}

func (c *cliState) shutdown() {
	if c.dashServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		_ = c.dashServer.Shutdown(ctx)
		c.dashServer = nil
	}
	if c.shadowWS != nil {
		_ = c.shadowWS.Cleanup()
		c.shadowWS = nil
	}
}

func (c *cliState) printHelp() {
	fmt.Println(`Slash commands:
  /help                                show this help
  /exit, /quit                         quit

  vibe / cognition
    /vibe                              show current style/proactivity/role
    /vibe set k=v [k=v ...]            keys: style, proactivity, role
    /intake <prompt>                   run cognitive intake + seed memory

  memory
    /mem list [N]                      list recent N entries (default 10)
    /mem add <text>                    write a memory entry via MOE
    /mem find <query>                  retrieve similar entries
    /mem rm <id>                       delete an entry

  shadow + evolve
    /shadow new                        create a shadow worktree from cwd
    /shadow run <cmd> [args...]        run a whitelisted command in shadow
    /shadow drop                       cleanup current shadow
    /evolve [title]                    evaluate cwd (lint/build/test), propose

  feedback
    /fb pending                        list pending suggestions
    /fb accept <id>                    accept
    /fb reject <id>                    reject
    /fb adjust <id> <text>             adjust + accept

  runtime
    /state                             scheduler/lock/watchdog snapshot
    /dash [port]                       start dashboard HTTP (default 8090)

Anything not starting with "/" is forwarded to the AI runtime.`)
}

// ——— vibe ———

func (c *cliState) cmdVibe(ctx context.Context, args []string) {
	if len(args) == 0 {
		s := c.k.Vibe.Get(ctx, c.sessionID)
		fmt.Printf("style=%s  proactivity=%s  role=%s  updatedAt=%s\n",
			s.Style, s.Proactivity, s.Role, s.UpdatedAt.Format(time.RFC3339))
		return
	}
	if args[0] != "set" {
		fmt.Fprintln(os.Stderr, "usage: /vibe | /vibe set key=value ...")
		return
	}
	s := c.k.Vibe.Get(ctx, c.sessionID)
	for _, kv := range args[1:] {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			fmt.Fprintf(os.Stderr, "skip malformed: %q\n", kv)
			continue
		}
		switch parts[0] {
		case "style":
			s.Style = vibe.Style(parts[1])
		case "proactivity":
			s.Proactivity = vibe.Proactivity(parts[1])
		case "role":
			s.Role = vibe.Role(parts[1])
		default:
			fmt.Fprintf(os.Stderr, "unknown key: %s\n", parts[0])
		}
	}
	if err := c.k.Vibe.Set(ctx, c.sessionID, s); err != nil {
		fmt.Fprintf(os.Stderr, "set vibe: %v\n", err)
		return
	}
	fmt.Printf("ok: style=%s proactivity=%s role=%s\n", s.Style, s.Proactivity, s.Role)
}

// ——— intake ———

func (c *cliState) cmdIntake(ctx context.Context, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: /intake <prompt>")
		return
	}
	prompt := strings.Join(args, " ")
	profile, err := c.k.Intake.Run(ctx, c.sessionID, c.cwd, prompt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "intake: %v\n", err)
		return
	}
	fmt.Printf("goal:    %s\n", profile.Goal)
	fmt.Printf("role:    %s\n", profile.Role)
	fmt.Printf("style:   %s\n", profile.Style)
	fmt.Printf("project: lang=%s framework=%s tool=%s\n",
		profile.ProjectHint.Language, profile.ProjectHint.Framework, profile.ProjectHint.BuildTool)
	if len(profile.ProjectHint.Files) > 0 {
		fmt.Printf("  files: %s\n", strings.Join(profile.ProjectHint.Files, ", "))
	}
}

// ——— memory ———

func (c *cliState) cmdMem(ctx context.Context, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: /mem list|add|find|rm ...")
		return
	}
	switch args[0] {
	case "list":
		n := 10
		if len(args) >= 2 {
			if v, err := strconv.Atoi(args[1]); err == nil && v > 0 {
				n = v
			}
		}
		entries, err := c.k.MemStore.Query(ctx, memory.Filter{Limit: n})
		if err != nil {
			fmt.Fprintf(os.Stderr, "query: %v\n", err)
			return
		}
		if len(entries) == 0 {
			fmt.Println("(no memory entries)")
			return
		}
		for _, e := range entries {
			fmt.Printf("  %-34s  [%s/%s]  %s\n", e.ID, e.Type, e.Domain, oneLine(e.Title, e.Content))
		}
	case "add":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: /mem add <text>")
			return
		}
		text := strings.Join(args[1:], " ")
		req := scheduler.IntentRequest{
			Kind:      scheduler.KindSlash,
			SessionID: c.sessionID,
			Owner:     c.sessionID,
			Command:   text,
			CWD:       c.cwd,
		}
		if err := c.k.MoeRouter.WriteMemory(ctx, req, text); err != nil {
			fmt.Fprintf(os.Stderr, "write: %v\n", err)
			return
		}
		fmt.Println("ok")
	case "find":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: /mem find <query>")
			return
		}
		query := strings.Join(args[1:], " ")
		req := scheduler.IntentRequest{
			Kind:      scheduler.KindSlash,
			SessionID: c.sessionID,
			Owner:     c.sessionID,
			Command:   query,
			CWD:       c.cwd,
		}
		hits, err := c.k.MoeRouter.Retrieve(ctx, req, 5)
		if err != nil {
			fmt.Fprintf(os.Stderr, "retrieve: %v\n", err)
			return
		}
		if len(hits) == 0 {
			fmt.Println("(no hits)")
			return
		}
		for _, h := range hits {
			fmt.Printf("  %.2f  %s  %s\n", h.Score, h.Entry.ID, oneLine(h.Entry.Title, h.Entry.Content))
		}
	case "rm":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: /mem rm <id>")
			return
		}
		if err := c.k.MemStore.Delete(ctx, args[1]); err != nil {
			fmt.Fprintf(os.Stderr, "delete: %v\n", err)
			return
		}
		fmt.Println("ok")
	default:
		fmt.Fprintf(os.Stderr, "unknown mem subcommand: %s\n", args[0])
	}
}

// ——— shadow ———

func (c *cliState) cmdShadow(ctx context.Context, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: /shadow new|run|drop")
		return
	}
	switch args[0] {
	case "new":
		if c.shadowWS != nil {
			fmt.Fprintf(os.Stderr, "shadow already exists: %s (run /shadow drop first)\n", c.shadowWS.Path)
			return
		}
		ws, err := c.k.ShadowMgr.CreateWorkspace(ctx, c.cwd)
		if err != nil {
			fmt.Fprintf(os.Stderr, "create: %v\n", err)
			return
		}
		c.shadowWS = ws
		fmt.Printf("shadow: %s\n", ws.Path)
	case "run":
		if c.shadowWS == nil {
			fmt.Fprintln(os.Stderr, "no shadow; run /shadow new first")
			return
		}
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: /shadow run <cmd> [args...]")
			return
		}
		caps := []shadow.Capability{
			shadow.CapRead, shadow.CapWrite,
			shadow.CapBuild, shadow.CapTest, shadow.CapLint, shadow.CapFormat,
		}
		res, err := c.shadowWS.Run(ctx, caps, args[1], args[2:]...)
		if err != nil {
			fmt.Fprintf(os.Stderr, "run: %v\n", err)
			return
		}
		fmt.Printf("exit=%d duration=%s\n", res.ExitCode, res.Duration)
		if res.Stdout != "" {
			fmt.Printf("--- stdout ---\n%s\n", res.Stdout)
		}
		if res.Stderr != "" {
			fmt.Fprintf(os.Stderr, "--- stderr ---\n%s\n", res.Stderr)
		}
	case "drop":
		if c.shadowWS == nil {
			fmt.Println("(no shadow)")
			return
		}
		if err := c.shadowWS.Cleanup(); err != nil {
			fmt.Fprintf(os.Stderr, "cleanup: %v\n", err)
		}
		c.shadowWS = nil
		fmt.Println("ok")
	default:
		fmt.Fprintf(os.Stderr, "unknown shadow subcommand: %s\n", args[0])
	}
}

// ——— evolve ———

func (c *cliState) cmdEvolve(ctx context.Context, args []string) {
	title := "agentd-evolve"
	if len(args) > 0 {
		title = strings.Join(args, " ")
	}
	res, err := c.k.Evolver.Evaluate(ctx, evolve.Proposal{
		Title:  title,
		CWD:    c.cwd,
		Checks: []evolve.CheckType{evolve.CheckLint, evolve.CheckBuild, evolve.CheckTest},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "evaluate: %v\n", err)
		return
	}
	fmt.Printf("%s  (duration=%s)\n", res.Summary, res.Duration)
	for _, ch := range res.Checks {
		mark := "FAIL"
		if ch.Passed {
			mark = "PASS"
		}
		fmt.Printf("  %s  %-7s exit=%d  %s\n", mark, ch.Type, ch.ExitCode, ch.Duration)
	}
	if len(res.Learnings) > 0 {
		fmt.Println("learnings:")
		for _, l := range res.Learnings {
			fmt.Printf("  [%s] conf=%.2f  %s\n", l.Outcome, l.Confidence, l.Pattern)
		}
	}
	patterns := make([]string, 0, len(res.Learnings))
	for _, l := range res.Learnings {
		patterns = append(patterns, l.Pattern)
	}
	if len(patterns) > 0 {
		sugs := c.k.Feedback.ProposeFromEvolveResult(title, c.sessionID, res.Passed, patterns)
		if len(sugs) > 0 {
			fmt.Printf("proposed %d suggestion(s); use /fb pending to review\n", len(sugs))
		}
	}
}

// ——— feedback ———

func (c *cliState) cmdFeedback(ctx context.Context, args []string) {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: /fb pending|accept|reject|adjust ...")
		return
	}
	switch args[0] {
	case "pending":
		pend := c.k.Feedback.Pending()
		if len(pend) == 0 {
			fmt.Println("(no pending suggestions)")
			return
		}
		sort.Slice(pend, func(i, j int) bool { return pend[i].CreatedAt.Before(pend[j].CreatedAt) })
		for _, s := range pend {
			fmt.Printf("  %s  conf=%.2f  src=%s  %s\n", s.ID, s.Confidence, s.Source, s.Title)
			for _, l := range s.Learnings {
				fmt.Printf("      - %s\n", l)
			}
		}
	case "accept":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: /fb accept <id>")
			return
		}
		if _, err := c.k.Feedback.Decide(ctx, args[1], feedback.DecisionAccept, ""); err != nil {
			fmt.Fprintf(os.Stderr, "decide: %v\n", err)
			return
		}
		fmt.Println("ok")
	case "reject":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "usage: /fb reject <id>")
			return
		}
		if _, err := c.k.Feedback.Decide(ctx, args[1], feedback.DecisionReject, ""); err != nil {
			fmt.Fprintf(os.Stderr, "decide: %v\n", err)
			return
		}
		fmt.Println("ok")
	case "adjust":
		if len(args) < 3 {
			fmt.Fprintln(os.Stderr, "usage: /fb adjust <id> <text>")
			return
		}
		text := strings.Join(args[2:], " ")
		if _, err := c.k.Feedback.Decide(ctx, args[1], feedback.DecisionAdjust, text); err != nil {
			fmt.Fprintf(os.Stderr, "decide: %v\n", err)
			return
		}
		fmt.Println("ok")
	default:
		fmt.Fprintf(os.Stderr, "unknown fb subcommand: %s\n", args[0])
	}
}

// ——— state ———

func (c *cliState) cmdState() {
	fmt.Printf("session:      %s\n", c.sessionID)
	fmt.Printf("cwd:          %s\n", c.cwd)
	fmt.Printf("locks active: %d\n", c.k.LockMgr.Active())
	fmt.Printf("watchdog:     %d pending\n", c.k.Watchdog.Pending())
	if holder, ttl := c.k.LockMgr.Holder(c.sessionID); holder != "" {
		fmt.Printf("session lock: holder=%s ttl=%s\n", holder, ttl)
	}
	if c.dashAddr != "" {
		fmt.Printf("dashboard:    http://%s/dashboard/\n", c.dashAddr)
	}
	if c.shadowWS != nil {
		fmt.Printf("shadow:       %s\n", c.shadowWS.Path)
	}
	fb := c.k.Feedback.Stats()
	fmt.Printf("feedback:     pending=%d total=%d accepted=%d rejected=%d adjusted=%d\n",
		fb.Pending, fb.Total, fb.Accepted, fb.Rejected, fb.Adjusted)
}

// ——— dashboard ———

func (c *cliState) cmdDash(args []string) {
	if c.dashServer != nil {
		fmt.Printf("dashboard already running at http://%s/dashboard/\n", c.dashAddr)
		return
	}
	port := "8090"
	if len(args) > 0 {
		port = strings.TrimPrefix(args[0], ":")
		if _, err := strconv.Atoi(port); err != nil {
			fmt.Fprintf(os.Stderr, "invalid port: %s\n", args[0])
			return
		}
	}
	mux := http.NewServeMux()
	h := dashboard.NewHandler(c.k.Bus, c.k.MemStore, c.k, c.k.Feedback)
	mux.Handle("/dashboard/", h)
	addr := ":" + port
	srv := &http.Server{Addr: addr, Handler: mux, ReadHeaderTimeout: 5 * time.Second}
	errCh := make(chan error, 1)
	go func() {
		err := srv.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			errCh <- err
		}
		close(errCh)
	}()
	// Short wait so a bind failure surfaces before we claim success.
	select {
	case err := <-errCh:
		if err != nil {
			fmt.Fprintf(os.Stderr, "dashboard: %v\n", err)
			return
		}
	case <-time.After(150 * time.Millisecond):
	}
	c.dashServer = srv
	c.dashAddr = "localhost:" + port
	fmt.Printf("dashboard: http://%s/dashboard/\n", c.dashAddr)
}

// ——— helpers ———

func oneLine(title, content string) string {
	s := strings.TrimSpace(title)
	if s == "" {
		s = strings.TrimSpace(content)
	}
	s = strings.ReplaceAll(s, "\n", " ")
	if len(s) > 80 {
		s = s[:77] + "..."
	}
	return s
}

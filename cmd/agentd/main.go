package main

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"

	"mobilevc/data"
	"mobilevc/engine"
	"mobilevc/kernel"
	"mobilevc/protocol"
	"mobilevc/session"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	store, err := data.NewFileStore("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create session store: %v\n", err)
		os.Exit(1)
	}

	k := kernel.New(store)
	defer k.Stop()

	summary, err := store.CreateSession(ctx, "CLI session")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create session: %v\n", err)
		os.Exit(1)
	}
	sessionID := summary.ID

	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	svc := session.NewService(sessionID, session.Dependencies{
		NewExecRunner: k.NewExecRunner,
		NewPtyRunner:  k.NewPtyRunner,
	})
	defer svc.Cleanup()

	cli := &cliState{k: k, sessionID: sessionID, cwd: cwd}
	defer cli.shutdown()

	// cliEmit formats kernel events for terminal output.
	cliEmit := func(event any) {
		switch e := event.(type) {
		case protocol.LogEvent:
			stream := strings.TrimSpace(e.Stream)
			switch stream {
			case "stderr":
				fmt.Fprintf(os.Stderr, "%s\n", e.Message)
			default:
				fmt.Println(e.Message)
			}
		case protocol.ErrorEvent:
			fmt.Fprintf(os.Stderr, "Error: %s\n", e.Message)
			if strings.TrimSpace(e.Stack) != "" {
				fmt.Fprintf(os.Stderr, "%s\n", e.Stack)
			}
		case protocol.AgentStateEvent:
			fmt.Fprintf(os.Stderr, "[%s] %s\n", e.State, e.Message)
		case protocol.SessionStateEvent:
			fmt.Fprintf(os.Stderr, "[session] %s: %s\n", e.State, e.Message)
		case protocol.PromptRequestEvent:
			fmt.Print(e.Message)
		}
	}

	fmt.Fprintf(os.Stderr, "agentd ready (session: %s, cwd: %s)\n", sessionID, cwd)
	fmt.Fprintf(os.Stderr, "Type /help for VCOS commands, or send free text to claude. /exit to quit.\n\n")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		if handled, exit := cli.handleSlash(ctx, text); handled {
			if exit {
				break
			}
			continue
		}

		svc.RecordUserInput(text)
		err := svc.SendInputOrResume(ctx, sessionID,
			session.ExecuteRequest{
				Command: "claude",
				CWD:     cwd,
				Mode:    engine.ModePTY,
			},
			session.InputRequest{Data: text + "\n"},
			cliEmit,
		)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
	}
}

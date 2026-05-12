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

	// Create the agent kernel.
	k := kernel.New(store)

	// Create a new session.
	summary, err := store.CreateSession(ctx, "CLI session")
	if err != nil {
		fmt.Fprintf(os.Stderr, "create session: %v\n", err)
		os.Exit(1)
	}
	sessionID := summary.ID

	svc := session.NewService(sessionID, session.Dependencies{
		NewExecRunner: k.NewExecRunner,
		NewPtyRunner:  k.NewPtyRunner,
	})
	defer svc.Cleanup()

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

	fmt.Fprintf(os.Stderr, "agentd ready (session: %s)\n", sessionID)
	fmt.Fprintf(os.Stderr, "Type your message and press Enter. /exit to quit.\n\n")

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		if text == "/exit" || text == "/quit" {
			break
		}
		if text == "/help" {
			fmt.Println("/exit, /quit — quit")
			fmt.Println("/help — show this help")
			fmt.Println("anything else — send to Claude")
			continue
		}

		svc.RecordUserInput(text)

		err := svc.SendInputOrResume(ctx, sessionID,
			session.ExecuteRequest{
				Command: "claude",
				CWD:     ".",
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

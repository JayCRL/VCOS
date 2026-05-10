# AgentOS

AI CLI agent backend that wraps Claude/Codex/Gemini command-line tools into a reusable Go service — decoupled from any specific transport or frontend.

## Architecture

```
cmd/
  mobilevc/        mobile WebSocket server entry point
  agentd/          desktop CLI daemon entry point

internal/
  session/         core AI agent lifecycle (22-method Service API)
  engine/          PTY/Exec runners for Claude, Codex, Gemini
  data/            persistence (file-backed session store, claudesync, codexsync)
    skills/        skill registry and launcher
    claudesync/    Claude CLI JSONL sync
    codexsync/     Codex session sync
  gateway/         WebSocket handler, permission rules, slash commands
  protocol/        event types and wire protocol (40+ event types)
  config/          environment-based configuration
  logx/            structured logging
  adb/             Android Debug Bridge (screen streaming, touch/key input)
  push/            push notification service abstraction
  tts/             text-to-speech (ChatTTS)
```

## Core API (`session.Service`)

The core is transport-agnostic — all I/O uses `emit func(any)` callbacks.

```go
svc := session.NewService(sessionID, session.Dependencies{
    NewExecRunner: func() engine.Runner { return engine.NewExecRunner() },
    NewPtyRunner:  func() engine.Runner { return engine.NewPtyRunner() },
})
defer svc.Cleanup()

// Execute an AI CLI command (PTY mode for interactive sessions)
svc.Execute(ctx, sessionID, session.ExecuteRequest{
    Command: "claude",
    CWD:     ".",
    Mode:    engine.ModePTY,
}, func(event any) {
    // handle events: LogEvent, PromptRequestEvent, StepUpdateEvent, FileDiffEvent...
})

// Send user input
svc.SendInput(ctx, sessionID, session.InputRequest{
    Data: "rewrite this function in Rust\n",
}, emit)

// Approve/deny permission requests
svc.SendPermissionDecision(ctx, sessionID, "approve", meta, emit)

// Review code changes
svc.ReviewDecision(ctx, sessionID, session.ReviewDecisionRequest{
    Decision: "accept", // accept / revert / revise
}, emit)
```

## Quick Start

```bash
# Build everything
go build ./...

# Run tests
go test ./...

# Start mobile WebSocket server
AUTH_TOKEN=your-token go run ./cmd/mobilevc

# Start desktop CLI daemon
go run ./cmd/agentd
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `AUTH_TOKEN` | *required* | WebSocket authentication token |
| `PORT` | `8001` | Server listen port |
| `RUNTIME_DEFAULT_COMMAND` | `claude` | Default AI CLI |
| `RUNTIME_DEFAULT_MODE` | `pty` | Execution mode: `pty` or `exec` |
| `RUNTIME_WORKSPACE_ROOT` | — | Workspace root directory |
| `RUNTIME_DEBUG` | `false` | Enable debug logging |
| `TTS_ENABLED` | `false` | Enable text-to-speech |

## Go Module

```
module mobilevc
go 1.25.0
```

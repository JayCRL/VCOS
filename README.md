# AgentOS

AI agent kernel — manages Claude/Codex/Gemini CLI tools through a layered architecture of protocol, behavior primitives, and orchestration. Two entry points: WebSocket server for mobile clients, and a desktop CLI daemon.

## Architecture

```
┌──────────────────────────────────────────────────┐
│  kernel/          orchestration layer            │
│  Kernel struct · session mgmt · permission       │
│  routing · event projection · catalog sync       │
├──────────────────────────────────────────────────┤
│  session/         behavior primitives (22 methods)│
│  Service: Execute · SendInput · Permission · ... │
├──────────────────────────────────────────────────┤
│  protocol/        wire protocol (40+ event types) │
├──────────────────────────────────────────────────┤
│  engine/          PTY/Exec runners               │
│  data/            persistence + claudesync       │
└──────────────────────────────────────────────────┘

cmd/
  mobilevc/         WebSocket server (uses gateway + kernel)
  agentd/           desktop CLI daemon (uses kernel directly)

internal/
  kernel/           agent orchestration core
  session/          AI agent lifecycle primitives
  engine/           PTY/Exec runners for Claude, Codex, Gemini
  data/             persistence (file-backed store, claudesync, codexsync)
  protocol/         event types (40+ events)
  gateway/          WebSocket handler, ADB, push
  config/           environment-based configuration
  logx/             structured logging
  adb/              Android Debug Bridge
  push/             push notification service
  tts/              text-to-speech (ChatTTS)
```

## Usage

### As a library (desktop CLI / custom frontend)

```go
store, _ := data.NewFileStore("")
k := kernel.New(store)

// Create session
summary, _ := store.CreateSession(ctx, "My session")

// Use behavior primitives directly
svc := session.NewService(summary.ID, session.Dependencies{
    NewExecRunner: k.NewExecRunner,
    NewPtyRunner:  k.NewPtyRunner,
})
svc.Execute(ctx, sessionID, session.ExecuteRequest{
    Command: "claude",
    Mode:    engine.ModePTY,
}, func(event any) {
    // handle events from agent
})
```

### As a server (mobile WebSocket)

```bash
AUTH_TOKEN=your-token go run ./cmd/mobilevc
```

### As a CLI daemon

```bash
go run ./cmd/agentd
```

## Build & Test

```bash
go build ./...     # build all packages
go test ./...      # run tests
```

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `AUTH_TOKEN` | *required* | WebSocket auth token |
| `PORT` | `8001` | Listen port |
| `RUNTIME_DEFAULT_COMMAND` | `claude` | Default AI CLI |
| `RUNTIME_DEFAULT_MODE` | `pty` | Execution mode |
| `RUNTIME_DEBUG` | `false` | Debug logging |
| `TTS_ENABLED` | `false` | Text-to-speech |

## Go Module

```
module mobilevc
go 1.25.0
```

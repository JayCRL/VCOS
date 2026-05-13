# Changelog

All notable changes to VCOS will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [Unreleased]

### Added
- P0 global event bus (`kernel/eventbus`) with classified sources (User / Session / Kernel / External)
- P1 scheduling trio (`kernel/scheduler`, `kernel/watchdog`, `kernel/lockmgr`): intent scheduler, watchdog, exclusive session lock
- P2 Memory + MOE (`memory`, `memory/moe`): MemStore / SQLiteStore + Type × Domain rule-based routing
- P2 Semantic embedder (`memory/semantic`): Chunker / Embedder / SymbolIndex / FlowAnalyzer layered interfaces + NaiveChunker + HashEmbedder zero-dep defaults
- P3 Interaction & cognition layer (`cognition/intake`, `cognition/vibe`, `cognition/dashboard`): cognitive intake, vibe controller, HTTP dashboard with SSE + memory CRUD + task dispatch + feedback API
- P4 Shadow workspace (`evolution/shadow`): `git worktree`-based isolation with capability allowlist
- P4 Evolution engine (`evolution/evolve`): build/test/lint evaluator in shadow, learnings distilled to long-term memory
- P5 Feedback closed loop (`evolution/feedback`): propose → accept/reject/adjust → write back to Memory with adjustable confidence
- Kernel orchestration core (`kernel/kernel.go`): wires all P0–P5 modules as first-class fields on the `Kernel` struct
- P1–P5 end-to-end integration tests (`kernel/p1_integration_test.go`)
- Dashboard memory editor endpoints (POST /memory, GET /memory/{id}, DELETE /memory/{id})
- Dashboard console exec endpoint (POST /dashboard/console/exec) for task dispatch
- Dashboard feedback API (pending / history / stats / decide)

### Changed
- Reorganized codebase from flat `internal/` to 5-layer architecture tree (`cognition/`, `kernel/`, `memory/`, `evolution/`, `session/`, `engine/`, `protocol/`, `data/`, `gateway/`, `support/`)
- Renamed project AgentOS → VibeOS → VCOS

### Fixed
- EventBus data race: `close(channel)` vs `channel<-send` by using stop channel instead of closing subscriber channel directly

## [0.1.0] — 2026-05-12

### Added
- Initial commit with AI CLI agent backend
- `session/` behavior primitives: Execute, SendInput, Permission, Projection
- `engine/` PTY / Exec runners for Claude, Codex, Gemini
- `protocol/` wire protocol: 40+ session-scoped events + EventCursor
- `data/` file-backed persistence + claudesync / codexsync
- `gateway/` WebSocket handler + ADB + push notifications
- `support/tts` text-to-speech via ChatTTS
- `cmd/mobilevc` WebSocket server for mobile clients
- `cmd/agentd` desktop CLI daemon
- Kernel orchestration shell (`kernel/kernel.go`, `kernel/registry.go`, `kernel/session_mgmt.go`)

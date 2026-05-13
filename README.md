<div align="center">

# VCOS

**事件驱动 AI Agent 操作系统内核**

为 Claude / Codex / Gemini CLI 提供调度、记忆、进化与反馈闭环的完整运行环境。

[![Go](https://img.shields.io/badge/Go-1.25%2B-00ADD8?logo=go)](https://golang.org)
[![Status](https://img.shields.io/badge/status-alpha-orange)](#阶段进度)
[![Arch](https://img.shields.io/badge/arch-5%20layer%20%2B%20dual%20track-blue)](docs/design/architecture.md)
[![Tests](https://img.shields.io/badge/tests-passing-brightgreen)](#构建)

[架构总览](docs/design/architecture.md) · [快速开始](#快速开始) · [模块地图](#模块地图)

</div>

---

## 架构

五层 + 调度/Memory 两条贯穿主线:

```
层1  cognition/  交互与认知接口  → intake · vibe · dashboard
层2  kernel/     事件驱动调度内核  → eventbus · scheduler · watchdog · lockmgr
层3  memory/     语义与记忆存储    → memory · moe · semantic
层4  evolution/  影子空间进化引擎  → shadow · evolve
层5  evolution/  用户反馈闭环      → feedback
基础 session/ engine/ protocol/ data/ gateway/ support/
```

> 架构全景图见 [`docs/assets/architecture.png`](docs/assets/architecture.png) · 设计信源 [`docs/design/architecture.md`](docs/design/architecture.md)

## 层 1 · cognition/ — 交互与认知接口

处理用户输入、推断意图、管理偏好、提供可视化控制面。

| 包 | 职责 |
|---|---|
| `cognition/intake` | 会话级认知构建 — 探测项目类型、推断角色/风格、植入初始 Memory |
| `cognition/vibe` | 有状态偏好控制器 — Style / Proactivity / Role 读写 |
| `cognition/dashboard` | HTTP 观测面 — 事件 SSE、记忆编辑器、任务分发、反馈裁定 |

```go
k.Intake.Run(ctx, sessionID, cwd, "review the auth module") // 认知分析
k.Vibe.Set(ctx, sessionID, vibe.State{Style: "concise", Role: "reviewer"})
```

## 层 2 · kernel/ — 事件驱动内核

全局事件总线 + 调度三件套 + 会话编排。

| 包 | 职责 |
|---|---|
| `kernel` | 编排核心 — 把所有子系统 wire 为 `Kernel` 的一等字段 |
| `kernel/eventbus` | 全局事件总线 — SourceUser / Session / Kernel / External 分类 |
| `kernel/scheduler` | 意图调度器 — 准入/延后/冲突裁决 |
| `kernel/watchdog` | 进程看门人 — 空闲/超时检测,自动强制释放 |
| `kernel/lockmgr` | 锁管理器 — 单会话独占锁,超时自动释放 |

```go
k := kernel.New(store)
d := k.Scheduler.Decide(scheduler.IntentRequest{Kind: "exec", SessionID: "s1", Owner: "e1"})
k.Watchdog.Watch(watchdog.WatchOptions{ExecutionID: "e1", Timeout: 30 * time.Second})
```

## 层 3 · memory/ — 语义与记忆

Memory 存储 + MOE 路由 + 代码语义分层。

| 包 | 职责 |
|---|---|
| `memory` | Entry / Store 实体 + MemStore / SQLiteStore 实现 |
| `memory/moe` | Type × Domain 二维 MOE 路由器 |
| `memory/semantic` | L1 Chunker/Embedder/Summarizer + L2 SymbolIndex + L3 FlowAnalyzer |

```go
k.MoeRouter.WriteMemory(ctx, req, "auth bug: infinite redirect in OAuth callback")
hits, _ := k.MoeRouter.Retrieve(ctx, req, 5)
_ = k.Semantic.EmbedFile("main.go", content) // 代码分片 → 向量
```

## 层 4 · evolution/ — 影子空间与进化

基于 `git worktree` 的物理隔离执行器 + 评估与学习提炼。

| 包 | 职责 |
|---|---|
| `evolution/shadow` | 影子工作区 — `git worktree` 创建/清理/文件变更 |
| `evolution/evolve` | 评估器 — 跑 build/test/lint,提炼学习 |

```go
ws, _ := k.ShadowMgr.CreateWorkspace(ctx, projectDir)
result, _ := k.Evolver.Evaluate(ctx, evolve.Proposal{Title: "fix-nil", Checks: []evolve.CheckType{evolve.CheckBuild}})
// result.Passed, result.Learnings → 写回 Memory
```

## 层 5 · evolution/feedback — 反馈闭环

将进化结果提案给用户,裁定后写回 Memory。

| 包 | 职责 |
|---|---|
| `evolution/feedback` | Propose → Accept/Reject/Adjust → 写回 Memory(置信度可调) |

```go
suggestions := k.Feedback.ProposeFromEvolveResult(...)
k.Feedback.Decide(ctx, suggestions[0].ID, feedback.DecisionAccept, "")
```

## 基础层

| 包 | 职责 |
|---|---|
| `session` | AI agent 行为原语 — Execute / SendInput / Permission / Projection |
| `engine` | PTY / Exec 运行器 — Claude / Codex / Gemini 统一封装 |
| `protocol` | 线协议 — 40+ session-scoped 事件 + EventCursor |
| `data` | 持久化 — FileStore + Claudesync / Codexsync |
| `gateway` | WebSocket 传输 + 鉴权 + 推送 / ADB 桥接 |
| `support/config` | 环境变量配置 |
| `support/logx` | 结构化日志 |
| `support/adb` | Android Debug Bridge |
| `support/push` | APNs / FCM 推送 |
| `support/tts` | ChatTTS 语音合成 |

## 阶段进度

| 阶段 | 内容 | 包 | 状态 |
|---|---|---|---|
| P0 | 全局事件总线 | `kernel/eventbus` | ✅ |
| P1 | 调度三件套 | `kernel/{scheduler,watchdog,lockmgr}` | ✅ |
| P2 | Memory + MOE + 语义 | `memory` `memory/moe` `memory/semantic` | ✅ 骨架 · ⏳ Tree-sitter/SCIP/Joern 后端 |
| P3 | 交互与认知接口 | `cognition/{intake,vibe,dashboard}` | ✅ |
| P4 | 影子空间 + 进化 | `evolution/{shadow,evolve}` | ✅ |
| P5 | 反馈闭环 | `evolution/feedback` | ✅ |

## 快速开始

```bash
git clone https://github.com/JayCRL/VCOS.git && cd VCOS
go build ./...

# 启动 WebSocket 服务器(移动端)
AUTH_TOKEN=dev go run ./cmd/mobilevc

# 启动 CLI 守护进程(桌面)
go run ./cmd/agentd
```

### 作为库使用

```go
k := kernel.New(store)
defer k.Stop()

svc := session.NewService(sessionID, session.Dependencies{
    NewExecRunner: k.NewExecRunner,
    NewPtyRunner:  k.NewPtyRunner,
})
svc.Execute(ctx, sessionID, session.ExecuteRequest{
    Command: "claude", Mode: engine.ModePTY,
}, func(event any) { /* 处理协议事件 */ })
```

### Dashboard

```go
mux.Handle("/dashboard/", dashboard.NewHandler(k.Bus, k.MemStore, k, k.Feedback))
```

| 端点 | 方法 | 用途 |
|---|---|---|
| `/dashboard/events?stream=1` | GET SSE | 实时事件流 |
| `/dashboard/memory` | GET/POST | 记忆列表 / 编辑 |
| `/dashboard/memory/{id}` | GET/DELETE | 单条读取 / 删除 |
| `/dashboard/console/exec` | POST | 派发 AI 指令(任务分发) |
| `/dashboard/feedback/pending` | GET | 待裁定建议 |
| `/dashboard/feedback/decide` | POST | 接受/拒绝/调整 |

## 模块地图

```
VCOS/
├── cognition/        # 层1: 交互与认知接口
│   ├── intake/       #   会话认知构建
│   ├── vibe/         #   偏好控制器
│   └── dashboard/    #   HTTP 可视化控制面
├── kernel/           # 层2: 事件驱动内核
│   ├── eventbus/     #   全局事件总线 [P0]
│   ├── scheduler/    #   意图调度器 [P1]
│   ├── watchdog/     #   进程看门人 [P1]
│   └── lockmgr/      #   锁管理器 [P1]
├── memory/           # 层3: 语义与记忆
│   ├── moe/          #   MOE 路由器 [P2]
│   └── semantic/     #   代码语义分层栈 [P2]
├── evolution/        # 层4+5: 进化与反馈
│   ├── shadow/       #   影子空间 [P4]
│   ├── evolve/       #   进化评估器 [P4]
│   └── feedback/     #   反馈闭环 [P5]
├── session/          # AI agent 行为原语
├── engine/           # PTY/Exec 运行器
├── protocol/         # 线协议事件
├── data/             # 持久化 + 同步
├── gateway/          # WebSocket 传输 + ADB + 推送
├── support/          # 基础设施
│   ├── config/       #   环境变量配置
│   ├── logx/         #   结构化日志
│   ├── adb/          #   Android Debug Bridge
│   ├── push/         #   推送通知
│   └── tts/          #   语音合成
├── cmd/
│   ├── mobilevc/     # WebSocket 服务器入口
│   └── agentd/       # CLI 守护进程入口
└── docs/
    ├── design/       # 架构设计文档
    └── assets/       # 架构图
```

## 构建

```bash
go build ./...     # 全量编译
go test ./...      # 全量测试
```

## 配置

| 变量 | 默认 | 说明 |
|---|---|---|
| `AUTH_TOKEN` | *必填* | WebSocket 鉴权 |
| `PORT` | 8001 | 监听端口 |
| `RUNTIME_DEFAULT_COMMAND` | claude | 默认 AI CLI |
| `RUNTIME_DEFAULT_MODE` | pty | 执行模式 |
| `RUNTIME_DEBUG` | false | 调试日志 |
| `TTS_ENABLED` | false | 语音合成 |

## 设计决策

| 决策 | 选型 | 说明 |
|---|---|---|
| CPG | Tree-sitter 常驻 + SCIP 按需 + Joern 备胎 | 分层,拒绝 JVM 常驻 |
| Memory 存储 | SQLite + sqlite-vec,接口预留 LanceDB | 单机 10w-100w 条目,零外部进程 |
| 影子隔离 | `git worktree` + 能力白名单 | <100ms,跨平台,零容器 |
| MOE 路由 | Type × Domain 二维规则路由 | 预留 CognitiveKind 学习升级位 |

---

<div align="center">
  <sub>VCOS · 把 AI 编码代理做成一台操作系统</sub>
</div>

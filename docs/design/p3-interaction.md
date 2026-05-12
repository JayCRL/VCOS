# P3:交互与认知接口层

## 1. 目标

补齐架构图第 1 层"交互与认知接口层":新会话初始诉求分析(认知构建)、Vibe 偏好控制器、Dashboard 可视化监控台。

## 2. 包结构

```
internal/intake/
  intake.go         # SessionIntake: 项目探测 + 初始 Memory 播种
  intake_test.go

internal/vibe/
  vibe.go           # Controller: 偏好状态机, memory 持久化
  vibe_test.go

internal/dashboard/
  dashboard.go      # HTTP Handler: SSE 事件流 / sessions / memory / decisions
```

## 3. Intake(认知构建)

**流程**:`Analyze(CWD, prompt) → CognitiveProfile → Bootstrap(sessionID, memory entries)`

| 步骤 | 输出 |
|---|---|
| `detectProject` | 扫描 16 种项目签名(go.mod, package.json, Cargo.toml...),**首匹配优先** |
| `inferRole` | prompt 含 review/audit→reviewer, design/architect→architect, fix/bug→debugger, 默认 developer |
| `inferStyle` | prompt 含 concise/brief→concise, detailed/verbose/explain→verbose, 默认 balanced |
| `Bootstrap` | 写 3 条初始 Memory:项目上下文(project+code), 会话目标(short_term+task), 偏好(long_term+dialogue) |

**Kernel 集成**:`CreateSession` 尾部自动调 `Intake.Run(ctx, sessionID, cwd, title)`。

## 4. Vibe Controller

**State**:`Style × Proactivity × Role × UpdatedAt`

**持久化**:以 `memory.Entry{ID:"vibe-state-<sessionID>", Type:long_term, Domain:dialogue}` 存到 MemStore,跨 Controller 实例可读。

**Set/Get**:内存缓存 + store 直读;`UpdateStyle/Proactivity/Role` 便捷方法。

**Tags**:`Controller.Tags(ctx, sessionID)` → `["vibe:style=concise", "vibe:proactivity=active", "vibe:role=debugger"]`

**Kernel 集成**:作为 `scheduler.Rule` 注入,在 MOE 之后、锁检查之前,自动给 `IntentRequest.Tags` 附加 vibe 标签。

## 5. Dashboard

**路由**:

| 路径 | 方法 | 内容 |
|---|---|---|
| `GET /dashboard/` | HTML | 监控首页,链接到各端点 |
| `GET /dashboard/events` | JSON | 最近 100 条事件 |
| `GET /dashboard/events?stream=1` | SSE | 实时事件流 |
| `GET /dashboard/sessions` | JSON | `Kernel.ActiveSessions()` |
| `GET /dashboard/memory?type=project` | JSON | 记忆总览(按类型过滤) |
| `GET /dashboard/decisions` | JSON | 调度决策记录 |

**集成方式**:`NewHandler(bus, memStore, kernel)` 返回 `http.Handler`,外部挂到 mux。

## 6. Kernel 新增接口

```go
Kernel.ActiveSessions() []string              // registry 的所有 sessionID
Registry.SessionIDs() []string                // 内部实现
```

## 7. 端到端流程(P3)

```
CreateSession(title="design cache", cwd="/app")
  → Intake.Run → detectProject("/app") → go项目
  → inferRole("design") → architect
  → Bootstrap 3 条 Memory(project context + goal + preference)

客户端请求 execute
  → Scheduler.Decide
  → MOE Rule: 查 Type=project Domain=code → 注入 moe:hit=...
  → Vibe Rule: Tags ← [vibe:style=..., vibe:role=architect, ...]
  → LockMgr.Acquire → Admit
  → Watchdog.Watch
  → [执行...bus heartbeat...]
  → Watchdog.Settle + Lock.Release
```

## 8. 验收

- `go build ./...` ✅
- `go test ./internal/intake/...` ✅(项目探测 / role/style 推断 / Bootstrap)
- `go test ./internal/vibe/...` ✅(默认值 / Set+Get / Tags / 缓存刷新 / 跨实例持久化)
- `go test ./internal/kernel/...` ✅(P3 集成 4 条:Intake 种子 / Vibe 调度 / ActiveSessions / 全认知链)
- `go test ./internal/eventbus/...` ✅(P0 仍工作)
- `go test ./internal/memory/...` ✅(P2 仍工作)

## 9. 不做

- Dashboard WebSocket 推送(已有 SSE)
- 可视化 UI 组件(只给 HTML 骨架)
- Vibe 自动学习/自适应(留 P5 反馈闭环)
- Intake 深度语义分析(留 P4 进化引擎)

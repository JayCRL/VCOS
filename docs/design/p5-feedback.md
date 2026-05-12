# P5:用户反馈闭环

## 1. 目标

实现架构图第 5 层——"用户调优与反馈"完整闭环:进化建议展示 → 用户确认/拒绝/调整 → MOE 权重更新 → 写回系统生效。

## 2. 包结构

```
internal/feedback/
  feedback.go       # Controller: Propose / Decide / Stats / History
  feedback_test.go
```

## 3. 数据模型

**Suggestion** — 进化引擎产出的待审建议:
```go
type Suggestion struct {
    ID, Title, Description string
    Learnings   []string  // 提炼的模式
    Confidence  float64   // 0-1
    Source      string    // "evolve" / "intake" / "manual"
    SessionID   string
}
```

**Decision** — 用户对建议的决策:
```
accept  → 写入 LongTerm+Code 记忆,status=accepted,confidence 保持
reject  → 写入 LongTerm+Code 记忆,status=rejected,confidence=0.1
adjust  → 写入 LongTerm+Code 记忆,status=adjusted,confidence=0.8,记录原始模式
```

**Record** — 决策历史:
```go
type Record struct {
    SuggestionID string
    Decision     Decision
    AdjustedText string    // adjust 时用户修改的文本
    Timestamp    time.Time
}
```

## 4. Controller API

```go
c := feedback.New(memStore)

// 从进化结果生成建议
suggestions := c.ProposeFromEvolveResult(title, sessionID, passed, learnings)

// 用户决策
rec, err := c.Decide(ctx, suggestionID, DecisionAccept, "")

// 查询
c.Pending()   // 待审建议
c.History()   // 决策历史
c.Stats()     // {Total, Accepted, Rejected, Adjusted, Pending}
```

## 5. Kernel 集成

```go
Kernel.Feedback *feedback.Controller
```

`kernel.New` 默认创建 `feedback.New(memStore)`,与其他组件共享同一个 Memory Store。

## 6. 完整闭环(P0-P5)

```
用户输入 → CreateSession → Intake(认知构建 + 初始 Memory)
  → Vibe(偏好设置)
  → Scheduler.Decide
    → MOE Rule(检索相关记忆,富化 Tags)
    → Vibe Rule(附加偏好标签)
    → LockMgr.Acquire(资源锁)
  → Admit
  → Watchdog.Watch(守护执行)

[Shadow 影子空间]
  → ShadowMgr.CreateWorkspace(git worktree 隔离)
  → Evolver.Evaluate(ApplyDiff + go build + go test + 提炼 Learnings)
  → Memory Store.Upsert(初始学习写入 long_term + code)

[Feedback 反馈闭环]
  → Feedback.ProposeFromEvolveResult(生成建议)
  → 用户审查 → Decide(Accept / Reject / Adjust)
  → Memory Store.Upsert(confirmed/rejected/adjusted 模式)
  → MOE 后续检索时提升/降低对应 confidence

  → Watchdog.Settle + LockMgr.Release
```

## 7. 验收

- `go build ./...` ✅
- `go test ./internal/feedback/...` ✅(Propose/Decide Accept/Reject/Adjust + Pending/Stats/History)
- `go test ./internal/kernel/...` ✅(P5 集成 4 条:Accept / Reject / Adjust / 全闭环 7 步)
- 全部 P0-P5 包:18/18 测试通过

## 8. 架构图完整对照

```
✅ P0 事件总线         ✅ P1 调度三件套       ✅ P2 Memory + MOE
✅ P3 Intake           ✅ P3 Vibe             ✅ P3 Dashboard
✅ P4 Shadow 影子空间   ✅ P4 Evolve 进化引擎  ✅ P5 Feedback 反馈闭环
⬜ engine/gateway 历史 fail 修复(非架构图范围)
```

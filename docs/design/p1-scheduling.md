# P1:调度三件套(scheduler / watchdog / lockmgr)

## 1. 目标

在 P0 事件总线上叠加 VibeOS 的"内核调度层"。三个子模块互相正交:

| 子模块 | 角色 | 依赖 |
|---|---|---|
| `lockmgr` | 资源独占 + TTL + 重入锁 | 无 |
| `watchdog` | 进程看门人,空闲超时报警 + 强制释放锁 | `eventbus` + `lockmgr` |
| `scheduler` | 意图调度器,准入决策 + 自动加锁 | `lockmgr` |

> 故意**不在主链路里强制接入**:scheduler 是 advisor,watchdog 是侧路监听,lockmgr 是工具。Kernel 暴露这三个组件,业务方按需调用。这给了 P2 (Memory MOE) 注入决策的余地,而不必现在重写 ExecuteAICommand。

## 2. 包结构

```
internal/kernel/
  lockmgr/
    lockmgr.go
    lockmgr_test.go
  watchdog/
    watchdog.go
    watchdog_test.go
  scheduler/
    scheduler.go
    scheduler_test.go
  p1_integration_test.go      # 三件套配合的端到端 demo
```

## 3. lockmgr

**形态**:string-key 锁,(key, owner) 唯一,TTL 到期自动释放并发广播 `Expired()` 通道。

**重入**:同 owner 再次 Acquire 不冲突,depth 累加;每次 Acquire 必须配一次 Release。

**强制释放**:`ForceRelease(key)` 让 watchdog 在超时后无视 owner 释放。

**API**

```go
m := lockmgr.New(nil)        // 默认 time.Now
defer m.Close()

lease, err := m.Acquire("sess-X", "exec-1", 30*time.Second)
// err == lockmgr.ErrConflict 表示别的 owner 持有
defer m.Release(lease.Key, lease.Owner)

m.ForceRelease("sess-X")     // 看门人路径
m.Holder("sess-X")           // ("exec-1", remainingTTL)
```

## 4. watchdog

**形态**:per-execution 空闲超时计时器。任何 SourceSession / SourceKernel 事件携带 ExecutionID 都会自动 heartbeat;Settle 主动结束;超时则发 `protocol.ErrorEvent{Code:"watchdog_timeout"}` 到 Bus,**并 ForceRelease 关联锁**。

**P1 不做强 kill** —— 仅警示 + 释放锁。强 kill 留给后续阶段(需要看门人能可靠拿到 engine.Runner 句柄)。

**MinIdleTimeout** 默认 5s 避免计时器风暴;`NewWithConfig` 可以下调用于测试。

**API**

```go
w := watchdog.New(bus, lm)   // 自动 Subscribe(SourceSession+SourceKernel)
defer w.Close()

w.Watch(watchdog.WatchOptions{
    ExecutionID: "exec-1",
    SessionID:   "sess-X",
    LockKey:     "sess-X",
    Timeout:     5 * time.Minute,
})
// ... 事件流自动 heartbeat ...
w.Settle("exec-1")           // 正常完成
```

## 5. scheduler

**形态**:`Decide(IntentRequest) Decision`。流程:

1. 串行跑已注册 `Rule`,**第一个非 OutcomeAdmit 的决策胜出**
2. 默认锁逻辑:`ResourceKey` 缺失则 fallback 到 `SessionID`
3. 调 `lockmgr.Acquire`:成功 → `OutcomeAdmit`;`ErrConflict` → `OutcomeDefer` + 当前 holder

**Outcome 取值**:`admit` / `deny` / `defer` / `need_confirm`(后者留给后续接确认 UI)

**Memory MOE 注入点**:P2 中 MOE 将作为 `Rule` 注册:Rule 看到 `IntentRequest` 后注入"领域+类型"路由结果到 `Tags`,但仍由 Decide 主流程做最终准入。

**API**

```go
s := scheduler.New(lm, scheduler.Config{DefaultLockTTL: 10*time.Minute})
s.AddRule(func(req scheduler.IntentRequest) scheduler.Decision {
    if req.Engine == "blocked-engine" {
        return scheduler.Decision{Outcome: scheduler.OutcomeDeny, Reason: "engine disabled"}
    }
    return scheduler.Decision{}
})

d := s.Decide(scheduler.IntentRequest{
    Kind: scheduler.KindExec, SessionID: "sess-X", Owner: "exec-1",
})
// d.Outcome / d.LockKey / d.Conflict
```

## 6. Kernel 集成

`Kernel` 新增字段:

```go
LockMgr   *lockmgr.Manager
Watchdog  *watchdog.Watchdog
Scheduler *scheduler.Scheduler
```

`kernel.New(store)` 默认创建并互联三者。新增 `Kernel.Stop()` 用于关闭 bus / lockmgr / watchdog。

**端到端示例**(见 `p1_integration_test.go`):

```
Scheduler.Decide → admit + lock("sess-A", "exec-1")
Watchdog.Watch(exec-1, lockKey="sess-A", timeout=30s)
[bus 事件流]    → 自动 heartbeat
Watchdog.Settle(exec-1)
Scheduler.Release("sess-A", "exec-1")
```

或者超时路径:

```
Watchdog 超时 → publish ErrorEvent{Code: watchdog_timeout}
              → ForceRelease("sess-A")
```

## 7. 验收

- `go build ./...` ✅
- `go test ./internal/kernel/...` ✅(含三件套 + 集成)
- `go test ./internal/eventbus/...` ✅(P0 仍工作)

## 8. 显式不做(留 P2+)

- Scheduler 自动接管 `ExecuteAICommand` —— 仍由调用方显式 `Decide`(避免在没有 MOE 的情况下做错决策)
- Watchdog 强 kill —— 需要看门人拿到 `engine.Runner` 句柄,P2/P3 再做
- 跨进程锁(Redis 等)—— 单机 VibeOS 不需要
- 锁等待队列 —— Defer 后由调用方决定重试,scheduler 不入队

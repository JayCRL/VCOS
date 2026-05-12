# P2:Memory + 语义层

## 1. 目标

补齐架构图"语义与存储层":Memory 实体 + 持久化 + MOE 二维路由 + 语义嵌入器骨架。MOE 以 `scheduler.Rule` 形式注入调度器决策链,实现"意图 → 记忆检索 → 上下文富化 → 准入"。

## 2. 包结构

```
internal/memory/
  types.go           # Entry / Type / Domain / CognitiveKind / Filter / Hit
  store.go           # Store interface
  memstore.go        # 内存实现 (单测 + 小负载)
  sqlite_store.go    # SQLite 实现 (modernc.org/sqlite, WAL + 超时)
  store_test.go      # 共享测试套件 (testStore), 覆盖两种实现
  moe/
    moe.go           # Router: Classify / Retrieve / WriteMemory / SchedulerRule
    moe_test.go

internal/semantic/
  embedder.go        # Embedder + Summarizer 接口 + Noop 实现
```

## 3. 数据模型

**MemoryEntry** = (Type × Domain × CognitiveKind) 三维标签 + Content + Metadata + Embedding + TTL。

| 维度 | 值 | 用途 |
|---|---|---|
| Type | short_term / long_term / project | MOE 一级路由:写策略 + TTL |
| Domain | code / dialogue / task | MOE 二级路由:检索通道 |
| CognitiveKind | episodic / semantic / procedural / working | 预留,未参与规则路由 |

**Store interface**:

```go
type Store interface {
    Upsert(ctx, Entry) error
    Get(ctx, id string) (Entry, error)
    Delete(ctx, id string) error
    Query(ctx, Filter) ([]Entry, error)
    QuerySimilar(ctx, text string, f Filter, k int) ([]Hit, error)
    Count(ctx, Filter) (int, error)
    PurgeExpired(ctx) (int, error)
}
```

实现:
- `MemStore` — sync.Map,文本相似用 word overlap(TF-IDF 降级)
- `SQLiteStore` — `modernc.org/sqlite`(已在 go.mod),WAL 模式,embedding 存 BLOB

## 4. MOE 路由器

**Classify 规则**(P2 用启发式,不上分类模型):

| 条件 | Type | Domain |
|---|---|---|
| CWD 非空 | project | — |
| 默认 | short_term | — |
| cmd 含 task/todo/plan/schedule/check/status | — | task |
| cmd 含 code/refactor/fix/bug/test/build/deploy/review | — | code |
| 否则 | — | dialogue |

**SchedulerRule**: 作为 `scheduler.Rule` 注入,调用 `Retrieve` 查找相关记忆,用 `req.Tags` 返回 `moe:hit=<id>`,后续规则或调度器可据此决策。Rule 始终返回 `OutcomeAdmit`(只富化,不拦截)。

**WriteMemory**: "初始 Memory 写入"路径。从 IntentRequest 生成 Entry,Upsert 到 Store。

## 5. 语义嵌入器(骨架)

```go
type Embedder interface {
    Embed(text string) ([]float32, error)
    Dim() int
}
```

`NoopEmbedder` 始终返回 nil,供 Memory 层在不接入真实模型时工作。真实 Tree-sitter + embedding model 集成留给 P3+。

## 6. Kernel 集成

```go
Kernel.MemStore  memory.Store          // 内存实现,可替换为 SQLiteStore
Kernel.MoeRouter *moe.Router           // 持有 Store 引用

// kernel.New(store) 默认:
// - MemStore = memory.NewMemStore()
// - MoeRouter = moe.NewRouter(MemStore)
// - Scheduler.AddRule(MoeRouter.SchedulerRule(3))
```

## 7. 端到端流程

```
User Intent → Scheduler.Decide
  → MOE Rule: Classify(type=project, domain=code)
  → MOE Rule: Retrieve(CWD="/proj", query="refactor") → [hit: mem-code-1]
  → MOE Rule: Tags enriched → ["moe:hit=mem-code-1"]
  → Scheduler lock check → Acquire(sess-X, exec-1) → Admit
  → Watchdog.Watch(exec-1, lockKey="sess-X")
  → [execution via session.Service, bus heartbeats]
  → Watchdog.Settle → Lock.Release
```

## 8. 验收

- `go build ./...` ✅
- `go test ./internal/memory/...` ✅(MemStore + SQLiteStore + emb roundtrip)
- `go test ./internal/memory/moe/...` ✅(Classify / Retrieve / WriteMemory / SchedulerRule)
- `go test ./internal/kernel/...` ✅(P1+P2 集成 6 条路径全通过)
- `go test ./internal/eventbus/...` ✅(P0 仍工作)

## 9. 不做(留 P3+)

- Tree-sitter CPG / 真实 embedding / SCIP indexer
- MOE 分类模型(仍用规则)
- 向量相似检索(sqlite-vec 集成)
- Memory 自动归档 / summarization pipeline

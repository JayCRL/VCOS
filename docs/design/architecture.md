# VCOS 架构设计

> 来源:[`docs/assets/architecture.png`](assets/architecture.png)(模块依赖与数据流示意图,进阶版)
> 进度版本:2026-05-12(P0–P5 骨架全部落地,语义嵌入器真实后端待接)

## 1. 模块全景

5 层 + 双主线(调度 / Memory):

```
┌─ 1. 交互与认知接口层 ──────────────────────────────────────┐
│  初始诉求 & 认知构建 · Vibe 控制器 · 记忆编辑器           │
│  任务分发器 · 可视化与监控台                              │
├─ 2. 内核调度层(事件驱动)─────────────────────────────────┤
│  全局事件队列(用户 / 会话 / 内核 / 外部)                │
│  意图调度器(规划) · 进程看门人 · 锁管理器               │
├─ 3. 语义与存储层 ──────────────────────────────────────────┤
│  语义嵌入器(CPG / 摘要 / 影响分析) · Memory MOE         │
├─ 4. 进化引擎(影子空间)──────────────────────────────────┤
│  影子空间(隔离副本) · 进化与反向传播                    │
├─ 5. 用户调优与反馈 ────────────────────────────────────────┤
│  进化建议展示 · 用户确认 · 反馈应用 · 写回系统生效        │
└────────────────────────────────────────────────────────────┘
```

## 2. 现状对照

| 架构图模块 | 代码包 | 状态 |
|---|---|---|
| 引擎 | `internal/engine` | ✅ |
| 会话原语 | `internal/session` | ✅ |
| 网关 | `internal/gateway` | ✅ |
| 数据持久化 | `internal/data` | ✅ |
| 协议 | `internal/protocol` | ✅(40+ session-scoped 事件 + EventCursor) |
| 内核(壳) | `internal/kernel` | ✅ 已串联 eventbus / scheduler / watchdog / lockmgr / memory / moe / intake / vibe / shadow / evolve / feedback / semantic;P1–P5 集成测试覆盖 |
| 全局事件队列 | `internal/eventbus` | ✅ |
| 意图调度器 / 看门人 / 锁管理器 | `internal/kernel/{scheduler,watchdog,lockmgr}` | ✅ |
| Memory + MOE | `internal/memory` + `internal/memory/moe` | ✅(MemStore + SQLiteStore;二维路由) |
| 语义嵌入器 | `internal/semantic` | ⚠️ 分层接口齐全(Chunker / Embedder / Summarizer / SymbolIndex / FlowAnalyzer)+ 默认 NaiveChunker + HashEmbedder + Noop 后端;Tree-sitter / SCIP / Joern **真实集成留 TODO** |
| 影子空间 / 进化引擎 | `internal/shadow` + `internal/evolve` | ✅ |
| 用户反馈闭环 | `internal/feedback` | ✅ |
| 初始诉求 & 认知构建 | `internal/intake` | ✅ |
| Vibe 控制器 | `internal/vibe` | ✅ |
| 可视化与监控台 | `internal/dashboard` | ✅(事件 SSE + 会话 + 记忆 CRUD + 调度决策 + 反馈) |
| 记忆编辑器 | `internal/dashboard`(/memory POST/GET/DELETE) | ✅ |
| 任务分发器 | `internal/kernel/scheduler` + `internal/dashboard`(/console/exec) | ✅ |

剩余缺口:
- `internal/semantic` 真实后端(Tree-sitter 常驻、SCIP 后台预热、Joern 按需拉起)— 当前是 hash-based 占位,接口已定型,替换不破坏 call-site。

## 3. 5 阶段路线图

| Phase | 范围 | 包 | 状态 |
|---|---|---|---|
| **P0** | 全局事件总线 | `internal/eventbus` | ✅ |
| **P1** | 调度三件套(意图调度 / 看门人 / 锁管理) | `internal/kernel/{scheduler,watchdog,lockmgr}` | ✅ |
| **P2** | Memory + 语义嵌入器 + MOE | `internal/memory`, `internal/semantic` | ✅ 骨架(语义嵌入器待接 Tree-sitter / SCIP) |
| **P3** | 交互/认知接口层 | `internal/intake`, `internal/vibe`, `internal/dashboard` | ✅ |
| **P4** | 影子空间 + 进化引擎 | `internal/shadow`, `internal/evolve` | ✅ |
| **P5** | 反馈闭环 | `internal/feedback` | ✅ |

## 4. 选型决策

### 4.1 CPG / 代码语义图

**选:Tree-sitter 常驻 + SCIP 按需 + Joern 备胎(分层)**

| 层 | 工具 | 用途 | 频率 |
|---|---|---|---|
| L1 | Tree-sitter | AST 切片、增量摘要、嵌入 chunking | 常驻、每次写入 |
| L2 | SCIP indexer | 跨文件 symbol/调用图 | 后台预热、按项目周期 |
| L3 | Joern | 数据流/控制流深度分析 | 按 query 拉起,**不常驻** |

不选 Joern 全量:JVM、GB 级内存、启动 2-5s,常驻不现实。
不选纯 LSP:协议偏 IDE 交互,批量分析吃力。

### 4.2 Memory 存储

**选:SQLite + sqlite-vec,接口预留 LanceDB**

```go
type Store interface {
    Upsert(ctx, MemoryEntry) error
    Query(ctx, embedding []float32, filter Filter, k int) ([]Hit, error)
    Get(ctx, id string) (MemoryEntry, error)
    Delete(ctx, id string) error
}
```

理由:
- 单用户单机场景,量级估算 10w-100w 条目,SQLite 完全够用
- `mattn/go-sqlite3` + sqlite-vec ext,零外部进程
- 接口抽象后,LanceDB 可平替
- **明确排除** Chroma:为向量库再起 Python sidecar 不值;Qdrant/Weaviate 部署过重

### 4.3 影子空间隔离粒度

**选:git worktree + 能力白名单**

| 选项 | 决策 | 理由 |
|---|---|---|
| git worktree | ✅ 主选 | <100ms,跨平台,够覆盖 80% 场景(Linter / 单测 / 静态分析) |
| Docker | ❌ | macOS 体验差,镜像管理复杂(mobilevc 入口暗示主战场是个人 dev 机) |
| Firecracker / gVisor | ❌ | Linux only,单机 VCOS 过度设计,留给云端版本 |
| nsjail / chroot | ⚠️ 备选 | 仅 Linux,需要时再做 |

危险动作(`rm -rf`、网络外联、长时进程)**不靠隔离层兜底**,在 `kernel/lockmgr` + `watchdog` 做能力白名单 + 超时杀。

### 4.4 MOE 专家边界

**选:二维路由(一级记忆类型 × 二级领域)**

```
一级 expert:short / long / project    决定写入策略 + TTL
二级 expert:code / dialogue / task    决定检索通道 + 嵌入模型
```

路由器**初期用规则**,不上分类模型:
- 写入路径/事件源 → 类型(用户输入 → short;`memory_request` → long;CWD 关联 → project)
- 关键词/AST 命中 → 领域

预留升级位:`MemoryEntry.cognitive_kind ∈ {episodic, semantic, procedural, working}`,有数据后再训分类器。

## 5. 决策一览

| 决策 | 选 |
|---|---|
| CPG | Tree-sitter 常驻 + SCIP 按需 + Joern 备胎 |
| Memory 存储 | SQLite + sqlite-vec,接口预留 LanceDB |
| 影子空间 | git worktree + 能力白名单,**不上容器** |
| MOE | 一级按记忆类型,二级按领域,路由用规则起步 |

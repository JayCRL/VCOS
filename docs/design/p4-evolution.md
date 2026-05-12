# P4:进化引擎(影子空间 + 进化与反向传播)

## 1. 目标

实现架构图第 4 层"进化引擎":git worktree 隔离的影子空间 + 代码变更验证 + 学习提炼 → Memory MOE 写回。

## 2. 包结构

```
internal/shadow/
  shadow.go        # Manager / Workspace / Run / ApplyDiff / capability whitelist
  shadow_test.go

internal/evolve/
  evolve.go        # Evolver: Evaluate / extractLearnings / buildSummary
  evolve_test.go
```

## 3. Shadow(影子空间)

**隔离策略**:
- git worktree(主选):`git worktree add --detach <target>`,<100ms 创建
- 文件副本(降级):非 git 项目直接 `copyDir`(跳过 `.git`)
- `Cleanup` 自动 `git worktree prune`

**能力白名单**:

| 命令 | 所需 Capability |
|---|---|
| `cat / ls / head / tail / wc` | `read` |
| `git status/log/diff` | `read` |
| `git push` | **始终拒绝** |
| `go build / cargo build / npm run build` | `build` |
| `go test / cargo test / pytest` | `test` |
| `go vet / golangci-lint / eslint` | `lint` |
| `gofmt / prettier` | `format` |
| `rm / sudo / curl / wget / chmod 777` | **始终拒绝** |
| `sh / bash / zsh` | `shell`(默认不授权) |

**API**:
```go
m := shadow.NewManager("")  // basePath defaults to os.TempDir/agentos-shadow
ws, _ := m.CreateWorkspace(ctx, projectPath)
defer ws.Cleanup()

ws.ApplyDiff([]FileChange{{Path: "main.go", NewContent: "..."}})
result, _ := ws.Run(ctx, caps, "go", "build", "./...")
// result.Stdout, result.Stderr, result.ExitCode, result.Duration
```

## 4. Evolve(进化引擎)

**流程**:
```
Proposal → CreateWorkspace → ApplyDiff → RunChecks(lint/build/test) → extractLearnings → persist to Memory
```

**Check type**: `build` / `test` / `lint` / `format`, 默认 `[lint, build, test]`

**Learning 提炼**:
- check 通过 → `{Pattern: "<type>_passed", Confidence: 0.9}`
- check 失败 → 从 stderr 提取错误模式(`file:line: message` → 取 message 部分) → `{Pattern: "<msg>", Confidence: 0.7}`

**Memory 写回**:每条 Learning 写入 `Type=LongTerm, Domain=Code` 的 Memory Entry,供 MOE 后续检索。

**API**:
```go
ev := evolve.New(shadowMgr, memStore)
result, _ := ev.Evaluate(ctx, Proposal{
    Title: "fix-null-pointer",
    CWD:   "/project",
    Changes: []shadow.FileChange{...},
    Checks: []CheckType{CheckBuild, CheckTest},
})
// result.Passed, result.Checks, result.Learnings, result.Summary
```

## 5. Kernel 集成

```go
Kernel.ShadowMgr *shadow.Manager   // workspace lifecycle
Kernel.Evolver  *evolve.Evolver    // proposal evaluation + learning writeback
```

`kernel.New` 默认创建并互联:`shadow.NewManager("")` + `evolve.New(shadowMgr, memStore)`。

## 6. 端到端流程

```
用户提出代码修改
  → Kernel.Evolver.Evaluate(Proposal{Changes, Checks})
    → Shadow.CreateWorkspace → git worktree 隔离副本
    → Workspace.ApplyDiff → 写入修改后的文件
    → Workspace.Run("go build ./...") → CheckBuild
    → Workspace.Run("go test ./...")  → CheckTest
    → extractLearnings → [{Pattern: "build_passed", Confidence: 0.9}, ...]
    → Memory Store.Upsert → 学习写入 long_term+code 记忆
  → Result{Passed: true, Learnings: [...]}
```

## 7. 验收

- `go build ./...` ✅
- `go test ./internal/shadow/...` ✅(worktree / copy / Run / block / diff / whitelist)
- `go test ./internal/evolve/...` ✅(build pass / fail / diff+eval / error pattern)
- `go test ./internal/kernel/...` ✅(P4 集成 4 条:Shadow / Evolve / Diff+Evolve / 全链)

## 8. 不做

- Tree-sitter 静态分析(预留 L2 SCIP)
- Docker/Firecracker 隔离(单机 AgentOS 不需要)
- 自动回滚(当前仅报告,不自动 revert)
- 跨语言 check runner(当前仅 Go,可通过 Proposal.Checks 自定义命令)
- 进化建议 UI 展示(留 P5 反馈闭环)

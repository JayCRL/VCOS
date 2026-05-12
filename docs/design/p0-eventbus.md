# P0:全局事件总线(internal/eventbus)

## 1. 目标

在不动现有 `EventSink`/`RuntimeSession.AppendPending` 的前提下,叠加一层**全局事件总线**,为后续 P1 调度三件套(意图调度器、看门人、锁管理器)提供统一订阅入口。

## 2. 现状回顾

- 协议:`internal/protocol`,40+ 事件类型,`Event.SessionID` + `EventCursor`(单调,per-session)
- 通道:`type EventSink func(any)`,kernel 各方法接 sink 单点回调
- 缓冲:`RuntimeSession.AppendPending` ringbuffer + cursor + 多 listener fan-out(per-session)
- 持久化:`MarkPersisted/PersistedCursor`(per-session)

**局限**
- 全部 session-scoped,**无系统级 / 跨会话 / 外部源通道**
- 无事件源标签(无法区分用户 / 会话 / 内核 / 外部)
- Sink 是点对点,无 pub/sub 多订阅者

## 3. 设计

### 3.1 包结构

```
internal/eventbus/
  envelope.go    # Envelope, Source, Topic
  bus.go         # Bus, Subscribe, Publish, Close
  filter.go      # Filter (source/topic/sessionID 维度)
  cursor.go      # 系统级单调游标
  bus_test.go
```

### 3.2 核心类型

```go
type Source string
const (
    SourceUser     Source = "user"      // 来自客户端的请求事件
    SourceSession  Source = "session"   // 来自 engine/session 的运行时事件
    SourceKernel   Source = "kernel"    // 来自调度器/看门人/锁管理器
    SourceExternal Source = "external"  // 来自 push/adb/MCP 等外部
)

type Envelope struct {
    Cursor    int64       // 系统级单调,Bus 分配
    Source    Source
    Topic     string      // 事件类型字符串(对齐 protocol.EventType*)
    SessionID string      // 可空
    Timestamp time.Time
    Payload   any         // 通常是 protocol.* 事件
}

type Filter struct {
    Sources    []Source  // 任一匹配
    Topics     []string  // 任一匹配,空表示全部
    SessionIDs []string  // 任一匹配,空表示全部
}

type Handler func(Envelope)

type Bus interface {
    Publish(env Envelope) int64                 // 返回分配的 Cursor
    Subscribe(name string, f Filter, h Handler) Subscription
    LatestCursor() int64
    Close() error
}

type Subscription interface {
    ID() string
    Close()
}
```

### 3.3 实现要点

- **单 goroutine fan-out**:Publish 入 channel(buffered 4096),后台 dispatcher 按订阅者过滤分发
- **背压**:每个订阅者独立 channel(buffered 256),满了**丢弃**并计数(P1 的 watchdog 会消费 metrics)
- **零持久化**:P0 只做内存,持久化游标接口预留:`type Persister interface { Save(cursor int64) error; Load() (int64, error) }`,默认 nil
- **不破坏现状**:不改 `EventSink`、`RuntimeSession`,而是提供一个适配函数:

```go
// WrapSink wraps an existing EventSink so every event is also published to the bus.
func WrapSink(sink kernel.EventSink, src Source, sessionID string, bus Bus) kernel.EventSink {
    return func(ev any) {
        sink(ev)
        bus.Publish(Envelope{
            Source:    src,
            Topic:     topicOf(ev),
            SessionID: sessionID,
            Timestamp: time.Now(),
            Payload:   ev,
        })
    }
}
```

### 3.4 与 Kernel 的连接

- `Kernel` 新增字段 `Bus eventbus.Bus`(可选,nil 时退化为现状)
- `kernel.New(store)` 默认创建一个 in-memory Bus
- `gateway` 与 `agentd` 在创建 sink 时调用 `eventbus.WrapSink(rawSink, SourceSession, sessionID, k.Bus)`

### 3.5 Topic 提取

`topicOf(any) string` 用类型 switch 映射到 `protocol.EventType*` 常量。fallback 用 `reflect.TypeOf().Name()`。

## 4. 验收

1. 单测:Publish 后能被多个订阅者按 Filter 收到,游标单调
2. 单测:订阅者慢 → 满载丢弃 + drop 计数
3. 单测:WrapSink 包装的旧 sink 行为不变,且 Bus 收到同份事件
4. `go build ./...` 通过
5. `go test ./internal/eventbus/...` 通过

## 5. 不做(留 P1+)

- 持久化(只留接口)
- 系统级 cursor 的快照/重放(per-session 已在 RuntimeSession 做)
- 跨进程订阅(WebSocket / gRPC)
- 死信队列
- 优先级调度(留 P1 调度器自己做)

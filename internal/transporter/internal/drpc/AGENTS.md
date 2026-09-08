# DRPC 高性能通信方案

## 1. 概述

DRPC 是 due 框架内部 transport 层基于 TCP 的高性能通信实现，用于集群节点（gate / node）之间的服务间通信。它采用自定义二进制协议 + 零拷贝缓冲 + 连接池 + 分片等待队列，实现了高性能的请求-响应（Call）与单向推送（Send）两种通信模式，并在连接中断时提供自动重连、请求重发与响应幂等去重能力，保证消息不丢失、不重复执行。

核心文件：

| 文件 | 职责 |
| --- | --- |
| `client.go` | 客户端，管理连接池与负载均衡 |
| `client_conn.go` | 客户端单条连接，状态机 / 拨号 / 读写 / 重连 |
| `server.go` | 服务端，监听 / 路由分发 / 响应缓存 |
| `server_conn.go` | 服务端单条连接，读写 / 心跳 / 优雅关闭 |
| `pending.go` | 客户端未完成调用等待队列（分片锁） |
| `def.go` | 连接状态常量与默认参数 |
| `options.go` | 客户端 / 服务端配置 |

## 2. 协议设计

协议定义在 `internal/transporter/internal/protocol` 包，采用 TLV 风格的定长帧头：

```
+--------+--------+--------+--------+--------+-----+
|  size  | header | route  |  seq   |  ...   |     |
| 4 字节 | 1 字节 | 1 字节 | 8 字节 | payload|     |
+--------+--------+--------+--------+--------+-----+
```

- `size`：大端 4 字节，表示除去 size 自身后的报文长度（`size - 4`）。
- `header`：1 字节位标识，`dataBit(0<<7)` 数据包、`heartbeatBit(1<<7)` 心跳包、`disconnectBit(1<<6)` 断连包。
- `route`：1 字节路由号，见 `internal/transporter/internal/route`，如 `Handshake / Bind / Push / Broadcast / Deliver` 等。
- `seq`：8 字节序列号，`seq == 0` 表示单向推送（Send），`seq > 0` 表示请求-响应（Call）。

### 2.1 心跳包

心跳包只有 `size + header`（size = 1），无 route / seq。客户端写协程按 `defaultHeartbeatInterval` 定时发送；服务端在收到任意消息时刷新 `lastHeartbeatTime`，写协程按 2 倍间隔检测超时并强制关闭。

### 2.2 握手协议

握手是客户端建立连接后的第一个交互，用于上报实例身份：

```
请求: size + header + route(Handshake) + seq + insKind(1) + insID(string)
响应: size + header + route(Handshake) + seq + code(2)
```

- 客户端 `handshake` 使用固定 `seq = 1`，通过 `pending` 等待响应，超时由 `DialTimeout` 控制。
- 服务端 `handshake` 解码 `insKind / insID` 保存到 `ServerConn`，并返回 `codes.OK`。此后服务端通过 `(insID, seq)` 识别该连接的请求来源，用于响应幂等去重。

## 3. 客户端设计

### 3.1 Client 与连接池

`Client` 维护 `ConnNum` 条 `ClientConn`，形成连接池：

- `Establish` 并发建立全部连接，失败采用指数退避（5ms 起，倍增，1s 封顶）重试，避免忙等。
- `load(idx ...int64)` 选择连接：
  - 无 `idx` 时通过 `atomic.Uint64` 轮询（round-robin）；
  - 有 `idx` 时按 `idx % n` 哈希路由，保证同一目标稳定落到同一连接。
- `Call`（请求-响应）与 `Send`（单向推送）均先检查 `ctx`，再 `load` 连接后委托给 `ClientConn`。

### 3.2 ClientConn 状态机

连接状态由 `atomic.Int32` 维护，三种状态定义于 `def.go`：

```
connClosed ──dial──▶ connOpened ──write/read失败──▶ connHanged
    ▲                    │                              │
    └────close/destroy───┴─────────retry(dial)──────────┘
```

- `connClosed`：未建立或已关闭。
- `connOpened`：连接可用。
- `connHanged`：连接异常，正在重连。

### 3.3 拨号与并发控制

- `dial` 用 `sync.Mutex + sync.Cond` 保证同一时刻仅一个拨号进行，其他调用方在 `cond.Wait` 上等待完成后复用结果，避免重复拨号。
- 阻塞的网络拨号放在锁外执行（`doDial`），降低锁粒度。
- `doDial` 按 `DialRetryTimes` 与指数退避重试拨号和握手，避免握手失败直接放弃。

### 3.4 会话（session）隔离

`session` 封装一次连接的生命周期（`conn / ctx / cancel`），通过 `atomic.Pointer[session]` 保存。读写协程都持有 `session` 参数：

- 重连时旧 session 的 `cancel` 被调用，读写协程通过 `ctx.Err()` 退出；
- `retry` 仅在传入的 session 仍是当前 session 时才触发重连，防止旧协程误伤新连接。

## 4. 服务端设计

### 4.1 Server

- `handlers [256]RouteHandler`：以路由号为下标的路由分发表，`NewServer` 时注册 `handshake`。
- `connections sync.Map`：以 `*net.TCPConn` 为键维护所有连接。
- `serve`：Accept 循环，瞬时错误指数退避重试，`net.ErrClosed` 时退出并关闭所有连接。
- `Start / Stop`：`started` 原子标志防重复操作；`Stop` 关闭 listener 并优雅关闭所有连接。

### 4.2 ServerConn

每条连接拥有独立的读协程（`wg1`）与写协程（`wg2`）以及写队列：

- `Send`：先记录响应缓存（幂等去重），再检测状态后写入队列。
- `Close(force)`：区分优雅关闭与强制关闭。
  - `graceClose`：`connOpened → connHanged`，写入 nil 关闭信号，等待写队列排空后关闭；
  - `forceClose`：立即切换 `connClosed` 并关闭。
- `doClose`：关闭写队列 → 等待写协程退出 → 关闭 TCP 连接 → 等待读协程退出 → 从 `connections` 移除。

## 5. 可靠性设计

### 5.1 请求-响应（Call）的断线重发

- `call` 在发送前将请求数据 `append([]byte(nil), buf.Bytes()...)` 复制一份存入 `pending`，用于重连后重发。
- 重连成功后的写协程优先调用 `resend`，遍历 `pending.snapshot()` 用 `net.Buffers` 批量重发所有未完成请求，保证请求不丢失。

### 5.2 单向推送（Send）的断线重发

- `doWrite` 写失败时：`seq == 0` 的消息保留到 `retryBuf`（单条），重连后优先重发；`seq > 0` 的 Call 由 `pending` 兜底。
- 由于 Send 是 fire-and-forget，仅保留最近一条失败消息，语义为尽力而为。

### 5.3 服务端响应幂等去重

为避免客户端重连重发导致服务端重复执行业务逻辑：

- 服务端维护 `replyCache`，以 `(insID, seq)` 为键缓存最近响应（TTL 1 分钟，惰性清理）。
- `ServerConn.read` 处理请求时，若命中缓存则直接重发缓存的响应并 `continue`，不再执行 handler；未命中则在执行前设置 `recordSeq = seq`。
- `ServerConn.Send` 在 `recordSeq != 0` 时将本次响应写入缓存，供后续重发去重。

## 6. 性能设计

- **零拷贝**：`buffer.NocopyBuffer` + `net.Buffers`（writev）聚合多个内存块一次性写出，避免拷贝；`ReaderBuffer` 直接从连接读入池化内存。
- **分片锁等待队列**：`pending` 用 64 个分片 `map[seq]*call` 分散锁竞争，按 `seq % 64` 路由。
- **连接池 + 轮询/哈希负载**：`Client.load` 无锁轮询选择连接，吞吐高。
- **非阻塞写队列**：`queue.Queue` 带超时，写满或超时可及时反馈，避免无限阻塞。
- **缓冲读**：`bufio.Reader` 4096 缓冲 + 定长帧头一次性 `io.ReadFull` 读取完整消息。

## 7. 并发与锁

- 连接状态用 `atomic.Int32`，会话用 `atomic.Pointer`，避免读状态与关闭操作间的数据竞争。
- 拨号用 `mutex + cond` 序列化，重连用 CAS/状态判断避免重复。
- 服务端 `rw.RWMutex` 保护 `conn` 指针的读写（`Send` 持读锁，`doClose` 持写锁置 nil）。
- 读写协程退出由 `WaitGroup`（`wg1 / wg2`）与 `session.ctx` 取消共同保证，避免 goroutine 泄漏。

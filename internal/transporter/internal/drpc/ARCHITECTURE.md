# DRPC 整体架构设计方案

本文基于 2026-09-08 工作区中的实际代码整理，包含尚未提交的实现。阅读范围覆盖 drpc 全部 Go 文件，以及 protocol、route、queue、buffer 和 gate/node 适配层的关联调用。目录内已有说明作为背景材料，技术结论以实现为准。

## 1. 定位与设计目标

DRPC 是 Due 集群内部面向 gate/node 通信的 TCP 传输内核。它通过固定路由号和自定义二进制帧，在长连接上提供请求响应调用（Call）与单向发送（Send），并承担连接复用、响应关联、故障重连和内存级重发。

其核心设计取舍是：用较小的协议与调度开销满足高频内部通信；通过多连接分散串行处理压力；通过保存请求与响应副本改善短时断线恢复。

它不提供持久化消息日志、事务提交或严格的 exactly-once 保证。业务取消不会自动传递到远端，服务发现和业务接口实现也由上层负责。

## 2. 分层与组件关系

```mermaid
flowchart LR
    A[调用方业务] --> B[gate/node Client\n编码请求、生成 seq、解码响应]
    B --> C[DRPC Client\n固定连接池、连接选择]
    C --> D[ClientConn\n队列、pending、session、重连]
    D <-->|TCP 二进制帧| E[ServerConn\n读写循环、心跳、响应缓存接入]
    E --> F[Server\n路由表、连接表、共享响应缓存]
    F --> G[gate/node Handler\n解码、身份判断、错误码转换]
    G --> H[Provider 业务实现]
```

| 层次 | 主要职责 | 代码位置 |
| --- | --- | --- |
| 业务适配 | 暴露 Bind、Push、Deliver、GetState 等接口；构造和解释协议 | `internal/transporter/gate`、`internal/transporter/node` |
| 客户端管理 | 为一个目标地址维护固定数量连接；轮询或按键选连接 | `client.go` |
| 客户端连接 | 拨号、握手、读写、响应等待、重发、故障恢复 | `client_conn.go`、`pending.go` |
| 服务端管理 | 监听、连接登记、路由注册、共享响应缓存 | `server.go` |
| 服务端连接 | 帧读取、同步路由调用、响应排队、心跳和关闭 | `server_conn.go` |
| 协议与基础组件 | 二进制编解码、缓冲区、有限容量队列 | `../protocol`、`core/buffer`、`core/queue` |

上层 Builder 按地址缓存 Client，并通过 singleflight 合并同地址的并发构建。DRPC 连接池只在同一目标地址的连接之间选择，不负责多个服务实例之间的负载均衡。

## 3. 通信协议

普通数据帧由 14 字节公共部分和变长业务字段组成：

```text
偏移      0         4          5         6          14
          +---------+----------+---------+----------+----------------+
          | size:4  | header:1 | route:1 | seq:8    | 业务字段 ...   |
          +---------+----------+---------+----------+----------------+
```

| 字段 | 语义 |
| --- | --- |
| size | 大端 uint32，值为整帧长度减 4；普通帧至少为 10 |
| header | 位标志；最高位表示心跳，bit 6 为业务断连标志 |
| route | uint8，定位服务端 256 槽位路由表 |
| seq | 大端 uint64；0 表示单向消息，非 0 用于响应关联 |
| 业务字段 | 各路由独立定义；响应通常以 2 字节 code 开始 |

这是长度前缀的定制二进制协议，各路由采用固定字段布局，不是通用的逐字段 TLV。`disconnectBit` 由 Push 等业务协议解释，不是 DRPC 连接关闭的通用控制帧。

心跳帧仅有 `size + header`，总长 5 字节，size 为 1。握手请求在公共部分之后携带 `epoch:8 + insKind:1 + insID`，握手响应携带 `code:2`。客户端握手固定使用 seq=1；epoch 为客户端实例创建时生成的启动代次（UnixNano），重连期间保持不变，用于服务端响应缓存按 (insID, epoch, seq) 对重连重发做幂等去重。

帧边界在读取前即被严格校验：size 必须为 1（心跳）或不小于 10（普通帧最小长度），且整帧长度不得超过 `protocol.MaxFrameSize`（16MB）。非法长度直接返回 `ErrInvalidMessage`，短包不会进入 `ParseBuffer` 引发切片越界，超大包不会触发无界内存分配。

TCP 拆包采用“先读取 4 字节长度，再读取剩余整帧”的方式，两端使用 4096 字节 bufio.Reader。服务端把包含公共头的完整帧传给 Handler；客户端也将完整响应帧返回上层解码。

## 4. 客户端架构

### 4.1 连接池与选择策略

`NewClient` 解析地址，`Establish` 并发建立 ConnNum 条连接。每轮保留成功连接，对不足数量继续补建。连续失败采用 5ms 起、翻倍、1s 封顶的退避。`Establish` 接受可选的 `context.Context`，取消或超时将中断补建循环并返回错误。

正常调用使用轮询；指定非负 idx 时使用 `idx % 连接数`，负 idx 等同于未指定。上层可传 cid，使同一个会话倾向于复用同一连接。

这不是健康感知调度：选中故障连接后等待该连接恢复，不自动切换其他可用连接。连接池应在 Establish 完成后发布使用；当前没有为并发扩容、重复 Establish 提供同步约束。Close 不清空连接切片，仅逐连接置 `closed` 标记并销毁，因此并发的 Call/Send 经 load 读取切片始终安全。

`Client.Close` 会销毁全部连接：关闭会话、关闭写队列并释放积压消息、通过 `pending.closeAll` 关闭所有等待 channel 唤醒全部调用者（返回 `ErrConnectionHanged`）、释放 retryBuf。关闭后的连接通过 `closed` 标记拒绝新的发送与拨号（`doSend`/`dial`/`doDial`/`process`/`retry` 各入口均校验），返回 `ErrClientClosed`；Close 期间到达的拨号结果会被直接废弃，不会残留僵尸会话。队列的写入（`queue.Write`，读锁）与关闭（`queue.Close`，写锁）通过 RWMutex 配对互斥，杜绝向已关闭 channel 写入导致的 panic。

### 4.2 逻辑连接与物理会话分离

一个 ClientConn 长期保留 queue、pending 和 retryBuf；每次 TCP 建连创建新的 session，持有 TCPConn、context 和 cancel。这样物理连接更换后，逻辑上的未完成请求仍可保留。

```mermaid
stateDiagram-v2
    [*] --> Closed
    Closed --> Opened: TCP 建立，发布 session
    Opened --> Hanged: 当前 session 读写失败
    Hanged --> Opened: 重拨 TCP，发布新 session
    Hanged --> Closed: 重试耗尽
    Opened --> Closed: 握手失败或关闭
```

上图反映实际状态时机：`Opened` 在握手成功完成后才被设置。握手在 `process` 阶段以同步方式完成：客户端先发送握手帧，再在同一连接上同步读取并校验响应（路由必须为 Handshake、seq 必须为 1、code 必须为 OK），校验通过后才发布 session、启动读写协程。握手不占用业务 pending 分片，seq=1 被保留为控制序列，不会与业务等待项互相覆盖。握手期间使用 `DialTimeout` 作为整次握手读写的 deadline。

mutex + cond 合并并发拨号，网络拨号放在锁外执行。retry 校验传入 session 仍是当前 session，并要求状态为 Opened，再切换到 Hanged；这减少旧连接读写循环对新会话的干扰。

### 4.3 请求等待表

每条 ClientConn 有独立 pending，固定分为 64 个带互斥锁的 map，按 `seq % 64` 定位。每项保存容量为 1 的响应 channel 和完整请求副本。

Call 的主路径是：

1. 创建响应 channel，复制请求，登记 pending。
2. 经连接状态检查后，将原始缓冲区写入 queue。
3. 等待响应、调用方 context 取消或 CallTimeout。
4. 读循环根据 seq 删除 pending 并交付响应；找不到等待者时释放响应。
5. 超时路径删除等待项，并回收已经到达但未被使用的响应。

登记发生在入队之前，避免响应先到而等待项尚未建立。序列号由上层生成，DRPC 不验证调用参数 seq 与帧内 seq 是否一致。

## 5. 服务端架构与调用时序

Server 管理 TCP listener、`sync.Map` 连接表、`[256]RouteHandler` 路由表，以及跨连接共享的 replyCache。构造时注册 Handshake，其余路由由 gate/node 在启动前注册。

每条 ServerConn 有一个读循环和一个写循环。读循环直接同步执行 Handler，未另设通用业务工作池。因此同连接上的 Handler 串行执行，不同连接之间可以并行。

```mermaid
sequenceDiagram
    participant B as 业务适配 Client
    participant C as ClientConn
    participant P as pending
    participant S as ServerConn
    participant R as replyCache
    participant H as Handler / Provider
    B->>C: Call(ctx, seq, 请求帧)
    C->>P: 保存响应 channel 与请求副本
    C->>C: 原始请求入写队列
    C->>S: TCP 请求帧
    S->>R: begin (insID, epoch, seq)
    alt 已有未过期响应
        R-->>S: 缓存响应，直接重发
    else 相同请求正在执行
        R-->>S: 等待执行完成后重发缓存响应
    else 首次执行
        S->>H: 同步处理完整请求帧
        H->>S: Reply(seq, 响应帧)
        S->>R: finish 缓存响应并唤醒等待者
    end
    S-->>C: 写队列发送响应
    C->>P: 按 seq 交付并删除等待项
    P-->>B: 返回完整响应，由上层解码
```

Handler 的返回 error 主要用于日志，并不会自动转化为错误响应。上层适配层负责使用 codes 编码业务失败；未知路由被忽略，解码失败等分支也可能没有响应，客户端最终依赖超时结束等待。

响应关联通过显式 `ServerConn.Reply(seq, buf)` 完成：`RouteHandler` 签名为 `func(conn, seq, data) error`，seq 由读循环从帧内解析后直接传入。Reply 先将响应副本写入去重缓存（finish），再将原始缓冲入写队列，因此异步 goroutine 回包与同步回包同样安全。服务端在完成握手前拒绝一切非 Handshake 业务帧并强制关闭连接；握手成功后才设置 `handshaked` 标记并记录 insID/insKind/epoch。

### 5.1 响应去重状态机

replyCache 以 `(insID, epoch, seq)` 为键，维护 `executing` / `done` 两种状态：

- **首次执行**：`begin` 原子地创建 executing 占位并返回执行权，Handler 执行后由 `Reply` 触发 `finish` 缓存响应并唤醒等待者；Handler 返回错误时 `finish(nil)` 使占位立即失效，后续重试可重新执行。
- **重复请求（已完成）**：`begin` 返回未过期的缓存响应，直接重发，不重复执行业务。
- **重复请求（执行中）**：`begin` 返回执行中条目的完成信号，等待者阻塞至 `finish` 后读取缓存响应重发，避免重连期间并发重复执行同一业务。

epoch 由客户端在每次连接（含重连）时生成（UnixNano），使同一客户端重建连接后 seq 重新计数也不会命中上一代连接的缓存；服务端进程重启则缓存整体消失。

## 6. 故障恢复与交付语义

### 6.1 恢复路径

连接读写失败后，客户端关闭并取消旧 session，再拨号、握手。新写循环的处理顺序为：

1. 尝试重发 retryBuf 中的一条写失败 Send。
2. 遍历 pending 快照，逐个发送保存的请求副本。
3. 消费原有 queue 中的消息，同时定时发送心跳。

重发遍历的是分片 map 快照，不按 seq 或原始发送顺序排序，也不是将全部 pending 合成一次批量写。尚在 queue 中的 Call 也可能同时出现在 pending，因此恢复时可能先发送副本，随后再次发送队列中的原始请求。

### 6.2 响应缓存

replyCache 以 `(insID, epoch, seq)` 为键，作用域为当前 Server 进程，TTL 为 1 分钟。Reply 在入队之前先 finish 缓存响应，使“业务已完成、连接已断开”的结果仍有机会被重连请求取回。

查询时删除命中的过期条目；finish 时至多每半个 TTL 扫描一次全表清理过期项。缓存没有容量上限，也没有后台清理线程。所有 begin/get/finish 共享一把互斥锁，网络 I/O 均在锁外完成。

### 6.3 对上层的准确承诺

| 情况 | 当前行为与边界 |
| --- | --- |
| Send 返回 nil | 表示成功入队，不代表对端已收到或业务成功 |
| Call 收到响应 | 表示收到匹配 seq 的帧；业务是否成功仍由上层解码 code |
| 短时断线 | 保留仍在 pending 的请求，重连后尝试重发 |
| 已缓存响应的重复请求 | 同一 Server 内、相同身份和 seq、TTL 内可复用响应 |
| 首次执行尚未完成时重发 | 去重状态机含 executing 占位，重复请求等待执行完成后重发缓存响应，不并发重复执行 |
| 缓存过期、服务端重启 | 去重记录消失，重试可能再次执行 |
| 客户端进程退出 | 内存请求副本和排队消息丢失 |
| Call 超时或取消 | 停止本地等待，不撤回已排队或已发送请求，也不取消远端执行 |
| Send 写入成功但对端未处理 | 没有确认机制，客户端不能检测并补偿这种丢失 |
| Send 写失败后重发 | 可能已被对端接收；seq=0 没有响应去重，可能重复执行 |

因此应描述为“内存级故障重发 + 有限窗口响应去重”，不能承诺严格的不丢失、不重复执行或持久化的至少一次交付。

相同非负 idx 可提供正常链路下的连接亲和性，但跨连接、并发调用、重连和重发不保证全局顺序；需要严格业务顺序时，应另设业务序列和校验机制。

## 7. 并发、背压与内存管理

| 机制 | 作用 | 成本或约束 |
| --- | --- | --- |
| 每连接独立读写循环 | 读响应与发送请求可并行 | 慢 Handler 会阻塞同连接后续读取 |
| 固定连接池 | 分散连接内串行处理压力 | 增加 TCP、goroutine、队列与 pending 数量 |
| 64 分片 pending | 减少不同调用之间的锁竞争 | 快照跨分片逐次加锁，不是全局原子快照 |
| NocopyBuffer + net.Buffers | 将一个消息的多个内存块交给 scatter/gather 写接口 | 具体系统调用优化依平台；不等于端到端零拷贝 |
| 有界 channel 队列 | 约束待发送消息数量，提供背压 | 满队列可能阻塞，未配置正超时时可无限等待 |
| 原子状态和 session 指针 | 支持并发读取状态与切换会话 | 不能替代完整的生命周期协调 |

客户端响应读取使用池化 Bytes；服务端 ReadMessage 使用 make 为每帧分配切片。Call 请求副本和服务端缓存响应均显式复制，多块 NocopyBuffer.Bytes 还会先拼接。因此性能方案应称“减少常规发送路径的拼接拷贝”，不应称“全链路零拷贝”。

缓冲区所有权在提交 Send/Call 后交给 DRPC：失败路径或完成写入后负责 Release。成功 Call 返回的响应由上层负责 Release。内存预算除队列外，还必须计入未完成请求副本和服务端响应缓存；后二者没有独立的数量或字节上限。

## 8. 心跳、超时与生命周期

客户端每 10 秒尝试写心跳。服务端每 10 秒检查一次上次收到完整消息的时间，超过 20 秒触发关闭；任何完整消息都会刷新时间。服务端不回复心跳，客户端也没有等待心跳响应的超时机制。

心跳收发依附于现有读写循环：慢 Handler 会延迟读取心跳，阻塞的 socket 写会延迟心跳发送或超时检查，所以不能视为独立故障探测器。

| 配置 | 实际控制范围 |
| --- | --- |
| ConnNum | 初始固定连接数量；调用方需保证正值 |
| DialTimeout | 单次 TCP 拨号，以及握手写入之后的响应等待；不是整个恢复期限 |
| DialRetryTimes | 非负值表示初次尝试之外的重试次数，负值持续重试 |
| CallTimeout | send 返回后开始的本地响应等待时间；不包含前面的拨号、重连等待和入队耗时 |
| WriteTimeout | queue 写入拥塞时的等待时间；同时作为 TCP SetWriteDeadline 应用于客户端数据帧/心跳写入与服务端响应写入 |
| WriteQueueSize | 每条连接的队列容量，下限 128 |
| FaultRecoveryTime | release/pre-release 模式下，Closed 状态发送时的故障冷却窗口 |
| Addr / Expose | 服务端监听地址及暴露地址生成 |

Establish 的外层补建支持通过可选 context 取消；取消或超时将中断补建循环并返回错误。Call 的等待路径已接入调用方 context，取消时删除等待项并回收已到达响应。cond 等待和队列写入仍未接入调用方 context，不能把 CallTimeout 理解为端到端截止时间。

服务端优雅关闭先将状态改为 Hanged，拒绝新 Send，向写队列插入 nil 标记；写循环处理到标记后退出，再关闭 TCP 并等待读循环结束。它尽力发送已排队响应，不是完整的在途业务排空协议。

强制关闭同样会先关闭 queue、等待写循环，再关闭 TCP。若 socket 写长期阻塞，关闭本身也可能等待，不能承诺立即完成。客户端提供统一的 `Client.Close`：销毁全部连接、关闭队列、释放积压消息与 retryBuf，并通过 `pending.closeAll` 关闭所有等待 channel，使全部在途 Call 立即以 `ErrConnectionHanged` 返回。

## 9. 关键设计约束与完善方向

以下 1~7 项原始风险点已落地修复；第 8 项起为仍需关注的后续方向。

1. ~~**统一请求身份**~~（已修复）。握手改为同步交互，不占用业务 pending；seq=1 保留为控制序列。握手请求携带实例级 epoch 启动代次（重连期间保持不变），去重缓存键为 `(insID, epoch, seq)`，客户端进程重建后 epoch 变化、seq 重新计数，不会命中旧代次缓存。
2. ~~**明确去重等级**~~（已修复）。replyCache 升级为 executing/done 状态机：重复请求在执行中等待完成信号，已完成直接重发缓存响应，重连期间不并发重复执行同一业务。涉及不可重复的业务副作用且需跨进程故障保证时，仍需业务幂等键或事务支持。
3. ~~**完善握手状态**~~（已修复）。客户端同步握手并校验响应路由、seq 与 code 后才发布 session；服务端未完成握手前拒绝一切业务帧并强制关闭连接。当前握手仍是身份声明，不是认证。
4. ~~**提供完整截止时间**~~（已修复）。Call 等待接入调用方 context；Establish 支持 context 取消；客户端与服务端 TCP 写均按 WriteTimeout 设置 SetWriteDeadline。cond 等待与队列写入仍未接入 context。
5. ~~**限制协议和内存资源**~~（已修复）。读取前校验帧长：心跳为 1、普通帧最小 10、整帧上限 16MB，短包不进入 ParseBuffer，超大包不触发无界分配。pending、缓存和队列的字节级预算仍未设置。
6. ~~**固定响应关联**~~（已修复）。RouteHandler 签名改为 `func(conn, seq, data) error`，废弃共享 recordSeq；服务端通过显式 `Reply(seq, buf)` 回包，异步 goroutine 回包与同步回包同样安全。
7. ~~**收敛恢复与关闭路径**~~（已修复）。新增 `Client.Close` 与 `pending.closeAll`，关闭时统一回收队列、retryBuf、pending 并唤醒全部在途调用。
8. **字节级内存预算。** 为 pending 请求副本、replyCache 响应缓存和写队列补充字节级上限，避免大量大包占用内存。
9. **认证与加密。** 握手目前只做身份声明，若部署边界不可信，需要补充认证与传输加密。

## 10. 验证范围与验收建议

本文属于源码分析，没有修改 Go 实现，也没有以运行结果证明恢复或吞吐保证。当前 drpc/server_test.go 的有效测试为启动服务器后永久阻塞的演示，不适合作为自动化通过依据。

建议为架构落地建立以下验收场景：

| 场景 | 验收重点 |
| --- | --- |
| 多连接并发 Call | seq 正确匹配、响应释放、不同连接并发处理 |
| 执行前、执行中、响应写入时断线 | 重发次数、业务执行次数、缓存命中行为 |
| 首个业务 seq=1 时重连 | 握手不会覆盖业务等待项 |
| 取消、队列满、重连耗尽 | 调用在约定期限结束，pending 与缓冲区回收 |
| 超短帧、超大帧、错误握手 | 安全拒绝，不 panic，不出现无界分配 |
| 慢 Handler、对端不读、并发 Stop | 心跳与关闭期限可预测，无 goroutine 长期滞留 |
| 缓存过期、客户端重建、服务端重启 | 去重窗口与身份复用行为符合约定 |

性能验收应同时观察吞吐、P95/P99 延迟、分配量、队列等待、pending 数量和缓存字节数；不能仅凭 scatter/gather 写接口推导实际性能收益。

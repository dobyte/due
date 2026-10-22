# Gate / Node 自动本地通信

## 实现与部署

Bridge 在 `internal/link` 已确定目标之后选择传输，不改变路由分组、负载均衡、UID 归属、内部路由权限和业务处理模型。同进程且启用桥接的目标自动使用有界内存队列；其他目标使用原有 DRPC。Node→Node 及通过 GateLinker 的调用同样适用。Mesh 微服务的 `transport/grpc`、`transport/rpcx` 不在替换范围内。

| 部署 | Registry / Locator | Bridge | 内部 DRPC 监听 |
| --- | --- | --- | --- |
| 纯单体 | 共享的内存实例 | 开启 | 可关闭 |
| 分布式 | 共享注册中心、定位服务 | 默认开启 | 开启 |
| 混合 | 所有进程使用同一个注册与定位空间 | 同进程自动命中 | 开启 |
| 性能对照 / 回退 | 保持原配置 | 显式关闭 | 开启 |

同一个 Docker 容器中的不同进程仍使用 DRPC。内存组件不会自动替换用户注入的基础设施。不同进程各自创建内存 Registry / Locator，不能组成一个分布式集群。

## 使用方式

Gate、Node 默认启用本地桥接并保留 DRPC 监听，已有应用无需修改业务代码。两个组件都提供：

- `WithLocalBridge(bool)`：控制自身本地端点登记及自身发起的本地调用，默认 `true`。
- `WithLinkerListen(bool)`：控制内部 DRPC 监听，默认 `true`，不影响 Gate 的公网网络服务器。

关闭桥接时必须保留监听；同时关闭两者会在内部服务构建时返回配置错误。混合部署不能因为本地存在 Node 就关闭监听，远端仍可能需要调用。

对应配置：

```toml
[cluster.gate.linker]
disableLocal = false
disableRemote = false

[cluster.node.linker]
disableLocal = false
disableRemote = false
```

纯单体将两处 `disableRemote` 设为 `true`。显式 Go Option 优先于配置默认值。Bridge 的四个固定队列分片，各自容量使用接收组件的 `linker.commandQueueSize`（默认 4096）；不是无限 goroutine 或无限队列。Node Deliver/Trigger 按 CID 分片，Gate Push 按 target 分片，其他操作进入固定分片。独立 Router、Trigger、Actor 队列之间仍不提供全局顺序保证。

### 可运行单体示例

源码：[network/tcp/examples/bridge/main.go](../../network/tcp/examples/bridge/main.go)。使用 Due Container 组合 Node 和 Gate，二者共享一组内存 Registry / Locator。先添加 Node，再添加 Gate。Echo 路由为 1，正文为 JSON `{"text":"hello"}`，客户端仍需发送 Due packet，而非裸 JSON TCP 数据。

在 `network/tcp` 模块目录运行：

```powershell
go run ./examples/bridge -addr 127.0.0.1:3553
# 相同业务代码和内存控制平面，改用 loopback DRPC：
go run ./examples/bridge -addr 127.0.0.1:3553 -drpc
```

核心配置可直接复用：

```go
// Import aliases:
// registrymemory "github.com/dobyte/due/v2/registry/memory"
// locatememory "github.com/dobyte/due/v2/locate/memory"
r := registrymemory.NewRegistry()
l := locatememory.NewLocator()

// 将同一 r、l 注入 Gate 与所有 Node。
// gate.WithRegistry(r), gate.WithLocator(l), gate.WithLinkerListen(false)
// node.WithRegistry(r), node.WithLocator(l), node.WithLinkerListen(false)
```

### 分布式和混合部署

继续通过 `WithRegistry`、`WithLocator` 注入原先使用的插件，保持同一集群的 namespace、数据库和定位前缀一致，保留 `WithLinkerListen(true)`。Gate 和 Node 可以分别加入不同进程的 Container；也可以在一个进程中合并 Gate+Node，同时在其他进程启动 Node。无需新增业务分支或本地优先路由策略。

例如使用现有插件构建共享控制平面客户端：

```go
// registryetcd "github.com/dobyte/due/registry/etcd/v2"
// locateredis "github.com/dobyte/due/locate/redis/v2"
r := registryetcd.NewRegistry(registryetcd.WithAddrs("127.0.0.1:2379"))
l := locateredis.NewLocator(locateredis.WithAddrs("127.0.0.1:6379"))
// 将 r、l 注入本进程的 Gate / Node；其他进程使用同一外部服务及命名空间。
```

以上外部服务地址是本地开发示例，不是生产配置。将现有部署拆开时，必须先把内存组件换成共享实现，再拆分进程。

## 实例发现与生命周期

每次内部服务启动生成一个不透明的 incarnation token，写入注册实例的 `Metadata["due.bridge"]`。用户 Metadata 会被复制，不会原地增加字段。Dispatcher 将 token 附带到内部端点描述，Resolver 与进程级目录精确匹配。不同注册空间中相同 ID 不会仅凭 ID 被误认为同一对象。该 token 不是认证凭据，不替代注册中心的访问控制。

未携带 token 的旧实例继续使用 DRPC；跨进程无法命中本地目录，也继续使用 DRPC。纯本地服务发布 `local://gate` 或 `local://node`，不创建内部 TCP 服务。遇到无法在当前进程解析的 local 端点，明确返回错误，不尝试 TCP 拨号。

Resolver 缓存本地适配器，缓存最多保留 256 个端点引用，防止不断替换实例导致无界增长。缓存引用仍检查 incarnation 是否已关闭；已关闭的本地 token 不回退 TCP。实例重建使用新 token，旧客户端无法调用新 Provider。

销毁时，端点从目录移除，停止接纳，取消执行器 Context，释放尚未处理的命令，并等待正在运行的消费者退出；随后组件才清理 Provider 依赖的资源。Provider 必须能响应取消或具有有界执行时间。这个过程不是业务事务的无损排空保证；没有 ACK 的已接纳消息可能在停服时被丢弃，与既有异步通信的能力边界一致。

Gate、Node、Mesh 销毁时还会关闭其 Linker 缓存的远程连接。Gate 的首次拨号使用配置的 dial timeout，与 Node 的行为保持一致。组件并发 Close/Destroy 的既有 Container 机制没有改成依赖图，也没有实现跨组件启动屏障；对外接流量仍应等待路由发现就绪。

回退回归还修复了既有 DRPC 对空私有请求体的错误过滤：合法的 `GetState` 请求没有私有请求体，现在交给路由处理器校验和应答，不再被接收循环静默丢弃。线上帧格式保持不变。

## 消息与资源契约

- 单向 Deliver / Trigger、`Ack=false` 推送：成功表示本地队列接纳，不表示 Provider 或业务 handler 已完成。
- `Ack=true` 和查询、绑定等操作：等待 Provider 返回，错误通过现有 DRPC 错误码映射归一；Gate 推送 ACK 不表示玩家已经收到。
- 无 ACK 的广播、多播、频道发布返回数量仍为 0；不引入本地专有的成功数量语义。
- 写队列等待使用 `commandWriteTimeout`，超时返回 `ErrWriteTimeout`；有响应的调用受 `callTimeout` 及调用方 Context 约束。已接纳的调用不会因调用方停止等待而撤销或重发。
- 每个异步 Buffer 消费者获得独立字节与视图。现阶段选择边界复制，避免解包移动视图或广播扇出产生竞争；没有宣称零拷贝。
- 可变的 targets 切片在提交前复制。Buffer 在失败、关闭、超时后继续执行等路径都由明确的所有者释放。
- 本地 Node 客户端绑定来源类型与 ID，保留 Gate / Node 来源限制。业务 Context 仍由原有 Node 路径建立，不自动继承调用方 Context。
- 本地投递错误不会自动补发 DRPC，避免重复执行。原有 Linker 定位逻辑继续按原规则处理远端调用结果。

## 内存组件契约

`registry/memory` 支持注册更新、注销、查询及完整快照 Watch。初始快照与订阅建立使用同一把锁；慢订阅者可以合并中间快照，但能看到最新状态。所有返回的实例、Map、Slice 都是独立副本。Bridge 代次不匹配的旧注销请求不会移除新实例。

`locate/memory` 支持用户 Gate 绑定、分服务组 Node 绑定、条件解绑和有序事件 Watch。错误实例 ID 的解绑是无操作，不会清除新绑定。每个 Watcher 默认最多积压 4096 个事件，可通过 `NewLocator(capacity)` 指定。如果任一匹配 Watcher 已满，本次变更在修改状态前返回 `ErrWriteTimeout`，不会静默丢事件或进行部分提交。调用方应处理该错误，不应无条件忽略。

两种组件的 Watch Context 只控制订阅建立，与现有插件一致。建立后取消该 Context 不会结束订阅；使用 `Watcher.Stop()` 或关闭存储。Linker 已通过自身生命周期 Context 停止 Watcher，避免 `Next()` 阻塞导致退出悬挂。共享存储必须在所有使用它的组件销毁后，由应用统一 Close。

## 验证与性能

功能覆盖：本地/远程混合路由、远端有状态绑定、本地过期引用、ACK 及会话操作、混合广播、队列背压、等待超时、关闭并发、Buffer 独立性、内存注册快照及条件解绑、订阅初始化兼容。

根模块完整测试通过；Bridge 使用 goleak 检查新增执行器泄漏，并对通信、组件和内存实现执行 race 检测。TCP 独立模块的 `TestBridgeRoundTrip` 用真实公网 TCP 客户端验证纯本地、自动桥接和强制 DRPC 三种模式；该模块已有部分测试是长期运行的手工服务器，因此集成回归采用指定测试名，不运行这些手工入口。

性能命令（在 `network/tcp`）：

```powershell
go test -run '^$' -bench '^BenchmarkBridgeRoundTrip$' -benchtime=1s -count=3
```

基准包含客户端 TCP、Gate、真实 Node Router 和回包链路，单连接、串行、256 字节负载；双方均使用相同的内存控制平面。原始输出见 [bridge-benchmark.txt](bridge-benchmark.txt)。

2026-10-22，Windows amd64、Go 1.27.1、i5-13400F 的一轮三次测量：

| 路径 | 平均每次往返范围 | 分配次数 | 分配字节 |
| --- | --- | --- | --- |
| 本地 Bridge | 45.4–46.5 μs | 33/op | 约 2.17 KB/op |
| loopback DRPC | 85.7–101.1 μs | 32/op | 约 1.78 KB/op |

这组结果显示本地路径减少了平均往返时间，但边界复制仍增加内存分配字节。宿主机计时出现 P50=0 的量化现象，因此原始 P50/P95/P99 不作为可靠尾延迟结论。此结果不能外推生产吞吐，也没有验证高并发、多包长下的容量、CPU 曲线或跨机器性能；这些应在部署目标环境进行负载测试。

根模块 `go vet ./...` 仍报告既有 `core/buffer/reader.go` 的 `ReadRune` 签名告警，已核对 HEAD 中同样存在。本次修改涉及的包定向 vet 通过。网络 KCP/WS/QUIC、gRPC/RPCX、四种 Registry 和 Redis Locator 的独立模块构建及 vet 通过；未启动外部服务执行这些插件的服务集成测试。

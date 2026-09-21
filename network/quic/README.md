# QUIC transport

QUIC 使用一条客户端发起的双向流，业务消息和心跳格式与框架 `packet` 包一致。
客户端开流后会发送一个普通心跳包，使服务端无需等待首条业务消息就能触发
`OnConnect`。即使定时心跳被禁用，这个初始化心跳仍会发送。

## Buffer 与回调

- `Push(buffer.Buffer)` 返回 nil 后，网络库负责释放发送 Buffer；返回错误时由调用方释放。
- `OnReceive` 收到的 Buffer 由业务层释放；未注册接收回调时网络库自动释放。
- 同一连接的 `OnConnect`、`OnHeartbeat`、`OnReceive` 按读取顺序执行。
  `OnConnect` 异步触发，可能在 `Dial` 返回之前或之后执行。
- `OnDisconnect` 在连接关闭且读写协程退出后触发一次。
- 请在 `Start` / `Dial` 之前注册回调。业务回调应及时返回。

## 关闭

`Close()` 同步执行优雅关闭：拒绝新消息，排空已接收的发送队列，再关闭流的发送方向。
QUIC 连接保留到关闭期限，为数据交付和重传留出时间；不会在 `Stream.Close` 后立即
关闭 QUIC 连接。此过程是有时间上限的尽力交付，不代表对端业务已经处理消息。

`WithClientCloseTimeout` 和 `WithServerCloseTimeout` 设置关闭的总时间上限，默认 5 秒，
包括队列排空及重传时间。到期后强制关闭。`Close(true)` 立即中断底层读写。

`Close` 会等待读写协程退出并触发 `OnDisconnect` 后才返回，因此不要在连接、接收或
心跳回调中同步调用 `Close`（否则会死锁）；确需在回调中关闭时，请通过 goroutine 或
`task` 异步触发。同理，`Stop` 也不要在回调中同步调用。

`Stop()` 关闭监听器、待建流连接和活动传输，等待接入任务结束后返回。服务器支持停止后
重新启动。

## TLS 与超时

- 客户端始终协商 `due-quic` ALPN，自定义 TLS 配置会先克隆，不修改调用方对象。
- 默认使用系统根证书；`WithClientCredentials` 可以指定私有 CA 和证书域名。
  不再因为没有 CA 文件而自动跳过证书验证。无效 CA 会作为拨号错误返回。
- `WithClientDialTimeout(0)` 取消整体拨号期限，QUIC 自身的握手超时仍然有效。
- `writeTimeout` 同时约束满队列等待及实际流写入；0 表示不设写超时，强制关闭仍能中断。
- 服务端 `handshakeTimeout` 也用于等待首条流，待建流连接计入 `maxConnNum`。
- 心跳间隔为 0 时关闭应用层定时心跳，保留传输层 keepalive 和 quic-go 默认空闲超时。

## 性能与验证

写入队列使用 `core/queue` 有界队列，满队列时可按 `writeTimeout` 超时返回，避免无界阻塞。
写入直接访问 Buffer 分段，每连接复用访问回调，不合并复制完整业务包，也不依赖 TCP
专有的 writev。连接管理按递增 ID 分片，服务端连接对象通过 `sync.Pool` 复用。

在此目录执行：

```sh
go test ./...
go test -race ./...
go test -run '^$' -bench . -benchmem
```

`BenchmarkWriteBuffer` 测量可复用写辅助层，不含 QUIC 加密和网络开销；
`BenchmarkQUICEcho` 测量本机 1 KiB 消息的顺序请求响应，不能视为并发吞吐或公网延迟。

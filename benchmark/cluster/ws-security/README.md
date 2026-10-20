# WebSocket 会话加密压测

参考 [ws](../ws)，测试完整的 Client → WS Gate → Node → Gate → Client 回显链路。
Client 和 Gate 使用 `WithSessionSecurity`：ECDSA 验证网关身份，X25519/HKDF
协商会话密钥，AEAD 加密双向业务消息并认证 route、seq。Node 解析并回显 JSON。

## 启动

需要 Go（版本见 [go.mod](../../go.mod)）、Redis 和 Nacos。默认地址为
`127.0.0.1:6379`、`http://127.0.0.1:8848/nacos`，可在 Gate/Node 的
`etc/etc.toml` 修改；二者的 Nacos 分组、命名空间和 Redis 前缀须一致。
示例使用独立 Nacos 分组，避免原 WS 示例的路由干扰。

分别打开三个终端，从本目录进入对应目录执行（必须保留该工作目录以加载配置和密钥）：

```powershell
cd node
go run .
```

```powershell
cd gate
go run . -suite aesgcm
```

等待 Gate/Node 注册、路由同步完成后启动客户端：

```powershell
cd client
go run . -c 100 -n 1000000 -size 1024 -window 16 -timeout 2m
```

客户端参数：

| 参数 | 默认值 | 含义 |
| --- | --- | --- |
| `-url` | `ws://127.0.0.1:3554/` | 网关地址 |
| `-c` | 100 | 长连接数量，要求 `0 < c <= n` |
| `-n` | 1000000 | 总请求数，上限 2147483647 |
| `-size` | 1024 | ASCII message 字段字节数，范围 0–65497 |
| `-window` | 16 | 每条连接最多在途请求数，要求 `0 < window <= n` |
| `-timeout` | 2m | 全部连接就绪后的业务阶段超时 |
| `-suite` | `aesgcm` | `aesgcm` / `chacha20poly1305`，必须与 Gate 相同 |
| `-public-key` | `./etc/key.pub.pem` | 验证网关的 ECDSA 公钥文件 |

Gate 同样支持 `-suite`、`-public-key`，并支持 `-private-key ./etc/key.pem`。
随附密钥复制自 `crypto/ecc/pem`，仅用于本地压测；真实部署请替换密钥，客户端只需公钥。

测试 ChaCha20-Poly1305 时重启 Gate 和客户端，并同时传入
`-suite chacha20poly1305`。Gate 的 pprof 默认监听 `:8092`，可通过
`-pprof 127.0.0.1:0` 使用随机本地端口，或修改配置中的监听地址。

## 结果与口径

- 连接逐个建立，`dial/handshake` 统计 WebSocket 建连及安全握手的客户端就绪耗时，
  不计入业务 TPS（Dial 在 Finished 帧入队后返回，协议没有额外的服务端确认回包）。
- 请求均匀分给各连接，按回包补充发送窗口；总在途上限为 `c × window`。
  这是有界窗口压测，与原 WS 示例一次性持续入队的发压方式不同。
- `valid_replies` 只统计序列号有效、JSON 可解析且内容一致的回包。
  RTT 从调用 Push 前计时，到回调开始时截止，包含客户端编码、加密、排队、集群往返与解密。
- 输出成功 TPS、RTT min/avg/P50/P95/P99/max、未尝试发送数、Push 错误数及未确认数。
  `message_MiB/s` 仅按成功回包的双向 message 字段大小计算，不含 JSON、加密、
  Due 帧、WebSocket、TCP/IP 开销，不代表网卡吞吐量。
- 超时、Push 失败、回包异常会取消所有发送任务，等待退出并关闭已建立连接，返回非零退出码。
  `-timeout` 不包含建连与关闭阶段；WS 单次拨号/写入上限 10s，关闭等待配置为 3s，
  正在执行的 Push 可能在业务超时后才返回。Ctrl+C 也会触发清理。
- 三个进程通过共享代码固定为 4 字节 route、4 字节 seq、64 KiB 消息上限；
  `size` 的上限已预留 JSON 的 14 字节和 AEAD 信封的 25 字节。
- 客户端保存每个成功请求的 RTT，内存随 `n` 增长。测量时留意客户端 CPU、内存与网络上限；
  比较算法时保持 `c/n/size/window`、硬件和服务配置一致。

快速检查可使用 `-c 2 -n 100 -size 64 -window 4`。服务尚未就绪或回包丢失时，
客户端会在超时后给出未确认数，不能将部分结果视为完整成功。

## 本地验证

从 `benchmark` 模块目录执行：

```powershell
go build ./cluster/ws-security/...
go test -race ./cluster/ws-security/...
go vet ./cluster/ws-security/...
```

自动测试使用本地加密 WS 回显端验证握手和压测客户端，不依赖 Redis/Nacos；
完整 Gate→Node 链路需按上面的步骤启动外部服务后验证。

# Security 消息路径性能优化

## 测量范围

日期：2026-10-19。基线提交：`805297b4`。环境：Windows/amd64，Intel Core i5-13400F，Go 1.27.1，默认 GOMAXPROCS=16。

使用相同基准代码先测原实现，再测优化实现。Session 第一轮使用 `-benchtime=150ms -count=3`；连续帧第二轮使用 `-benchtime=500ms -count=3`。以下记录分配量，耗时仅用于本机对比，不能当成线上吞吐保证。

`BenchmarkSession` 覆盖 AES-GCM、ChaCha20-Poly1305，以及 0、64、256、1024、4096 字节。基线对比选用 64 和 1024 字节；Seal 测发送，RoundTrip 同时测加密和解密，每条消息使用新 counter，避免测到重放拒绝路径。ParallelSeal / ParallelMessageSeal 使用同一个 Session 并发发送，各 worker 独占目标缓冲区。

`BenchmarkSecurityPush` 覆盖 Extract、Seal、Pack 和 Release，并计入创建共享明文包的成本。1、8、64、1000 接收者场景顺序执行一次扇出，共享同一明文、每个接收者使用独立 Session；不计任务池调度、真实 socket 写入和集群转发。并发广播正确性由独立回归测试与 race 检测覆盖。

`BenchmarkSecurityReceive` 测客户端加密、封包及 Gate 解密，覆盖空明文及 64、1024、4096 字节；Gate 保持未启动状态，解密后立即释放明文，不计路由查找和 Node 转发。该项用于后续回归，目前没有同代码的优化前对照记录。

## 分配对比

| 场景 | 优化前 B/op | 优化后 B/op | 优化前 allocs/op | 优化后 allocs/op |
| --- | ---: | ---: | ---: | ---: |
| Session AES-GCM 64B Seal → SealMessage，复用目标 | 128 | 0 | 3 | 0 |
| Session AES-GCM 64B RoundTrip → MessageRoundTrip，复用目标 | 208 | 0 | 5 | 0 |
| Session AES-GCM 1KB Seal → SealMessage，复用目标 | 1184 | 0 | 3 | 0 |
| Session AES-GCM 1KB RoundTrip → MessageRoundTrip，复用目标 | 2224 | 0 | 5 | 0 |
| Gate 1KB 单接收者 | 约 2787 | 约 224 | 16 | 6 |
| Gate 1KB 8 接收者，一次扇出 | 约 20729 | 约 224 | 86 | 6 |

上述 Session 零分配结果依赖目标容量足够以及工作区池复用；冷启动、池被 GC 清理、扩容仍可能分配。Gate 结果包含输入明文包的对象、节点和包头成本，因此整条路径仍有分配；接收者自己的密文帧在池复用后不再增加稳态分配率，但在途帧及池的工作集仍随并发接收者增加。ChaCha20-Poly1305 的复用目标基准也测得 0 B/op、0 allocs/op。

连续帧补测中，64B 单接收者 Push 为 429–462 ns/op，8 接收者为 1.95–2.11 µs/op；1KB 单接收者为 729–795 ns/op，8 接收者为 3.67–3.99 µs/op。Receive 的 1KB 为 971–1054 ns/op，4KB 为 2.01–2.12 µs/op。预留区采用 16 字节对齐；直接把 9 字节默认包头作为偏移会让 AES-GCM 输出地址不对齐，并在本机造成大包吞吐回退，因此没有采用该布局。

扩展扇出补测中，64B 的 64/1000 接收者分别为 12.85–13.18 µs 和 216–237 µs，1KB 分别为 22.89–29.68 µs 和 568–604 µs，均保持 6 allocs/op。密码计算仍随 Session 数线性增长；该微基准不含任务提交，因此是否增加广播 worker 分片需要结合真实 Gate 写队列和调度 profile 决定。

## 实现与约束

- 默认 Packer 挂接 payload，不进行正文复制。优化针对提取时整个分段 buffer 的合并，连续 payload 直接取片段，跨段才复制。
- 加密路径预留对齐的包头空间，AEAD 直接写入同一个池化 `Bytes`，Packer 成功时原位前插真实包头；不支持 headroom 的自定义 Packer 继续使用原有复制或分段回退路径。
- WebSocket 单节点消息继续使用 `WriteMessage` 快速路径；分段消息通过 `NextWriter` 和 `VisitBytes` 顺序写入一个 WebSocket message，不再调用 `Bytes()` 展平整包。
- 新消息接口将 nonce/AAD 放入每次调用独占的池化工作区，并允许复用输出；原有 Seal/Open 及线协议保持兼容。
- 新打包接口持有池化输出，网络写队列消费完成后释放。推送失败仍保留调用方原始明文，避免破坏广播与重试。
- Gate 原子发布已建立的 Session，连接包装直接引用对应状态；握手转换与状态移除继续同步，断开后清除 Session，迟到握手不能重新创建状态。
- 重放窗口锁、发送 counter 原子操作、认证后推进窗口的顺序保持不变。本轮未加入握手准入、日志限流或算法偏好调整。

## 复现

在根模块执行：

```sh
go test -run '^$' -bench 'BenchmarkSession/[^/]+/(64|1024)/|BenchmarkSecurityPush' -benchmem -benchtime=150ms -count=3 ./core/security/handshake ./cluster/gate
go test -run '^$' -bench BenchmarkSecurityReceive -benchmem -benchtime=150ms -count=3 ./cluster/gate
go test -run '^$' -bench 'BenchmarkSecurity(Push|Receive)' -benchmem -benchtime=500ms -count=3 ./cluster/gate
go test ./...
go test -race ./core/buffer ./core/security/... ./packet ./cluster/gate ./cluster/client ./session
go build ./...
go vet ./...
```

完整 `go vet ./...` 当前受既有 `core/buffer/reader.go` 的 `ReadRune(order)` 签名告警阻塞；`go vet -stdmethods=false ./...` 通过，此结果不等同于完整 vet 通过。

四个网络子模块均独立构建与 vet。WebSocket 的真实连接测试覆盖单节点和分段消息；QUIC 执行测试。TCP、KCP 的测试包含长期运行的手动服务端/客户端，本次仅通过 `go test -run '^$' ./...` 编译，不记为功能测试通过。

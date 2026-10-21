# TCP 集群压测

分别在 `gate`、`node` 目录启动服务，再在 `client` 目录运行客户端。服务端依赖 Redis 和 Nacos，地址见各自的 `etc/etc.toml`。安全模式需要 Gate 和 Client 同时启用 `-security=true`。

```sh
go run . -bench=throughput -conns=50,100,200,500,1000 -window=32 -duration=30s -warmup=3s -repeat=3 -out=./results
```

- `throughput`：默认每连接 32 个在途请求，测试持续处理能力。建议扫描窗口 8、16、32、64，结合每秒 TPS、inflight 和资源使用率寻找稳定区间。
- `latency`：默认每连接 1 个在途请求，完成 `-requests` 指定的请求数（向上取整为连接数的倍数）。延迟包含客户端编码、排队及完整往返处理时间。
- `rate`：`-rate` 为所有连接合计的目标 QPS，默认窗口为 32。目标低于连接数时部分连接不发请求；背压可能使实际速率低于目标，不补发历史积压。属于有限窗口、按批次节流的负载模型。
- `-window=0` 使用模式默认值。只有显式 `-bench=throughput -unbounded=true` 才取消窗口限制；不能同时指定正数窗口。无限窗口用于过载实验，可能积累大量请求和内存。

`Window TPS` 使用测量期间收到的有效响应数除以实际测量时长，排空期间的回包不计入该值。`Drain Duration` 单独报告停止发送和排空耗时；`Sent / Recv`、成功率和延迟统计包含排空结果。发送失败计入成功率分母。失败轮次显示 `FAILED`，不进入有效结果汇总；预热失败会记录错误。

带宽为测量窗口内成功响应折算的双向业务载荷吞吐，单位实际为 MiB/s，不是网卡线速。GC/Alloc 仅反映客户端进程，包含排空阶段。CSV 仅记录测量阶段，短于一秒的尾段不输出完整秒记录。

延迟使用每连接蓄水池采样，按该连接的实际观测数加权合并。百分位和平均值是估计值，Sample Min/Max 是样本极值。闭环延迟不能代替固定外部到达率下的排队延迟评估。

客户端 TCP 写入和优雅关闭分别设置 1 秒超时，失败时并行清理连接并等待发送协程退出。预热屏障及排空等待各为 10 秒。若服务端停止消费，应检查客户端失败日志及 Gate/Node 的 goroutine 栈，而非只延长排空期限。

在 `benchmark` 模块目录验证：

```sh
go test ./cluster/tcp/...
go test -race ./cluster/tcp/client
go vet ./cluster/tcp/...
```

测试包含真实 TCP 回环回显，可独立于 Redis/Nacos 验证客户端三种负载模式；它不替代完整 Gate → Node 链路的容量测试。

## security 参数

客户端和 Gate 均支持 `-security=true|false`，默认 `false`。双方必须使用相同设置。Node 无需传入此参数，加解密由 Client 和 Gate 完成。

分别在对应目录的独立终端中启动：

```sh
# gate 目录：开启会话加密
go run . -security=true

# node 目录：正常启动
go run .

# client 目录：开启会话加密并执行压测
go run . -security=true -addr=127.0.0.1:3553 -bench=throughput -conns=50,100,200,500,1000 -window=32 -duration=30s -warmup=3s -repeat=3 -out=./results-security
```

Gate 从 `etc/etc.toml` 的 `[crypto.ecc.signer]` 读取签名私钥，客户端读取对应公钥。报告中的 `Security: true` 表示客户端已开启此模式。比较明文与加密性能时，双方均改为 `-security=false`，保持其他负载参数一致。

仓库中的密钥仅用于示例和本地压测，生产环境需替换为独立密钥。

明文与加密回归测试使用压测程序相同的客户端创建入口，覆盖三种负载模式及真实 ECC 签名、X25519 握手和双向消息加解密。在 `benchmark` 模块目录运行：

```sh
go test -race ./cluster/tcp/client -run TestSecurityFlag -count=1
```

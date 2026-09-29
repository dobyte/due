### 代码规范
1. 所有所有代码严格遵循uber-go/guide编码规范，详细规范请参考：https://github.com/uber-go/guide/blob/master/style.md
2. 每次提交到git仓库时，都使用英文提交信息
3. Go代码添加注释严格参考Go语言官方文档注释形式
4. 在所有要使用到json编码解码的场景中，都必须使用github.com/dobyte/due/v2/encoding/json包
5. 在所有要使用到errors包的场景中，都必须使用github.com/dobyte/due/v2/errors包
6. 在所有高性能场景中，能不用go defer就不用

### 注释规范

1. 函数和方法的注释格式参考如下：

```go
// Backoff 指数退避调用函数
// 按指数递增的间隔重试调用 fn：第 i 次尝试的间隔为 1<<i 倍的基础延迟（封顶为 maxDelay），
// 直到 fn 返回 next=false、ctx 被取消或达到最大重试次数 retry
// @param ctx context.Context 上下文
// @param fn func(ctx context.Context, attempt int) (bool, error) 待调用的函数，attempt 为当前尝试次数（从1开始），返回值 next 表示是否继续重试
// @param retry int 最大重试次数
// @param baseDelay time.Duration 基础延迟时间
// @param maxDelay time.Duration 最大延迟时间
// @return @1 error 最后一次调用返回的错误；若 ctx 被取消，则返回 ctx.Err()
func Backoff(ctx context.Context, fn func(ctx context.Context, attempt int) (bool, error), retry int, baseDelay, maxDelay time.Duration) error {
    // 实现指数退避调用函数的逻辑
    // ...
}
```

### 发布版本

#### 变动内容参考

```markdown
## 新增模块

- network/quic：全新的 QUIC 网络实现（客户端/服务端、连接管理器、流写入器、TLS 与代理协议支持）
- config/polaris：新增 Polaris 配置源（source、watcher、配置项、测试）

## 核心模块

- buffer： 统一游标模型并新增`MoveTo` ；重构游标滑动与消息解包；新增`VisitBytes` 遍历与延迟释放；修复对象池分级边界问题；空指针安全加固；基于`bufio` 实现零分配收包。
- queue： 加固等待/队列并发，补充文档与单元测试；actor 销毁后不再派发队列消息。
```

#### 发布流程参考：

1. 首先，你要检查当前是否是main分支，如果不是则不执行任何操作，如果是main分支，则从./core/info/info.go文件中的version值，这个就是本次要发布的版本号
2. 然后，你需要从git的tag中找到上次合并的日志点位，并且整理上个日志点位到当前版本间的所有提交日志，最好能够清晰地表述清楚每个模块修改的内容，清理掉重复内容，要求全部使用中文
3. 接着，用当前版本号创建一个tag，格式为v1.0.0，例如v1.0.0，然后提交到git仓库
4. 最后，你需要将发布日志整理成md文档输出到面板上，并且为了方便我进行复制，你最好使用md富文本框

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
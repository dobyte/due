package server

import (
	"context"
	"github.com/dobyte/due/v2/log"
	"google.golang.org/grpc"
	"runtime"
)

// recoverInterceptor 一元调用恢复拦截器
// 捕获处理器执行期间产生的 panic，运行时错误直接抛出，其他错误记录日志
// @param ctx context.Context 上下文
// @param req any 请求参数
// @param info *grpc.UnaryServerInfo 服务方法信息
// @param handler grpc.UnaryHandler 处理方法
// @return @1 any 响应参数
// @return @2 error 错误信息
func recoverInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (any, error) {
	defer func() {
		if err := recover(); err != nil {
			switch err.(type) {
			case runtime.Error:
				log.Panic(err)
			default:
				log.Panicf("panic error: %v", err)
			}
		}
	}()

	return handler(ctx, req)
}

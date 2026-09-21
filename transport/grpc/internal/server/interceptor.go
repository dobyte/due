package server

import (
	"context"
	"runtime"

	"github.com/dobyte/due/v2/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// recoverInterceptor 一元调用恢复拦截器
// 捕获处理器执行期间产生的 panic，记录日志并向客户端返回内部错误
// @param ctx context.Context 上下文
// @param req any 请求参数
// @param info *grpc.UnaryServerInfo 服务方法信息
// @param handler grpc.UnaryHandler 处理方法
// @return @1 any 响应参数
// @return @2 error 错误信息
func recoverInterceptor(ctx context.Context, req any, info *grpc.UnaryServerInfo, handler grpc.UnaryHandler) (resp any, err error) {
	defer func() {
		if r := recover(); r != nil {
			switch r.(type) {
			case runtime.Error:
				log.Error(r)
			default:
				log.Errorf("panic error: %v", r)
			}

			err = status.Error(codes.Internal, "internal server error")
		}
	}()

	return handler(ctx, req)
}

package server

import (
	"context"
	"github.com/dobyte/due/v2/log"
	"google.golang.org/grpc"
	"runtime"
)

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

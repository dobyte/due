package server

import (
	"context"
	"runtime"

	"github.com/dobyte/due/v2/log"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// recoverInterceptor is a unary call recovery interceptor.
//
// It catches panics raised while the handler is running, logs them and returns an internal error to
// the client.
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

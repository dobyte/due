package xlocation

import (
	"context"

	"github.com/dobyte/due/v2/core/location"
)

var globalLocation = location.NewLocation(
	location.NewIPWHOISResolver(),
	location.NewIPAPICOResolver(),
	location.NewIPAPICOMResolver(),
)

// Parse 解析 IP 地址
// @param ctx context.Context 上下文，用于超时控制
// @param ip string IP 地址
// @return @1 *location.Result 解析结果
// @return @2 error 错误信息
func Parse(ctx context.Context, ip string) (*location.Result, error) {
	return globalLocation.Parse(ctx, ip)
}

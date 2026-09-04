package location

import (
	"context"
	"sync"
	"sync/atomic"

	"github.com/dobyte/due/v2/errors"
)

// Result 表示 IP 地址的地理位置解析结果
type Result struct {
	IP       string `json:"ip"`       // IP地址
	Country  string `json:"country"`  // 国家
	Province string `json:"province"` // 省/自治区/直辖市
	City     string `json:"city"`     // 城市
	ISP      string `json:"isp"`      // 运营商
}

// Location 表示地理位置解析器，聚合多个解析器并发解析 IP 地址
type Location struct {
	resolvers []Resolver // 解析器列表
}

// NewLocation 创建一个新的 Location 实例
func NewLocation(resolvers ...Resolver) *Location {
	return &Location{
		resolvers: resolvers,
	}
}

// Parse 解析 IP 地址的地理位置信息
// @param ctx context.Context 上下文，用于超时控制
// @param ip string IP 地址
// @return @1 *Result 解析结果
// @return @2 error 错误信息
func (l *Location) Parse(ctx context.Context, ip string) (*Result, error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		ch    = make(chan *Result, len(l.resolvers))
		done  = make(chan struct{})
		wg    sync.WaitGroup
		state atomic.Bool
	)

	for _, r := range l.resolvers {
		wg.Go(func() {
			loc, err := r.Resolve(ctx, ip)
			if err != nil {
				return
			}

			if state.CompareAndSwap(false, true) {
				ch <- loc
			}
		})
	}

	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case loc := <-ch:
		return loc, nil
	case <-done:
		select {
		case loc := <-ch:
			return loc, nil
		default:
			if err := ctx.Err(); err != nil {
				return nil, err
			}
			return nil, errors.ErrNotFoundIPAddress
		}
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

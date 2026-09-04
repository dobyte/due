package location

import (
	"context"
	"fmt"
	"net/url"

	"github.com/dobyte/due/v2/errors"
)

// IPWHOISResolver 基于 ipwho.is 服务的 IP 地址解析器
type IPWHOISResolver struct {
}

var _ Resolver = (*IPWHOISResolver)(nil)

// NewIPWHOISResolver 创建一个基于 ipwho.is 服务的 IP 地址解析器
func NewIPWHOISResolver() *IPWHOISResolver {
	return &IPWHOISResolver{}
}

// Name 获取解析器名称
func (i *IPWHOISResolver) Name() string {
	return "ipwho.is"
}

// Resolve 解析 IP 地址
// @param ctx context.Context 上下文，用于超时控制
// @param ip string IP 地址
// @return @1 *Result 解析结果
// @return @2 error 错误信息
func (i *IPWHOISResolver) Resolve(ctx context.Context, ip string) (*Result, error) {
	var resp struct {
		Success    bool   `json:"success"`
		Message    string `json:"message"`
		IP         string `json:"ip"`
		Country    string `json:"country"`
		Region     string `json:"region"`
		City       string `json:"city"`
		Connection struct {
			ISP string `json:"isp"`
		} `json:"connection"`
	}

	endpoint := fmt.Sprintf("https://ipwho.is/%s?lang=zh-CN", url.PathEscape(ip))

	if err := fetchJSON(ctx, endpoint, &resp); err != nil {
		return nil, err
	}

	if resp.Success {
		return &Result{
			IP:       resp.IP,
			Country:  resp.Country,
			Province: resp.Region,
			City:     resp.City,
			ISP:      normalizeISP(resp.Connection.ISP),
		}, nil
	}

	if resp.Message != "" {
		return nil, errors.New(resp.Message)
	}

	return nil, errors.New("query failed")
}

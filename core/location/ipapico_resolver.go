package location

import (
	"context"
	"fmt"
	"net/url"

	"github.com/dobyte/due/v2/errors"
)

// IPAPICOResolver 基于 ipapi.co 服务的 IP 地址解析器
type IPAPICOResolver struct {
}

var _ Resolver = (*IPAPICOResolver)(nil)

// NewIPAPICOResolver 创建一个基于 ipapi.co 服务的 IP 地址解析器
func NewIPAPICOResolver() *IPAPICOResolver {
	return &IPAPICOResolver{}
}

// Name 获取解析器名称
func (i *IPAPICOResolver) Name() string {
	return "ipapi.co"
}

// Resolve 解析 IP 地址
// @param ctx context.Context 上下文，用于超时控制
// @param ip string IP 地址
// @return @1 *Result 解析结果
// @return @2 error 错误信息
func (i *IPAPICOResolver) Resolve(ctx context.Context, ip string) (*Result, error) {
	var resp struct {
		IP      string `json:"ip"`
		Country string `json:"country_name"`
		Region  string `json:"region"`
		City    string `json:"city"`
		Org     string `json:"org"`
		Error   bool   `json:"error"`
		Reason  string `json:"reason"`
	}

	endpoint := fmt.Sprintf("https://ipapi.co/%s/json/", url.PathEscape(ip))

	if err := fetchJSON(ctx, endpoint, &resp); err != nil {
		return nil, err
	}

	if resp.Error {
		if resp.Reason != "" {
			return nil, errors.New(resp.Reason)
		}

		return nil, errors.New("query failed")
	}

	return &Result{
		IP:       resp.IP,
		Country:  resp.Country,
		Province: resp.Region,
		City:     resp.City,
		ISP:      normalizeISP(resp.Org),
	}, nil
}

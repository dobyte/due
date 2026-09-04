package location

import (
	"context"
	"fmt"
	"net/url"

	"github.com/dobyte/due/v2/errors"
)

// IPAPICOMResolver 基于 ip-api.com 服务的 IP 地址解析器
type IPAPICOMResolver struct {
}

var _ Resolver = (*IPAPICOMResolver)(nil)

// NewIPAPICOMResolver 创建一个基于 ip-api.com 服务的 IP 地址解析器
func NewIPAPICOMResolver() *IPAPICOMResolver {
	return &IPAPICOMResolver{}
}

// Name 获取解析器名称
func (i *IPAPICOMResolver) Name() string {
	return "ip-api.com"
}

// Resolve 解析 IP 地址
// @param ctx context.Context 上下文，用于超时控制
// @param ip string IP 地址
// @return @1 *Result 解析结果
// @return @2 error 错误信息
func (i *IPAPICOMResolver) Resolve(ctx context.Context, ip string) (*Result, error) {
	var resp struct {
		Status     string `json:"status"`
		Message    string `json:"message"`
		Country    string `json:"country"`
		RegionName string `json:"regionName"`
		City       string `json:"city"`
		ISP        string `json:"isp"`
		Query      string `json:"query"`
	}

	endpoint := fmt.Sprintf("http://ip-api.com/json/%s?lang=zh-CN&fields=status,message,country,regionName,city,isp,query", url.PathEscape(ip))

	if err := fetchJSON(ctx, endpoint, &resp); err != nil {
		return nil, err
	}

	if resp.Status != "success" {
		if resp.Message != "" {
			return nil, errors.New(resp.Message)
		}

		return nil, errors.New("query failed")
	}

	return &Result{
		IP:       resp.Query,
		Country:  resp.Country,
		Province: resp.RegionName,
		City:     resp.City,
		ISP:      normalizeISP(resp.ISP),
	}, nil
}

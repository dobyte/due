package xnet

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
)

const (
	locationQueryTimeout    = 3 * time.Second // IP归属地查询的兜底超时时间
	maxLocationResponseSize = 1 << 20         // 限制响应体最大读取字节数(1MB)
)

// Location IP归属地信息
type Location struct {
	IP       string `json:"ip"`       // IP地址
	Country  string `json:"country"`  // 国家
	Province string `json:"province"` // 省/自治区/直辖市
	City     string `json:"city"`     // 城市
	ISP      string `json:"isp"`      // 运营商
}

// ipLocationProvider IP归属地查询服务
type ipLocationProvider struct {
	name  string
	url   string
	query func(ctx context.Context, url, ip string) (*Location, error)
}

// 内置的IP归属地查询服务
var locationProviders = []ipLocationProvider{
	{name: "ip-api.com", url: "http://ip-api.com", query: locateByIPAPI},
	{name: "ipwho.is", url: "https://ipwho.is", query: locateByIPWhois},
	{name: "ipapi.co", url: "https://ipapi.co", query: locateByIPAPICo},
}

// httpClient HTTP客户端，复用底层连接池
// Timeout作为兜底超时：调用方ctx无截止时间时防止请求永久挂起；
// 调用方ctx截止时间更早时，请求会经由请求上下文优先被取消
var httpClient = &http.Client{Timeout: locationQueryTimeout}

// LocateIP 查询IP地址的归属地
// 并发请求多个第三方接口，返回最先返回且数据有效的结果，保证返回数据的一致性
// @param ctx context.Context 上下文，用于超时控制
// @param ip string 待查询的IP地址
// @return @1 *Location IP归属地信息
// @return @2 error 错误信息
func LocateIP(ctx context.Context, ip string) (*Location, error) {
	if net.ParseIP(ip) == nil {
		return nil, errors.ErrInvalidArgument
	}

	return locateIP(ctx, ip, locationProviders)
}

// locateIP 基于指定的归属地查询服务并发查询IP地址归属地
// 并发请求多个第三方接口，返回最先返回且数据有效的结果，保证返回数据的一致性
// @param ctx context.Context 上下文，用于超时控制
// @param ip string 待查询的IP地址
// @param providers []ipLocationProvider 归属地查询服务列表
// @return @1 *Location IP归属地信息
// @return @2 error 错误信息
func locateIP(ctx context.Context, ip string, providers []ipLocationProvider) (*Location, error) {
	// 派生可取消的上下文：任一查询返回结果后，立即取消其余在途请求，避免无用的并发开销
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	var (
		ch    = make(chan *Location, len(providers))
		done  = make(chan struct{})
		state atomic.Bool
		wg    sync.WaitGroup
	)

	for _, p := range providers {
		wg.Add(1)
		go func(p ipLocationProvider) {
			defer wg.Done()

			loc, err := p.query(ctx, p.url, ip)
			if err != nil || !isConsistent(loc) {
				return
			}

			loc.ISP = normalizeISP(loc.ISP)
			if state.CompareAndSwap(false, true) {
				ch <- loc
			}
		}(p)
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

// isConsistent 校验归属地信息是否有效，用于保证返回数据的一致性
// @param loc *Location IP归属地信息
// @return @1 bool 是否有效
func isConsistent(loc *Location) bool {
	return loc != nil && loc.Country != ""
}

// normalizeISP 将运营商名称转换为中文
// @param isp string 运营商名称
// @return @1 string 转换后的中文运营商名称
func normalizeISP(isp string) string {
	lower := strings.ToLower(isp)
	switch {
	case strings.Contains(lower, "china mobile"), strings.Contains(lower, "cmnet"):
		return "移动"
	case strings.Contains(lower, "china unicom"), strings.Contains(lower, "unicom"), strings.Contains(lower, "cncgroup"):
		return "联通"
	case strings.Contains(lower, "china telecom"), strings.Contains(lower, "chinanet"):
		return "电信"
	case strings.Contains(lower, "china broadcasting"), strings.Contains(lower, "cbn"):
		return "广电"
	case strings.Contains(lower, "tietong"), strings.Contains(lower, "crtc"):
		return "铁通"
	case strings.Contains(lower, "cernet"), strings.Contains(lower, "education"):
		return "教育网"
	default:
		return isp
	}
}

// fetchJSON 发起HTTP请求并解析JSON响应
// @param ctx context.Context 上下文，用于超时控制
// @param url string 请求地址
// @param out any 解析目标
// @return @1 error 错误信息
func fetchJSON(ctx context.Context, url string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// 消费完响应体以便连接可被复用
		_, _ = io.Copy(io.Discard, resp.Body)
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxLocationResponseSize+1))
	if err != nil {
		return err
	}

	if len(data) > maxLocationResponseSize {
		return fmt.Errorf("response body too large: %d bytes", len(data))
	}

	return json.Unmarshal(data, out)
}

// locateByIPAPI 通过 ip-api.com 查询
// @param ctx context.Context 上下文，用于超时控制
// @param url string 接口地址
// @param ip string 待查询的IP地址
// @return @1 *Location IP归属地信息
// @return @2 error 错误信息
func locateByIPAPI(ctx context.Context, url, ip string) (*Location, error) {
	var resp struct {
		Status     string `json:"status"`
		Message    string `json:"message"`
		Country    string `json:"country"`
		RegionName string `json:"regionName"`
		City       string `json:"city"`
		ISP        string `json:"isp"`
		Query      string `json:"query"`
	}

	requestURL := fmt.Sprintf("%s/json/%s?lang=zh-CN&fields=status,message,country,regionName,city,isp,query", url, ip)
	if err := fetchJSON(ctx, requestURL, &resp); err != nil {
		return nil, err
	}

	if resp.Status != "success" {
		if resp.Message != "" {
			return nil, fmt.Errorf("ip-api.com: %s", resp.Message)
		}
		return nil, fmt.Errorf("ip-api.com: query failed")
	}

	return &Location{
		IP:       resp.Query,
		Country:  resp.Country,
		Province: resp.RegionName,
		City:     resp.City,
		ISP:      resp.ISP,
	}, nil
}

// locateByIPWhois 通过 ipwho.is 查询
// @param ctx context.Context 上下文，用于超时控制
// @param url string 接口地址
// @param ip string 待查询的IP地址
// @return @1 *Location IP归属地信息
// @return @2 error 错误信息
func locateByIPWhois(ctx context.Context, url, ip string) (*Location, error) {
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

	requestURL := fmt.Sprintf("%s/%s?lang=zh-CN", url, ip)
	if err := fetchJSON(ctx, requestURL, &resp); err != nil {
		return nil, err
	}

	if !resp.Success {
		if resp.Message != "" {
			return nil, fmt.Errorf("ipwho.is: %s", resp.Message)
		}
		return nil, fmt.Errorf("ipwho.is: query failed")
	}

	return &Location{
		IP:       resp.IP,
		Country:  resp.Country,
		Province: resp.Region,
		City:     resp.City,
		ISP:      resp.Connection.ISP,
	}, nil
}

// locateByIPAPICo 通过 ipapi.co 查询
// @param ctx context.Context 上下文，用于超时控制
// @param url string 接口地址
// @param ip string 待查询的IP地址
// @return @1 *Location IP归属地信息
// @return @2 error 错误信息
func locateByIPAPICo(ctx context.Context, url, ip string) (*Location, error) {
	var resp struct {
		IP      string `json:"ip"`
		Country string `json:"country_name"`
		Region  string `json:"region"`
		City    string `json:"city"`
		Org     string `json:"org"`
		Error   bool   `json:"error"`
		Reason  string `json:"reason"`
	}

	requestURL := fmt.Sprintf("%s/%s/json/", url, ip)
	if err := fetchJSON(ctx, requestURL, &resp); err != nil {
		return nil, err
	}

	if resp.Error {
		if resp.Reason != "" {
			return nil, fmt.Errorf("ipapi.co: %s", resp.Reason)
		}
		return nil, fmt.Errorf("ipapi.co: query failed")
	}

	return &Location{
		IP:       resp.IP,
		Country:  resp.Country,
		Province: resp.Region,
		City:     resp.City,
		ISP:      resp.Org,
	}, nil
}

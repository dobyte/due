package location

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/dobyte/due/v2/encoding/json"
	"github.com/dobyte/due/v2/errors"
)

const (
	maxResponseSize = 1 << 20 // 限制响应体最大读取字节数(1MB)
)

// Resolver 定义 IP 地址解析器接口
type Resolver interface {
	// Name 获取解析器名称
	Name() string
	// Resolve 解析 IP 地址
	Resolve(ctx context.Context, ip string) (*Result, error)
}

var httpClient = &http.Client{Timeout: 3 * time.Second}

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
		_, _ = io.Copy(io.Discard, resp.Body)
		return errors.New(fmt.Sprintf("unexpected status code: %d", resp.StatusCode))
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxResponseSize+1))
	if err != nil {
		return err
	}

	if len(data) > maxResponseSize {
		return errors.New(fmt.Sprintf("response body too large: %d bytes", len(data)))
	}

	return json.Unmarshal(data, out)
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

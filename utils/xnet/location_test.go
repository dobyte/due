package xnet

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/dobyte/due/v2/errors"
)

// TestLocateIP_InvalidIP 验证LocateIP入口对非法IP参数的校验
func TestLocateIP_InvalidIP(t *testing.T) {
	if _, err := LocateIP(context.Background(), "invalid"); !errors.Is(err, errors.ErrInvalidArgument) {
		t.Fatalf("expected ErrInvalidArgument, got %v", err)
	}
}

func TestLocateIP_FirstWins(t *testing.T) {
	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"status":"success","country":"中国"}`))
	}))
	defer fast.Close()

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"ip":"8.8.8.8","country":"美国","region":"加利福尼亚州","city":"山景城","connection":{"isp":"Google"}}`))
	}))
	defer slow.Close()

	loc, err := locateIP(context.Background(), "8.8.8.8", []ipLocationProvider{
		{name: "ip-api.com", url: fast.URL, query: locateByIPAPI},
		{name: "ipwho.is", url: slow.URL, query: locateByIPWhois},
	})
	if err != nil {
		t.Fatalf("locateIP() unexpected error: %v", err)
	}
	if loc.Country != "中国" {
		t.Fatalf("expected country 中国, got %q", loc.Country)
	}
}

func TestLocateIP_Fallback(t *testing.T) {
	s1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "rate limited", http.StatusTooManyRequests)
	}))
	defer s1.Close()

	s2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"ip":"8.8.8.8","country":"美国","region":"加利福尼亚州","city":"山景城","connection":{"isp":"Google"}}`))
	}))
	defer s2.Close()

	loc, err := locateIP(context.Background(), "8.8.8.8", []ipLocationProvider{
		{name: "ip-api.com", url: s1.URL, query: locateByIPAPI},
		{name: "ipwho.is", url: s2.URL, query: locateByIPWhois},
	})
	if err != nil {
		t.Fatalf("locateIP() unexpected error: %v", err)
	}
	if loc.Country != "美国" {
		t.Fatalf("expected country 美国, got %q", loc.Country)
	}
}

func TestLocateIP_SkipInconsistent(t *testing.T) {
	fast := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// 缺少 country，数据不一致，应被跳过
		_, _ = w.Write([]byte(`{"status":"success"}`))
	}))
	defer fast.Close()

	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(100 * time.Millisecond)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"success":true,"ip":"8.8.8.8","country":"美国","region":"加利福尼亚州","city":"山景城","connection":{"isp":"Google"}}`))
	}))
	defer slow.Close()

	loc, err := locateIP(context.Background(), "8.8.8.8", []ipLocationProvider{
		{name: "ip-api.com", url: fast.URL, query: locateByIPAPI},
		{name: "ipwho.is", url: slow.URL, query: locateByIPWhois},
	})
	if err != nil {
		t.Fatalf("locateIP() unexpected error: %v", err)
	}
	if loc.Country != "美国" {
		t.Fatalf("expected country 美国, got %q", loc.Country)
	}
}

func TestLocateIP_AllFailed(t *testing.T) {
	handler := func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "internal error", http.StatusInternalServerError)
	}

	s1 := httptest.NewServer(http.HandlerFunc(handler))
	defer s1.Close()

	s2 := httptest.NewServer(http.HandlerFunc(handler))
	defer s2.Close()

	if _, err := locateIP(context.Background(), "8.8.8.8", []ipLocationProvider{
		{name: "ip-api.com", url: s1.URL, query: locateByIPAPI},
		{name: "ipwho.is", url: s2.URL, query: locateByIPWhois},
	}); !errors.Is(err, errors.ErrNotFoundIPAddress) {
		t.Fatalf("expected ErrNotFoundIPAddress, got %v", err)
	}
}

// TestLocateIP_ContextDeadline 验证调用方传入的context可控制查询超时
func TestLocateIP_ContextDeadline(t *testing.T) {
	// 模拟长时间无响应的归属地查询服务：挂起请求直到客户端断开，其响应时间远超locationQueryTimeout
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := locateIP(ctx, "8.8.8.8", []ipLocationProvider{
		{name: "ipwho.is", url: server.URL, query: locateByIPWhois},
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("locateIP() unexpected nil error, want context timeout error")
	}
	if elapsed >= 2*time.Second {
		t.Fatalf("locateIP() should respect context deadline, took %v", elapsed)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("locateIP() error = %v, want context.DeadlineExceeded", err)
	}
}

// TestLocateIP_39_182_14_197 验证指定IP的归属地解析结果（中国·浙江·杭州·移动）
func TestLocateIP_39_182_14_197(t *testing.T) {
	const ip = "39.182.14.197"

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		// 取自 ipwho.is 对 39.182.14.197 的真实响应
		_, _ = w.Write([]byte(`{"ip":"39.182.14.197","success":true,"country":"中国","region":"浙江省","city":"杭州","connection":{"isp":"CMNET-Zhejiang-AP - China Mobile communications corporation"}}`))
	}))
	defer server.Close()

	loc, err := locateIP(context.Background(), ip, []ipLocationProvider{
		{name: "ipwho.is", url: server.URL, query: locateByIPWhois},
	})
	if err != nil {
		t.Fatalf("locateIP(%q) unexpected error: %v", ip, err)
	}

	t.Logf("locateIP(%q) = %v", ip, loc)

	if loc.IP != ip {
		t.Errorf("IP = %q, want %q", loc.IP, ip)
	}
	if loc.Country != "中国" {
		t.Errorf("Country = %q, want 中国", loc.Country)
	}
	if loc.Province != "浙江省" {
		t.Errorf("Province = %q, want 浙江省", loc.Province)
	}
	if loc.City != "杭州" {
		t.Errorf("City = %q, want 杭州", loc.City)
	}
	if loc.ISP != "移动" {
		t.Errorf("ISP = %q, want 移动", loc.ISP)
	}
}

// buildChinaIPs 生成不少于100个国内IP地址
func buildChinaIPs() []string {
	prefixes := []string{
		"14.23", "27.115", "36.110", "39.182", "42.120", "49.7", "58.60", "59.37",
		"60.12", "61.135", "101.71", "103.235", "106.14", "110.64", "111.13", "112.17",
		"113.108", "114.80", "115.238", "116.24", "117.136", "118.112", "119.147", "120.36",
		"121.8", "122.224", "123.58", "124.74", "125.118", "140.207", "144.52", "150.138",
		"153.3", "157.0", "159.226", "163.125", "171.113", "175.6", "180.168", "182.92",
		"183.6", "202.108", "203.208", "210.75", "211.151", "218.108", "219.136", "220.181",
		"221.130", "222.85", "223.5",
	}

	ips := make([]string, 0, len(prefixes)*2)
	for _, p := range prefixes {
		ips = append(ips, fmt.Sprintf("%s.1.1", p))
		ips = append(ips, fmt.Sprintf("%s.2.2", p))
	}
	return ips
}

// TestLocateIP_ChinaIPs 使用不少于100个国内IP地址验证归属地解析
func TestLocateIP_ChinaIPs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ip := strings.TrimPrefix(r.URL.Path, "/")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"ip":%q,"success":true,"country":"中国","region":"浙江省","city":"杭州","connection":{"isp":"China Mobile communications corporation"}}`, ip)
	}))
	defer server.Close()

	ips := buildChinaIPs()
	if len(ips) < 100 {
		t.Fatalf("test requires at least 100 ip addresses, got %d", len(ips))
	}

	for _, ip := range ips {
		loc, err := locateIP(context.Background(), ip, []ipLocationProvider{
			{name: "ipwho.is", url: server.URL, query: locateByIPWhois},
		})
		if err != nil {
			t.Fatalf("locateIP(%q) unexpected error: %v", ip, err)
		}
		if loc.IP != ip {
			t.Errorf("IP = %q, want %q", loc.IP, ip)
		}
		if loc.Country != "中国" {
			t.Errorf("Country = %q, want 中国", loc.Country)
		}
		if loc.ISP != "移动" {
			t.Errorf("ISP = %q, want 移动", loc.ISP)
		}
	}
}

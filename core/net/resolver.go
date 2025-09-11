package net

import (
	"io"
	"net"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/errors"
)

var urls = []string{
	"http://ipinfo.io/ip",
	"http://ifconfig.me/ip",
	"https://api.ipquery.io",
	"https://api.ipify.org",
}

type IPResolver func() (string, error)

var (
	globalPublicIPResolver  IPResolver = defaultPublicIPResolver
	globalPrivateIPResolver IPResolver = defaultPrivateIPResolver
)

// SetPublicIPResolver 设置公网IP解析器
func SetPublicIPResolver(resolver IPResolver) {
	if resolver != nil {
		globalPublicIPResolver = resolver
	}
}

// SetPrivateIPResolver 设置私网IP解析器
func SetPrivateIPResolver(resolver IPResolver) {
	if resolver != nil {
		globalPrivateIPResolver = resolver
	}
}

// 默认私网IP解析器
func defaultPrivateIPResolver() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	var (
		addrs []net.Addr
		ipnet net.IP
		ip    string
	)
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue
		}

		if iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		if addrs, err = iface.Addrs(); err != nil {
			return "", err
		}

		for _, addr := range addrs {
			switch v := addr.(type) {
			case *net.IPNet:
				ipnet = v.IP
			case *net.IPAddr:
				ipnet = v.IP
			default:
				continue
			}

			if ipnet == nil || ipnet.IsLoopback() {
				continue
			}

			if ipv4 := ipnet.To4(); ipv4 != nil && ipv4.IsPrivate() {
				if ipv4[0] == 192 && ipv4[1] == 168 {
					return ipv4.String(), nil
				}

				if ip == "" {
					ip = ipv4.String()
				}
			}
		}
	}

	if ip != "" {
		return ip, nil
	} else {
		return "", errors.ErrNotFoundIPAddress
	}
}

// 默认公网IP解析器
func defaultPublicIPResolver() (string, error) {
	var (
		ch      = make(chan string)
		state   atomic.Bool
		timeout = 500 * time.Millisecond
	)

	for _, url := range urls {
		go func() {
			if ip, err := doQueryPublicIP(url, timeout); err == nil {
				if state.CompareAndSwap(false, true) {
					ch <- ip
				}
			}
		}()
	}

	defer close(ch)

	select {
	case ip := <-ch:
		return ip, nil
	case <-time.After(timeout):
		return "", errors.ErrNotFoundIPAddress
	}
}

// 获取公网IP地址
func doQueryPublicIP(url string, timeout time.Duration) (string, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: timeout}

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if ip := net.ParseIP(string(body)); ip == nil {
		return "", errors.ErrNotFoundIPAddress
	} else {
		return ip.String(), nil
	}
}

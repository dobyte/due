package net

import (
	"io"
	"net"
	"net/http"
	"strconv"
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

const (
	IPv4Zero     = "0.0.0.0"
	IPv4Loopback = "127.0.0.1"
)

// ParseAddr 解析地址
// 注：仅在addr为0.0.0.0:[port]或:[port]时才会根据wan参数自动获取暴露IP
func ParseAddr(addr string, expose ...bool) (string, string, error) {
	var (
		err        error
		host       string
		port       string
		listenHost string
		exposeHost string
	)

	if addr != "" {
		if host, port, err = net.SplitHostPort(addr); err != nil {
			return "", "", err
		}
	}

	if port == "" || port == "0" {
		if p, err := AssignRandPort(host); err != nil {
			return "", "", err
		} else {
			port = strconv.Itoa(p)
		}
	}

	if host != "" && host != IPv4Zero && host != "[::]" && host != "::" {
		listenHost = host
		exposeHost = host
	} else {
		if len(expose) > 0 && expose[0] {
			if ip, err := PublicIP(); err != nil {
				return "", "", err
			} else {
				exposeHost = ip
			}
		} else {
			if ip, err := PrivateIP(); err != nil {
				return "", "", err
			} else {
				exposeHost = ip
			}
		}

		listenHost = IPv4Zero
	}

	return net.JoinHostPort(listenHost, port), net.JoinHostPort(exposeHost, port), nil
}

// ExtractIP 提取主机地址
func ExtractIP(addr net.Addr) (ip string, err error) {
	ip, _, err = net.SplitHostPort(addr.String())
	return
}

// ExtractPort 提取主机端口
func ExtractPort(addr net.Addr) (int, error) {
	_, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(port)
}

// ExternalIP 获取外网IP地址
//
// Deprecated: As of due 2.2.8, this function simply calls [net.PublicIP].
func ExternalIP() (string, error) {
	return PublicIP()
}

// InternalIP 获取内网IP地址
//
// Deprecated: As of due 2.2.8, this function simply calls [net.PublicIP].
func InternalIP() (string, error) {
	return PrivateIP()
}

// PublicIP 获取公网IP
func PublicIP() (string, error) {
	var (
		ch      = make(chan string)
		state   atomic.Bool
		timeout = 500 * time.Millisecond
	)

	for _, url := range urls {
		go func() {
			if ip, err := queryPublicIP(url, timeout); err == nil {
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

// PrivateIP 获取私网IP
func PrivateIP() (string, error) {
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

// 获取外网IP地址
func queryPublicIP(url string, timeout time.Duration) (string, error) {
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

// AssignRandPort 分配一个随机端口
func AssignRandPort(ip ...string) (int, error) {
	addr := ":0"
	if len(ip) > 0 {
		addr = ip[0] + addr
	}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return 0, err
	}

	port := listener.Addr().(*net.TCPAddr).Port

	_ = listener.Close()

	return port, nil
}

// FulfillAddr 补全地址
func FulfillAddr(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" {
		host = IPv4Zero
	}

	return net.JoinHostPort(host, port)
}

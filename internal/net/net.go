package net

import (
	"github.com/dobyte/due/errors"
	"net"
	"strconv"
)

// ParseAddr 解析地址
func ParseAddr(addr string) (listenAddr, exposeAddr string, err error) {
	var host, port string

	if addr != "" {
		host, port, err = net.SplitHostPort(addr)
		if err != nil {
			return
		}
	}

	if port == "" || port == "0" {
		p, err := AssignRandPort(host)
		if err != nil {
			return "", "", err
		}
		port = strconv.Itoa(p)
	}

	if len(host) > 0 && (host != "0.0.0.0" && host != "[::]" && host != "::") {
		listenAddr = net.JoinHostPort(host, port)
		exposeAddr = listenAddr
	} else {
		ip, err := InternalIP()
		if err != nil {
			return "", "", err
		}
		listenAddr = net.JoinHostPort("0.0.0.0", port)
		exposeAddr = net.JoinHostPort(ip, port)
	}

	return
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

// InternalIP 获取内网IP地址
func InternalIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}

	var (
		addrs []net.Addr
		ipnet net.IP
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
				err = errors.New("invalid addr interface")
				continue
			}

			if ipnet == nil || ipnet.IsLoopback() {
				continue
			}

			if ipv4 := ipnet.To4(); ipv4 != nil {
				return ipv4.String(), nil
			}
		}
	}

	return "", errors.New("not found ip address")
}

// ExternalIP 获取外网IP地址
func ExternalIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:54")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	return ExtractIP(conn.LocalAddr())
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
		host = "0.0.0.0"
	}

	return net.JoinHostPort(host, port)
}

/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/28 12:13 下午
 * @Desc: TODO
 */

package xnet

import (
	"errors"
	"net"
	"strconv"
)

const (
	localhost = "127.0.0.1"
)

// ExtractIP 提取主机地址
func ExtractIP(addr net.Addr) (host string, err error) {
	host, _, err = net.SplitHostPort(addr.String())
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

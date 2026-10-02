package net

import (
	"net"
	"net/url"
	"strconv"

	"github.com/dobyte/due/v2/errors"
)

const (
	IPv4Zero     = "0.0.0.0"
	IPv4Loopback = "127.0.0.1"
)

// ParseAddr parses addr into a listen address and an expose address.
//
// The expose IP is only resolved automatically from the expose argument when addr is
// 0.0.0.0:[port] or :[port].
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

// ExtractIP extracts the host from addr.
func ExtractIP(addr net.Addr) (ip string, err error) {
	ip, _, err = net.SplitHostPort(addr.String())
	return
}

// ExtractPort extracts the port from addr.
func ExtractPort(addr net.Addr) (int, error) {
	_, port, err := net.SplitHostPort(addr.String())
	if err != nil {
		return 0, err
	}
	return strconv.Atoi(port)
}

// ExternalIP returns the external IP address.
//
// Deprecated: As of due v2.3.0, this function simply calls [net.PublicIP].
func ExternalIP() (string, error) {
	return PublicIP()
}

// InternalIP returns the internal IP address.
//
// Deprecated: As of due v2.3.0, this function simply calls [net.PublicIP].
func InternalIP() (string, error) {
	return PrivateIP()
}

// PublicIP returns the public IP address.
func PublicIP() (string, error) {
	if globalPublicIPResolver != nil {
		return globalPublicIPResolver()
	} else {
		return "", errors.ErrNotFoundIPAddress
	}
}

// PrivateIP returns the private IP address.
func PrivateIP() (string, error) {
	if globalPrivateIPResolver != nil {
		return globalPrivateIPResolver()
	} else {
		return "", errors.ErrNotFoundIPAddress
	}
}

// AssignRandPort assigns a random port, optionally bound to ip.
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

// FulfillAddr completes addr with the default host when it is missing.
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

// ParseHostPort parses an endpoint into a host and a port.
func ParseHostPort(endpoint string) (string, uint64, error) {
	raw, err := url.Parse(endpoint)
	if err != nil {
		return "", 0, err
	}

	host, p, err := net.SplitHostPort(raw.Host)
	if err != nil {
		return "", 0, err
	}

	port, err := strconv.ParseUint(p, 10, 64)
	if err != nil {
		return "", 0, err
	}

	return host, port, nil
}

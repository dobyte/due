// Author: fuxiao
// Email: 576101059@qq.com
// Date: 2022/05/28 12:13 PM

package xnet

import (
	"encoding/binary"
	"net"

	innernet "github.com/dobyte/due/v2/core/net"
)

// ExtractIP extracts the host IP address from addr.
func ExtractIP(addr net.Addr) (string, error) {
	return innernet.ExtractIP(addr)
}

// ExtractPort extracts the host port from addr.
func ExtractPort(addr net.Addr) (int, error) {
	return innernet.ExtractPort(addr)
}

// InternalIP returns the internal (private) IP address.
func InternalIP() (string, error) {
	return innernet.InternalIP()
}

// ExternalIP returns the external IP address.
func ExternalIP() (string, error) {
	return innernet.ExternalIP()
}

// PublicIP returns the public IP address.
func PublicIP() (string, error) {
	return innernet.PublicIP()
}

// PrivateIP returns the private IP address.
func PrivateIP() (string, error) {
	return innernet.PrivateIP()
}

// FulfillAddr completes addr so that it is a valid, fully qualified address.
func FulfillAddr(addr string) string {
	return innernet.FulfillAddr(addr)
}

// AssignRandPort allocates a free random port, optionally bound to one of the given IP addresses.
func AssignRandPort(ip ...string) (int, error) {
	return innernet.AssignRandPort(ip...)
}

// IP2Long converts an IPv4 address into its uint32 representation. It returns 0 when ip is not a
// valid IPv4 address.
func IP2Long(ip string) uint32 {
	v := net.ParseIP(ip).To4()

	if len(v) == 0 {
		return 0
	}

	return binary.BigEndian.Uint32(v)
}

// Long2IP converts a uint32 into its dotted-decimal IPv4 address string.
func Long2IP(v uint32) string {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, v)
	return ip.String()
}

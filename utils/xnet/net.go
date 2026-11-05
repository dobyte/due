// Author: fuxiao
// Email: 576101059@qq.com
// Date: 2022/05/28 12:13 PM

package xnet

import (
	"encoding/binary"
	"net"
	"net/netip"

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

// FulfillAddr completes addr with the dual-stack wildcard host when the host is missing.
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
	addr, err := netip.ParseAddr(ip)
	if err != nil || !(addr.Is4() || addr.Is4In6()) {
		return 0
	}

	v := addr.As4()

	return binary.BigEndian.Uint32(v[:])
}

// Long2IP converts a uint32 into its dotted-decimal IPv4 address string.
func Long2IP(v uint32) string {
	var ip [4]byte
	binary.BigEndian.PutUint32(ip[:], v)
	return net.IP(ip[:]).String()
}

// IP2Bytes converts an IP address into its 16-byte network-order representation. IPv4 addresses
// use the IPv4-mapped IPv6 form, so Bytes2IP can restore the dotted-decimal string. Invalid
// addresses return the zero value.
func IP2Bytes(ip string) [16]byte {
	var raw [16]byte

	if addr, err := netip.ParseAddr(ip); err == nil {
		raw = addr.As16()
	}

	return raw
}

// Bytes2IP converts a 16-byte network-order address into its string representation. IPv4-mapped
// addresses are rendered as dotted-decimal IPv4.
func Bytes2IP(v [16]byte) string {
	return netip.AddrFrom16(v).Unmap().String()
}

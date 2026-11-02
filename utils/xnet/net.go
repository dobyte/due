// Author: fuxiao
// Email: 576101059@qq.com
// Date: 2022/05/28 12:13 PM

package xnet

import (
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

// FulfillAddr completes addr with the dual-stack wildcard host when the host is missing.
func FulfillAddr(addr string) string {
	return innernet.FulfillAddr(addr)
}

// AssignRandPort allocates a free random port, optionally bound to one of the given IP addresses.
func AssignRandPort(ip ...string) (int, error) {
	return innernet.AssignRandPort(ip...)
}

// IP2Long converts an IP address into its 16-byte network-order representation. IPv4 addresses
// use the IPv4-mapped IPv6 form, so Long2IP can restore the dotted-decimal string. Invalid
// addresses return the zero value.
func IP2Long(ip string) [16]byte {
	var bytes [16]byte

	parsed := net.ParseIP(ip)
	if parsed == nil {
		return bytes
	}

	copy(bytes[:], parsed.To16())

	return bytes
}

// Long2IP converts a 16-byte network-order address into its string representation. IPv4-mapped
// addresses are rendered as dotted-decimal IPv4.
func Long2IP(v [16]byte) string {
	return net.IP(v[:]).String()
}

// IP2Bytes converts an IP address into its 16-byte network-order representation. IPv4 addresses
// use the IPv4-mapped IPv6 form, so Bytes2IP can restore the dotted-decimal string. Invalid
// addresses return the zero value.
func IP2Bytes(ip string) [16]byte {
	var raw [16]byte

	parsed := net.ParseIP(ip)
	if parsed == nil {
		return raw
	}

	copy(raw[:], parsed.To16())

	return raw
}

// Bytes2IP converts a 16-byte network-order address into its string representation. IPv4-mapped
// addresses are rendered as dotted-decimal IPv4.
func Bytes2IP(v [16]byte) string {
	return net.IP(v[:]).String()
}

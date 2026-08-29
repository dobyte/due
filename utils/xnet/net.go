/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/28 12:13 下午
 * @Desc: TODO
 */

package xnet

import (
	"encoding/binary"
	"net"

	innernet "github.com/dobyte/due/v2/core/net"
)

// ExtractIP 提取主机地址
// @param addr net.Addr 网络地址
// @return @1 string 主机IP地址
// @return @2 error 错误信息
func ExtractIP(addr net.Addr) (string, error) {
	return innernet.ExtractIP(addr)
}

// ExtractPort 提取主机端口
// @param addr net.Addr 网络地址
// @return @1 int 主机端口
// @return @2 error 错误信息
func ExtractPort(addr net.Addr) (int, error) {
	return innernet.ExtractPort(addr)
}

// InternalIP 获取内网IP地址
// @return @1 string 内网IP地址
// @return @2 error 错误信息
func InternalIP() (string, error) {
	return innernet.InternalIP()
}

// ExternalIP 获取外网IP地址
// @return @1 string 外网IP地址
// @return @2 error 错误信息
func ExternalIP() (string, error) {
	return innernet.ExternalIP()
}

// PublicIP 获取公网IP
// @return @1 string 公网IP地址
// @return @2 error 错误信息
func PublicIP() (string, error) {
	return innernet.PublicIP()
}

// PrivateIP 获取私网IP
// @return @1 string 私网IP地址
// @return @2 error 错误信息
func PrivateIP() (string, error) {
	return innernet.PrivateIP()
}

// FulfillAddr 补全地址
// @param addr string 待补全的地址
// @return @1 string 补全后的地址
func FulfillAddr(addr string) string {
	return innernet.FulfillAddr(addr)
}

// AssignRandPort 分配一个随机端口
// @param ip ...string 可选，绑定的IP地址
// @return @1 int 随机端口号
// @return @2 error 错误信息
func AssignRandPort(ip ...string) (int, error) {
	return innernet.AssignRandPort(ip...)
}

// IP2Long IP地址转换为长整型
// @param ip string IP地址
// @return @1 uint32 转换后的长整型
func IP2Long(ip string) uint32 {
	v := net.ParseIP(ip).To4()

	if len(v) == 0 {
		return 0
	}

	return binary.BigEndian.Uint32(v)
}

// Long2IP 长整型转换为字符串地址
// @param v uint32 长整型
// @return @1 string 转换后的IP地址字符串
func Long2IP(v uint32) string {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, v)
	return ip.String()
}

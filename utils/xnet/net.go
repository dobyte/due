/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/28 12:13 下午
 * @Desc: TODO
 */

package xnet

import (
	innernet "github.com/symsimmy/due/internal/net"
	"net"
)

// ExtractIP 提取主机地址
func ExtractIP(addr net.Addr) (string, error) {
	return innernet.ExtractIP(addr)
}

// ExtractPort 提取主机端口
func ExtractPort(addr net.Addr) (int, error) {
	return innernet.ExtractPort(addr)
}

// InternalIP 获取内网IP地址
func InternalIP() (string, error) {
	return innernet.InternalIP()
}

// ExternalIP 获取外网IP地址
func ExternalIP() (string, error) {
	return innernet.ExternalIP()
}

// FulfillAddr 补全地址
func FulfillAddr(addr string) string {
	return innernet.FulfillAddr(addr)
}

// AssignRandPort 分配一个随机端口
func AssignRandPort(ip ...string) (int, error) {
	return innernet.AssignRandPort(ip...)
}

package packet

import (
	"net"
)

var globalPacker Packer

func init() {
	globalPacker = NewPacker()
}

// SetPacker 设置打包器
func SetPacker(packer Packer) {
	globalPacker = packer
}

// GetPacker 获取打包器
func GetPacker() Packer {
	return globalPacker
}

// Pack 打包消息
func Pack(message *Message) ([]byte, error) {
	return globalPacker.Pack(message)
}

// Unpack 解包消息
func Unpack(data []byte) (*Message, error) {
	return globalPacker.Unpack(data)
}

// Read 读取数据包
func Read(conn net.Conn) (isHeartbeat bool, buffer []byte, err error) {
	return globalPacker.Read(conn)
}

// Parse 解析数据包
func Parse(data []byte) (len int, route int32, buffer []byte, err error) {
	return globalPacker.Parse(data)
}

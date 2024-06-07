package packet

import "github.com/dobyte/due/v2/core/buffer"

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

// ReadMessage 读取消息
func ReadMessage(reader interface{}) ([]byte, error) {
	return globalPacker.ReadMessage(reader)
}

// PackBuffer 打包消息
func PackBuffer(message *Message) (buffer.Buffer, error) {
	return globalPacker.PackBuffer(message)
}

// PackMessage 打包消息
func PackMessage(message *Message) ([]byte, error) {
	return globalPacker.PackMessage(message)
}

// UnpackMessage 解包消息
func UnpackMessage(data []byte) (*Message, error) {
	return globalPacker.UnpackMessage(data)
}

// PackHeartbeat 打包心跳
func PackHeartbeat() ([]byte, error) {
	return globalPacker.PackHeartbeat()
}

// CheckHeartbeat 检测心跳包
func CheckHeartbeat(data []byte) (bool, error) {
	return globalPacker.CheckHeartbeat(data)
}

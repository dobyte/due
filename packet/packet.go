package packet

import "io"

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
func ReadMessage(reader io.Reader) ([]byte, error) {
	return globalPacker.ReadMessage(reader)
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

// IsNotNeedDeliverMsg 是否不需要传递的消息网关直接返回,比如心跳,握手等消息, return 是否不需要传递、消息内容
func IsNotNeedDeliverMsg(data []byte) (bool, []byte, error) {
	return globalPacker.IsNotNeedDeliverMsg(data)
}

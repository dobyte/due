package packet

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
func Pack1(message *Message) ([]byte, error) {
	return globalPacker.Pack1(message)
}

// Pack 打包消息
func Pack(message *Message) ([]byte, error) {
	return globalPacker.Pack(message)
}

// Unpack 解包消息
func Unpack1(data []byte) (*Message, error) {
	return globalPacker.Unpack1(data)
}

// Unpack 解包消息
func Unpack(data []byte) (*Message, error) {
	return globalPacker.Unpack(data)
}

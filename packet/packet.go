package packet

var defaultPacker = NewPacker()

// SetPacker 设置打包器
func SetPacker(packer Packer) {
	defaultPacker = packer
}

// GetPacker 获取打包器
func GetPacker() Packer {
	return defaultPacker
}

// Pack 打包消息
func Pack(message *Message) ([]byte, error) {
	return defaultPacker.Pack(message)
}

// Unpack 解包消息
func Unpack(data []byte) (*Message, error) {
	return defaultPacker.Unpack(data)
}

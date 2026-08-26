package packet

import (
	"io"

	"github.com/dobyte/due/v2/core/buffer"
)

var globalPacker Packer

// init 初始化全局打包器
func init() {
	globalPacker = NewPacker()
}

// SetPacker 设置打包器
// 覆盖全局打包器，用于替换默认的打包实现
// @param packer Packer 打包器
func SetPacker(packer Packer) {
	globalPacker = packer
}

// GetPacker 获取打包器
// @return @1 Packer 全局打包器
func GetPacker() Packer {
	return globalPacker
}

// ReadBuffer 以buffer的形式读取消息
// @param reader io.Reader 数据读取源
// @return @1 buffer.Buffer 读取到的消息缓冲区
// @return @2 error 读取失败时返回的错误
func ReadBuffer(reader io.Reader) (buffer.Buffer, error) {
	return globalPacker.ReadBuffer(reader)
}

// PackBuffer 以buffer的形式打包消息
// @param message *Message 待打包的消息
// @return @1 *buffer.NocopyBuffer 打包后的无拷贝缓冲区
// @return @2 error 打包失败时返回的错误
func PackBuffer(message *Message) (*buffer.NocopyBuffer, error) {
	return globalPacker.PackBuffer(message)
}

// ReadMessage 读取消息
// @param reader io.Reader 数据读取源
// @return @1 []byte 读取到的消息字节
// @return @2 error 读取失败时返回的错误
func ReadMessage(reader io.Reader) ([]byte, error) {
	return globalPacker.ReadMessage(reader)
}

// PackMessage 打包消息
// @param message *Message 待打包的消息
// @return @1 []byte 打包后的消息字节
// @return @2 error 打包失败时返回的错误
func PackMessage(message *Message) ([]byte, error) {
	return globalPacker.PackMessage(message)
}

// UnpackMessage 解包消息
// @param data []byte 待解包的原始消息字节
// @return @1 *Message 解包后的消息对象
// @return @2 error 解包失败时返回的错误
func UnpackMessage(data []byte) (*Message, error) {
	return globalPacker.UnpackMessage(data)
}

// PackHeartbeat 打包心跳
// @return @1 []byte 心跳包字节
// @return @2 error 打包失败时返回的错误
func PackHeartbeat() ([]byte, error) {
	return globalPacker.PackHeartbeat()
}

// CheckHeartbeat 检测心跳包
// @param data []byte 待检测的消息字节
// @return @1 bool 是否为心跳包
// @return @2 error 检测失败时返回的错误
func CheckHeartbeat(data []byte) (bool, error) {
	return globalPacker.CheckHeartbeat(data)
}

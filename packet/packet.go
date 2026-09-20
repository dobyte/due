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

// Read 以buffer的形式读取消息
// @param reader io.Reader 数据读取源
// @return @1 bool 是否为心跳消息
// @return @2 int64 服务器侧时间戳（纳秒）
// @return @3 buffer.Buffer 消息缓冲区
// @return @4 error 读取失败时返回的错误
func Read(reader io.Reader) (bool, int64, buffer.Buffer, error) {
	return globalPacker.Read(reader)
}

// PackMessage 以buffer的形式打包消息
// @param message *Message 消息
// @return @1 buffer.Buffer 打包后的消息缓冲区
// @return @2 error 打包失败时返回的错误
func PackMessage(message *Message) (buffer.Buffer, error) {
	return globalPacker.PackMessage(message)
}

// ExtractRouteSeq 从消息缓冲区中提取路由与序列号
// @param buf buffer.Buffer 消息缓冲区
// @return @1 int32 路由
// @return @2 int32 序列号
// @return @3 error 解包失败时返回的错误
func ExtractRouteSeq(buf buffer.Buffer) (int32, int32, error) {
	return globalPacker.ExtractRouteSeq(buf)
}

// UnpackMessage 解包消息
// @param buf buffer.Buffer 消息缓冲区
// @return @1 *Message 消息对象
// @return @2 error 解包失败时返回的错误
func UnpackMessage(buf buffer.Buffer) (int32, int32, buffer.Buffer, error) {
	return globalPacker.UnpackMessage(buf)
}

// PackHeartbeat 打包心跳
// 返回的心跳包缓冲区不可修改或释放
// @param server ...bool 是否为服务端心跳
// @return @1 buffer.Buffer 心跳包缓冲区
func PackHeartbeat(server ...bool) buffer.Buffer {
	return globalPacker.PackHeartbeat(server...)
}

package tcp

import (
	"github.com/cloudwego/netpoll"
)

const protocol = "tcp"

const (
	closeSig   int = iota // 关闭信号
	dataPacket            // 数据包
)

type chWrite struct {
	typ int
	msg []byte
}

// 执行写入操作
func write(writer netpoll.Writer, msg []byte) error {
	if _, err := writer.WriteBinary(msg); err != nil {
		return err
	}

	return writer.Flush()
}

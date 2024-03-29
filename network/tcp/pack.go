/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/5/12 10:58 下午
 * @Desc: TODO
 */

package tcp

import (
	"bufio"
	"bytes"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
)

const (
	msgLenBytes  uint16 = 2        // 消息长度字节数
	msgByteOrder string = "little" // 消息字节排序
)

var (
	errMsgSizeTooLarge = errors.New("the msg size too large")
)

// 打包消息
func pack(msg []byte) (packet []byte, err error) {
	var buf bytes.Buffer
	buf.Grow(len(msg) + int(msgLenBytes))

	if err = binary.Write(&buf, byteOrder(), uint16(len(msg))); err != nil {
		return
	}

	if err = binary.Write(&buf, byteOrder(), msg); err != nil {
		return
	}

	packet = buf.Bytes()

	return
}

// 解包消息
func unpack(packet []byte) (msg []byte, err error) {
	var (
		buf    = bytes.NewBuffer(packet)
		msgLen uint16
	)

	if err = binary.Read(buf, byteOrder(), &msgLen); err != nil {
		return
	}

	if msgLen > 0 {
		msg = make([]byte, msgLen)
		if err = binary.Read(buf, byteOrder(), &msg); err != nil {
			return
		}
	}

	return
}

// 读取连接数据
func readMsgFromConn(reader *bufio.Reader, maxMsgLength int) (msg []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()
	packet := make([]byte, msgLenBytes)
	if _, err = io.ReadFull(reader, packet); err != nil {
		return
	}

	var (
		buf    = bytes.NewBuffer(packet)
		msgLen uint16
	)

	if err = binary.Read(buf, byteOrder(), &msgLen); err != nil {
		return
	}

	if msgLen > 0 {
		msg = make([]byte, msgLen)
		if _, err = io.ReadFull(reader, msg); err != nil {
			return
		}

		if int(msgLen) > maxMsgLength {
			err = errMsgSizeTooLarge
		}
	}

	return
}

func byteOrder() binary.ByteOrder {
	switch msgByteOrder {
	case "little":
		return binary.LittleEndian
	default:
		return binary.BigEndian
	}
}

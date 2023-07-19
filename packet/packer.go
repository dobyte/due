package packet

import (
	"bytes"
	"encoding/binary"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
	"io"
	"net"
	"os"
	"reflect"
	"runtime"
	"strings"
)

var (
	ErrLenOverflow      = errors.New("len overflow")
	ErrSeqOverflow      = errors.New("seq overflow")
	ErrRouteOverflow    = errors.New("route overflow")
	ErrInvalidMessage   = errors.New("invalid message")
	ErrBufferTooLarge   = errors.New("buffer too large")
	ErrConnectionClosed = errors.New("connection is closed")
)

type Packer interface {
	// Pack 打包
	Pack(message *Message) ([]byte, error)
	// Unpack 解包
	Unpack(data []byte) (*Message, error)
	// Read 读取数据包
	Read(conn net.Conn) (len int, buffer []byte, err error)
	// Parse 解析数据包
	Parse(data []byte) (len int, route int32, buffer []byte, err error)
}

type defaultPacker struct {
	opts *options
}

func NewPacker(opts ...Option) Packer {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
	}

	if o.lenBytes != 1 && o.lenBytes != 2 && o.lenBytes != 4 && o.lenBytes != 8 {
		log.Fatalf("the number of len bytes must be 1、2、4、8, and give %d", o.lenBytes)
	}

	if o.routeBytes != 1 && o.routeBytes != 2 && o.routeBytes != 4 {
		log.Fatalf("the number of route bytes must be 1、2、4, and give %d", o.routeBytes)
	}

	if o.seqBytes != 0 && o.seqBytes != 1 && o.seqBytes != 2 && o.seqBytes != 4 {
		log.Fatalf("the number of seq bytes must be 0、1、2、4, and give %d", o.seqBytes)
	}

	if o.bufferBytes < 0 {
		log.Fatalf("the number of buffer bytes must be greater than or equal to 0, and give %d", o.bufferBytes)
	}

	maxLen := o.lenBytes + o.routeBytes + o.seqBytes + o.bufferBytes
	if maxLen > 1<<(8*o.lenBytes-1)-1 || maxLen < -1<<(8*o.lenBytes-1) {
		log.Fatalf("the number of bytes in the packet length cannot accommodate the entire packet data length")
	}

	return &defaultPacker{opts: o}
}

// Pack 打包消息
func (p *defaultPacker) Pack(message *Message) ([]byte, error) {
	if message == nil {
		return p.doPackHeartbeat()
	} else {
		return p.doPackMessage(message)
	}
}

// 打包心跳
func (p *defaultPacker) doPackHeartbeat() ([]byte, error) {
	var (
		err error
		buf = &bytes.Buffer{}
	)

	buf.Grow(p.opts.lenBytes)

	switch p.opts.lenBytes {
	case 1:
		err = binary.Write(buf, p.opts.byteOrder, int8(0))
	case 2:
		err = binary.Write(buf, p.opts.byteOrder, int16(0))
	case 4:
		err = binary.Write(buf, p.opts.byteOrder, int32(0))
	case 8:
		err = binary.Write(buf, p.opts.byteOrder, int64(0))
	}
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// 打包消息
func (p *defaultPacker) doPackMessage(message *Message) ([]byte, error) {
	if message.Route > int32(1<<(8*p.opts.routeBytes-1)-1) || message.Route < int32(-1<<(8*p.opts.routeBytes-1)) {
		return nil, ErrRouteOverflow
	}

	if p.opts.seqBytes > 0 {
		if message.Seq > int32(1<<(8*p.opts.seqBytes-1)-1) || message.Seq < int32(-1<<(8*p.opts.seqBytes-1)) {
			return nil, ErrSeqOverflow
		}
	}

	if length(message.Buffer) > p.opts.bufferBytes {
		return nil, ErrBufferTooLarge
	}

	ln := p.opts.lenBytes + p.opts.routeBytes + p.opts.routeBytes + len(message.Buffer)

	if ln > 1<<(8*p.opts.lenBytes-1)-1 || ln < -1<<(8*p.opts.lenBytes-1) {
		return nil, ErrLenOverflow
	}

	var (
		err error
		buf = &bytes.Buffer{}
	)

	buf.Grow(ln)

	switch p.opts.lenBytes {
	case 1:
		err = binary.Write(buf, p.opts.byteOrder, int8(ln))
	case 2:
		err = binary.Write(buf, p.opts.byteOrder, int16(ln))
	case 4:
		err = binary.Write(buf, p.opts.byteOrder, int32(ln))
	case 8:
		err = binary.Write(buf, p.opts.byteOrder, int64(ln))
	}
	if err != nil {
		return nil, err
	}

	switch p.opts.routeBytes {
	case 1:
		err = binary.Write(buf, p.opts.byteOrder, int8(message.Route))
	case 2:
		err = binary.Write(buf, p.opts.byteOrder, int16(message.Route))
	case 4:
		err = binary.Write(buf, p.opts.byteOrder, message.Route)
	}
	if err != nil {
		return nil, err
	}

	switch p.opts.seqBytes {
	case 1:
		err = binary.Write(buf, p.opts.byteOrder, int8(message.Seq))
	case 2:
		err = binary.Write(buf, p.opts.byteOrder, int16(message.Seq))
	case 4:
		err = binary.Write(buf, p.opts.byteOrder, message.Seq)
	}
	if err != nil {
		return nil, err
	}

	err = binary.Write(buf, p.opts.byteOrder, message.Buffer)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// Unpack 解包消息
func (p *defaultPacker) Unpack(data []byte) (*Message, error) {
	var (
		err    error
		ln     int64
		reader = bytes.NewReader(data)
	)

	switch p.opts.lenBytes {
	case 1:
		var l int8
		if err = binary.Read(reader, p.opts.byteOrder, &l); err != nil {
			return nil, err
		}
		ln = int64(l)
	case 2:
		var l int16
		if err = binary.Read(reader, p.opts.byteOrder, &l); err != nil {
			return nil, err
		}
		ln = int64(l)
	case 4:
		var l int32
		if err = binary.Read(reader, p.opts.byteOrder, &l); err != nil {
			return nil, err
		}
		ln = int64(l)
	case 8:
		if err = binary.Read(reader, p.opts.byteOrder, &ln); err != nil {
			return nil, err
		}
	}

	bufLen := ln - int64(p.opts.lenBytes+p.opts.routeBytes+p.opts.seqBytes)
	if bufLen < 0 {
		return nil, ErrInvalidMessage
	}

	message := &Message{Buffer: make([]byte, bufLen)}

	switch p.opts.routeBytes {
	case 1:
		var route int8
		if err = binary.Read(reader, p.opts.byteOrder, &route); err != nil {
			return nil, err
		} else {
			message.Route = int32(route)
		}
	case 2:
		var route int16
		if err = binary.Read(reader, p.opts.byteOrder, &route); err != nil {
			return nil, err
		} else {
			message.Route = int32(route)
		}
	case 4:
		var route int32
		if err = binary.Read(reader, p.opts.byteOrder, &route); err != nil {
			return nil, err
		} else {
			message.Route = route
		}
	}

	switch p.opts.seqBytes {
	case 1:
		var seq int8
		if err = binary.Read(reader, p.opts.byteOrder, &seq); err != nil {
			return nil, err
		} else {
			message.Seq = int32(seq)
		}
	case 2:
		var seq int16
		if err = binary.Read(reader, p.opts.byteOrder, &seq); err != nil {
			return nil, err
		} else {
			message.Seq = int32(seq)
		}
	case 4:
		var seq int32
		if err = binary.Read(reader, p.opts.byteOrder, &seq); err != nil {
			return nil, err
		} else {
			message.Seq = seq
		}
	}

	err = binary.Read(reader, p.opts.byteOrder, &message.Buffer)
	if err != nil {
		return nil, err
	}

	return message, nil
}

// Read 读取数据包
func (p *defaultPacker) Read(conn net.Conn) (len int, buffer []byte, err error) {
	lenBuf := make([]byte, p.opts.lenBytes)
	if _, err = io.ReadFull(conn, lenBuf); err != nil {
		if isClosedConnError(err) {
			err = ErrConnectionClosed
		}
		return
	}

	buf := bytes.NewBuffer(lenBuf)
	switch p.opts.lenBytes {
	case 1:
		var l int8
		if err = binary.Read(buf, p.opts.byteOrder, &l); err != nil {
			return
		}
		len = int(l)
	case 2:
		var l int16
		if err = binary.Read(buf, p.opts.byteOrder, &l); err != nil {
			return
		}
		len = int(l)
	case 4:
		var l int32
		if err = binary.Read(buf, p.opts.byteOrder, &l); err != nil {
			return
		}
		len = int(l)
	case 8:
		var l int64
		if err = binary.Read(buf, p.opts.byteOrder, &l); err != nil {
			return
		}
		len = int(l)
	}

	if len == 0 {
		return
	}

	if len < p.opts.lenBytes+p.opts.routeBytes+p.opts.seqBytes {
		len, err = 0, ErrInvalidMessage
		return
	}

	buffer = make([]byte, len)
	copy(buffer, lenBuf)
	_, err = io.ReadFull(conn, buffer[p.opts.lenBytes:])

	return
}

// Parse 解析数据包
func (p *defaultPacker) Parse(data []byte) (len int, route int32, buffer []byte, err error) {
	if length(data) == 0 {
		err = ErrInvalidMessage
		return
	}

	buf := bytes.NewBuffer(data)
	switch p.opts.lenBytes {
	case 1:
		var l int8
		if err = binary.Read(buf, p.opts.byteOrder, &l); err != nil {
			return
		}
		len = int(l)
	case 2:
		var l int16
		if err = binary.Read(buf, p.opts.byteOrder, &l); err != nil {
			return
		}
		len = int(l)
	case 4:
		var l int32
		if err = binary.Read(buf, p.opts.byteOrder, &l); err != nil {
			return
		}
		len = int(l)
	case 8:
		var l int64
		if err = binary.Read(buf, p.opts.byteOrder, &l); err != nil {
			return
		}
		len = int(l)
	}

	if len == 0 {
		return
	}

	if len < p.opts.lenBytes+p.opts.routeBytes+p.opts.seqBytes {
		len, err = 0, ErrInvalidMessage
		return
	}

	buf = bytes.NewBuffer(data[p.opts.lenBytes : p.opts.lenBytes+p.opts.routeBytes])
	switch p.opts.routeBytes {
	case 1:
		var r int8
		if err = binary.Read(buf, p.opts.byteOrder, &r); err != nil {
			return
		}
		route = int32(r)
	case 2:
		var r int16
		if err = binary.Read(buf, p.opts.byteOrder, &r); err != nil {
			return
		}
		route = int32(r)
	case 4:
		if err = binary.Read(buf, p.opts.byteOrder, &route); err != nil {
			return
		}
	}

	buffer = data[:len]

	return
}

func length(buf []byte) int {
	return len(buf)
}

func isClosedConnError(err error) bool {
	if err == nil {
		return false
	}

	if err == io.EOF || err == io.ErrUnexpectedEOF {
		return true
	}

	if strings.Contains(err.Error(), "use of closed network connection") {
		return true
	}

	if runtime.GOOS == "windows" {
		if oe, ok := err.(*net.OpError); ok && oe.Op == "read" {
			if se, ok := oe.Err.(*os.SyscallError); ok && se.Syscall == "wsarecv" {
				const WSAECONNABORTED = 10053
				const WSAECONNRESET = 10054
				if n := errno(se.Err); n == WSAECONNRESET || n == WSAECONNABORTED {
					return true
				}
			}
		}
	}

	return false
}

func errno(v error) uintptr {
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Uintptr {
		return uintptr(rv.Uint())
	}

	return 0
}

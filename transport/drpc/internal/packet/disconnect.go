package packet

import (
	"bytes"
	"encoding/binary"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/session"
	"io"
	"sync"
)

const (
	disconnectReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + 1 + 8 + 1
	disconnectResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes + 4
)

type DisconnectPacker struct {
	reqPool *sync.Pool
	resPool *sync.Pool
}

func NewDisconnectPacker() *DisconnectPacker {
	p := &DisconnectPacker{}
	p.reqPool = &sync.Pool{}
	p.reqPool.New = func() any { return NewBuffer(p.reqPool, disconnectReqBytes) }
	p.resPool = &sync.Pool{}
	p.resPool.New = func() any { return NewBuffer(p.resPool, disconnectResBytes) }

	return p
}

// PackReq 打包请求
// 协议格式：size + header + route + seq + session kind + target
func (p *DisconnectPacker) PackReq(seq uint64, kind session.Kind, target int64, isForce bool) (buf *Buffer, err error) {
	buf = p.reqPool.Get().(*Buffer)
	defer func() {
		if err != nil {
			buf.Recycle()
		}
	}()

	size := disconnectReqBytes - defaultSizeBytes

	if err = binary.Write(buf, binary.BigEndian, int32(size)); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, dataBit); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, disconnectReq); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, seq); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, int8(kind)); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, target); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, isForce); err != nil {
		return
	}

	return
}

// UnpackReq 解包请求
// 协议格式：size + header + route + seq + session kind + target
func (p *DisconnectPacker) UnpackReq(data []byte) (seq uint64, kind session.Kind, target int64, isForce bool, err error) {
	if len(data) != disconnectReqBytes {
		err = errors.ErrInvalidMessage
		return
	}

	reader := bytes.NewReader(data)

	if _, err = reader.Seek(defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes, io.SeekStart); err != nil {
		return
	}

	if err = binary.Read(reader, binary.BigEndian, &seq); err != nil {
		return
	}

	var k int8

	if err = binary.Read(reader, binary.BigEndian, &k); err != nil {
		return
	}

	if err = binary.Read(reader, binary.BigEndian, &target); err != nil {
		return
	}

	if err = binary.Read(reader, binary.BigEndian, &isForce); err != nil {
		return
	}

	kind = session.Kind(k)

	return
}

// PackRes 打包响应
// size + header + route + seq + code
func (p *DisconnectPacker) PackRes(seq uint64, code int16) (buf *Buffer, err error) {
	buf = p.resPool.Get().(*Buffer)
	defer func() {
		if err != nil {
			buf.Recycle()
		}
	}()

	size := disconnectResBytes - defaultSizeBytes

	if err = binary.Write(buf, binary.BigEndian, int32(size)); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, dataBit); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, disconnectRes); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, seq); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, code); err != nil {
		return
	}

	return
}

// UnpackRes 解包响应
// size + header + route + seq + code
func (p *DisconnectPacker) UnpackRes(data []byte) (code int16, err error) {
	if len(data) != disconnectResBytes {
		err = errors.ErrInvalidMessage
		return
	}

	reader := bytes.NewReader(data)

	if _, err = reader.Seek(defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes+defaultSeqBytes, io.SeekStart); err != nil {
		return
	}

	if err = binary.Read(reader, binary.BigEndian, &code); err != nil {
		return
	}

	return
}

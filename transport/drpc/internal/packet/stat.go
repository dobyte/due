package packet

import (
	"bytes"
	"encoding/binary"
	"github.com/dobyte/due/v2/session"
	"io"
	"sync"
)

const (
	statReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + 1
	statResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + 8
)

type StatPacker struct {
	reqPool *sync.Pool
	resPool *sync.Pool
}

func NewStatPacker() *StatPacker {
	p := &StatPacker{}
	p.reqPool = &sync.Pool{}
	p.reqPool.New = func() any { return NewBuffer(p.reqPool, statReqBytes) }
	p.resPool = &sync.Pool{}
	p.resPool.New = func() any { return NewBuffer(p.resPool, statResBytes) }

	return p
}

// PackReq 打包请求
// 协议格式：size + header + route + seq + session kind
func (p *StatPacker) PackReq(seq uint64, kind session.Kind) (buf *Buffer, err error) {
	buf = p.reqPool.Get().(*Buffer)
	defer func() {
		if err != nil {
			buf.Recycle()
		}
	}()

	size := statReqBytes - defaultSizeBytes

	if err = binary.Write(buf, binary.BigEndian, int32(size)); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, dataBit); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, statReq); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, seq); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, int8(kind)); err != nil {
		return
	}

	return
}

// UnpackReq 解包请求
// 协议格式：size + header + route + seq + session kind + target
func (p *StatPacker) UnpackReq(data []byte) (seq uint64, kind session.Kind, err error) {
	if len(data) != statReqBytes {
		err = ErrInvalidPacket
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

	kind = session.Kind(k)

	return
}

// PackRes 打包响应
// size + header + route + seq + [total]
func (p *StatPacker) PackRes(seq uint64, total ...int64) (buf *Buffer, err error) {
	buf = p.resPool.Get().(*Buffer)
	defer func() {
		if err != nil {
			buf.Recycle()
		}
	}()

	size := statResBytes - defaultSizeBytes

	if len(total) == 0 || total[0] == 0 {
		size -= 8
	}

	if err = binary.Write(buf, binary.BigEndian, int32(size)); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, dataBit); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, statRes); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, seq); err != nil {
		return
	}

	if len(total) > 0 && total[0] != 0 {
		if err = binary.Write(buf, binary.BigEndian, total[0]); err != nil {
			return
		}
	}

	return
}

// UnpackRes 解包响应
// size + header + route + seq + [total]
func (p *StatPacker) UnpackRes(data []byte) (total int64, err error) {
	if len(data) != statResBytes && len(data) != statResBytes-8 {
		err = ErrInvalidPacket
		return
	}

	reader := bytes.NewReader(data)

	if _, err = reader.Seek(defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes+defaultSeqBytes, io.SeekStart); err != nil {
		return
	}

	if reader.Len() > 0 {
		err = binary.Read(reader, binary.BigEndian, &total)
	}

	return
}

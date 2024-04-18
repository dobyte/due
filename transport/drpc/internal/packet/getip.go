package packet

import (
	"bytes"
	"encoding/binary"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/transport/drpc/internal/codes"
	"github.com/dobyte/due/v2/utils/xnet"
	"io"
	"sync"
)

const (
	getIPReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + 1 + 8
	getIPResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes + 4
)

type GetIPPacker struct {
	reqPool *sync.Pool
	resPool *sync.Pool
}

func NewGetIPPacker() *GetIPPacker {
	p := &GetIPPacker{}
	p.reqPool = &sync.Pool{}
	p.reqPool.New = func() any { return NewBuffer(p.reqPool, getIPReqBytes) }
	p.resPool = &sync.Pool{}
	p.resPool.New = func() any { return NewBuffer(p.resPool, getIPResBytes) }

	return p
}

// PackReq 打包请求
// 协议格式：size + header + route + seq + session kind + target
func (p *GetIPPacker) PackReq(seq uint64, kind session.Kind, target int64) (buf *Buffer, err error) {
	buf = p.reqPool.Get().(*Buffer)
	defer func() {
		if err != nil {
			buf.Recycle()
		}
	}()

	size := getIPReqBytes - defaultSizeBytes

	if err = binary.Write(buf, binary.BigEndian, int32(size)); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, dataBit); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, getIPReq); err != nil {
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

	return
}

// UnpackReq 解包请求
// 协议格式：size + header + route + seq + session kind + target
func (p *GetIPPacker) UnpackReq(data []byte) (seq uint64, kind session.Kind, target int64, err error) {
	if len(data) != getIPReqBytes {
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

	kind = session.Kind(k)

	return
}

// PackRes 打包响应
// size + header + route + seq + code + [ip]
func (p *GetIPPacker) PackRes(seq uint64, code int16, ip ...string) (buf *Buffer, err error) {
	buf = p.resPool.Get().(*Buffer)
	defer func() {
		if err != nil {
			buf.Recycle()
		}
	}()

	size := bindResBytes - defaultSizeBytes

	if code != codes.OK || len(ip) == 0 || ip[0] == "" {
		size -= 4
	}

	if err = binary.Write(buf, binary.BigEndian, int32(size)); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, dataBit); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, getIPRes); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, seq); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, code); err != nil {
		return
	}

	if code == codes.OK && len(ip) > 0 && ip[0] != "" {
		if err = binary.Write(buf, binary.BigEndian, xnet.IP2Long(ip[0])); err != nil {
			return
		}
	}

	return
}

// UnpackRes 解包响应
// size + header + route + seq + code + [ip]
func (p *GetIPPacker) UnpackRes(data []byte) (code int16, ip string, err error) {
	if len(data) != getIPResBytes && len(data) != getIPResBytes-4 {
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

	if code == codes.OK && reader.Len() > 0 {
		var v uint32

		if err = binary.Read(reader, binary.BigEndian, &v); err != nil {
			return
		}

		ip = xnet.Long2IP(v)
	}

	return
}

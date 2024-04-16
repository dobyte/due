package packet

import (
	"encoding/binary"
	"github.com/dobyte/due/v2/session"
	"sync"
)

type PushPacker struct {
	reqPool *sync.Pool
	resPool *sync.Pool
}

func NewPushPacker() *PushPacker {
	p := &PushPacker{}
	p.reqPool = &sync.Pool{}
	p.reqPool.New = func() any { return NewBuffer(p.reqPool, getIPReqBytes) }
	p.resPool = &sync.Pool{}
	p.resPool.New = func() any { return NewBuffer(p.resPool, getIPResBytes) }

	return p
}

// PackReq 打包请求
// 协议格式：size + header + route + seq + session kind + target
func (p *PushPacker) PackReq(seq uint64, kind session.Kind, target int64) (buf *Buffer, err error) {
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

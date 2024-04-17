package packet

import (
	"bytes"
	"encoding/binary"
	packets "github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/session"
	"io"
	"sync"
)

const (
	pushReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + 1 + 8 + 4 + 4
	pushResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes + 4
)

type PushPacker struct {
	reqPool *sync.Pool
	resPool *sync.Pool
}

func NewPushPacker() *PushPacker {
	p := &PushPacker{}
	p.reqPool = &sync.Pool{}
	p.reqPool.New = func() any { return NewBuffer(p.reqPool, pushReqBytes) }
	p.resPool = &sync.Pool{}
	p.resPool.New = func() any { return NewBuffer(p.resPool, pushResBytes) }

	return p
}

// PackReq 打包请求（分包发送，规避一次不定长数据拷贝过程）
// 协议格式：size + header + route + seq + session kind + target + client route + client seq
func (p *PushPacker) PackReq(seq uint64, kind session.Kind, target int64, message *packets.Message) (buf *Buffer, err error) {
	buf = p.reqPool.Get().(*Buffer)
	defer func() {
		if err != nil {
			buf.Recycle()
		}
	}()

	size := pushReqBytes - defaultSizeBytes + len(message.Buffer)

	if err = binary.Write(buf, binary.BigEndian, int32(size)); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, dataBit); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, pushReq); err != nil {
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

	if err = binary.Write(buf, binary.BigEndian, message.Route); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, message.Seq); err != nil {
		return
	}

	return
}

// UnpackReq 解包请求
// 协议格式：size + header + route + seq + session kind + target + client route + client seq + client data
func (p *PushPacker) UnpackReq(data []byte) (seq uint64, kind session.Kind, target int64, message *packets.Message, err error) {
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

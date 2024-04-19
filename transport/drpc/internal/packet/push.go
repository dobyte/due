package packet

import (
	"bytes"
	"encoding/binary"
	"github.com/dobyte/due/v2/errors"
	packets "github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/transport/drpc/internal/route"
	"io"
	"sync"
)

const (
	pushReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + 1 + 8 + 4 + 4
	pushResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes
)

type PushPacker struct {
	reqPool *sync.Pool
	resPool *sync.Pool

	reqPool2 *sync.Pool
	resPool2 *sync.Pool
}

func NewPushPacker() *PushPacker {
	p := &PushPacker{}
	p.reqPool = &sync.Pool{}
	p.reqPool.New = func() any { return NewBuffer(p.reqPool, pushReqBytes) }
	p.resPool = &sync.Pool{}
	p.resPool.New = func() any { return NewBuffer(p.resPool, pushResBytes) }

	p.reqPool2 = &sync.Pool{}
	p.reqPool2.New = func() any { return NewWriter(p.reqPool2, pushReqBytes) }
	p.resPool2 = &sync.Pool{}
	p.resPool2.New = func() any { return NewWriter(p.resPool2, pushResBytes) }

	return p
}

// PackReq 打包请求（分包发送，规避一次不定长数据拷贝过程）
// 协议格式：size + header + route + seq + session kind + target + client route + client seq
func (p *PushPacker) PackReq2(seq uint64, kind session.Kind, target int64, message *packets.Message) (buf *Buffer, err error) {
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

	if err = binary.Write(buf, binary.BigEndian, route.Push); err != nil {
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

func (p *PushPacker) PackReq(seq uint64, kind session.Kind, target int64, message *packets.Message) (buf *Writer, err error) {
	buf = p.reqPool2.Get().(*Writer)
	defer func() {
		if err != nil {
			buf.Recycle()
		}
	}()

	size := pushReqBytes - defaultSizeBytes + len(message.Buffer)

	buf.WriteInt32s(binary.BigEndian, int32(size))
	buf.WriteUint8s(dataBit)
	buf.WriteInt8s(route.Push)
	buf.WriteUint64s(binary.BigEndian, seq)
	buf.WriteInt8s(int8(kind))
	buf.WriteInt64s(binary.BigEndian, target)
	buf.WriteInt32s(binary.BigEndian, message.Route)
	buf.WriteInt32s(binary.BigEndian, message.Seq)

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

	kind = session.Kind(k)

	if err = binary.Read(reader, binary.BigEndian, &target); err != nil {
		return
	}

	message = &packets.Message{}

	if err = binary.Read(reader, binary.BigEndian, &message.Route); err != nil {
		return
	}

	if err = binary.Read(reader, binary.BigEndian, &message.Seq); err != nil {
		return
	}

	message.Buffer = data[pushReqBytes:]

	return
}

// PackRes 打包响应
// size + header + route + seq + code
func (p *PushPacker) PackRes2(seq uint64, code int16) (buf *Buffer, err error) {
	buf = p.resPool.Get().(*Buffer)
	defer func() {
		if err != nil {
			buf.Recycle()
		}
	}()

	size := pushResBytes - defaultSizeBytes

	if err = binary.Write(buf, binary.BigEndian, int32(size)); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, dataBit); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, route.Push); err != nil {
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

func (p *PushPacker) PackRes(seq uint64, code int16) (buf *Writer, err error) {
	buf = p.resPool2.Get().(*Writer)
	defer func() {
		if err != nil {
			buf.Recycle()
		}
	}()

	size := pushResBytes - defaultSizeBytes

	buf.WriteInt32s(binary.BigEndian, int32(size))
	buf.WriteUint8s(dataBit)
	buf.WriteInt8s(route.Push)
	buf.WriteUint64s(binary.BigEndian, seq)
	buf.WriteInt16s(binary.BigEndian, code)

	return
}

// UnpackRes 解包响应
// size + header + route + seq + code
func (p *PushPacker) UnpackRes(data []byte) (code int16, err error) {
	if len(data) != pushResBytes {
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

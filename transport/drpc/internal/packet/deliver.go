package packet

import (
	"bytes"
	"encoding/binary"
	"github.com/dobyte/due/v2/errors"
	packets "github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/transport/drpc/internal/route"
	"io"
	"sync"
)

const (
	deliverReqBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + 36 + 8 + 8 + 4 + 4
	deliverResBytes = defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes + defaultCodeBytes
)

type DeliverPacker struct {
	reqPool  *sync.Pool
	resPool  *sync.Pool
	reqPool2 *sync.Pool
	resPool2 *sync.Pool
}

func NewDeliverPacker() *DeliverPacker {
	p := &DeliverPacker{}
	p.reqPool = &sync.Pool{}
	p.reqPool.New = func() any { return NewBuffer(p.reqPool, deliverReqBytes) }
	p.resPool = &sync.Pool{}
	p.resPool.New = func() any { return NewBuffer(p.resPool, deliverResBytes) }

	p.reqPool2 = &sync.Pool{}
	p.reqPool2.New = func() any { return NewWriter(p.reqPool2, deliverReqBytes) }
	p.resPool2 = &sync.Pool{}
	p.resPool2.New = func() any { return NewWriter(p.resPool2, deliverResBytes) }

	return p
}

// PackReq 打包请求（分包发送，规避一次不定长数据拷贝过程）
// 协议格式：size + header + route + seq + gid + cid + uid + client route + client seq
func (p *DeliverPacker) PackReq2(seq uint64, gid string, cid, uid int64, message *packets.Message) (buf *Buffer, err error) {
	buf = p.reqPool.Get().(*Buffer)
	defer func() {
		if err != nil {
			buf.Recycle()
		}
	}()

	size := deliverReqBytes - defaultSizeBytes + len(message.Buffer)

	if err = binary.Write(buf, binary.BigEndian, int32(size)); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, dataBit); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, route.Deliver); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, seq); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, []byte(gid)); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, cid); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, uid); err != nil {
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

func (p *DeliverPacker) PackReq(seq uint64, gid string, cid, uid int64, message *packets.Message) (writer *Writer, err error) {
	writer = p.reqPool2.Get().(*Writer)
	defer func() {
		if err != nil {
			writer.Recycle()
		}
	}()

	size := deliverReqBytes - defaultSizeBytes + len(message.Buffer)

	writer.WriteInt32s(binary.BigEndian, int32(size))
	writer.WriteUint8s(dataBit)
	writer.WriteInt8s(route.Deliver)
	writer.WriteUint64s(binary.BigEndian, seq)
	writer.WriteString(gid)
	writer.WriteInt64s(binary.BigEndian, cid)
	writer.WriteInt64s(binary.BigEndian, uid)
	writer.WriteInt32s(binary.BigEndian, message.Route)
	writer.WriteInt32s(binary.BigEndian, message.Seq)

	return
}

// UnpackReq 解包请求
// 协议格式：size + header + route + seq + gid + cid + uid + client route + client seq + client data
func (p *DeliverPacker) UnpackReq(data []byte) (seq uint64, gid string, cid, uid int64, message *packets.Message, err error) {
	reader := bytes.NewReader(data)

	if _, err = reader.Seek(defaultSizeBytes+defaultHeaderBytes+defaultRouteBytes, io.SeekStart); err != nil {
		return
	}

	if err = binary.Read(reader, binary.BigEndian, &seq); err != nil {
		return
	}

	from := defaultSizeBytes + defaultHeaderBytes + defaultRouteBytes + defaultSeqBytes
	to := from + 36
	gid = string(data[from:to])

	reader.Seek(36, io.SeekCurrent)

	if err = binary.Read(reader, binary.BigEndian, &cid); err != nil {
		return
	}

	if err = binary.Read(reader, binary.BigEndian, &uid); err != nil {
		return
	}

	message = &packets.Message{}

	if err = binary.Read(reader, binary.BigEndian, &message.Route); err != nil {
		return
	}

	if err = binary.Read(reader, binary.BigEndian, &message.Seq); err != nil {
		return
	}

	message.Buffer = data[deliverReqBytes:]

	return
}

// PackRes 打包响应
// size + header + route + seq + code
func (p *DeliverPacker) PackRes2(seq uint64, code int16) (buf *Buffer, err error) {
	buf = p.resPool.Get().(*Buffer)
	defer func() {
		if err != nil {
			buf.Recycle()
		}
	}()

	size := deliverResBytes - defaultSizeBytes

	if err = binary.Write(buf, binary.BigEndian, int32(size)); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, dataBit); err != nil {
		return
	}

	if err = binary.Write(buf, binary.BigEndian, route.Deliver); err != nil {
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

// PackRes 打包响应
// size + header + route + seq + code
func (p *DeliverPacker) PackRes(seq uint64, code int16) (buf *Writer, err error) {
	buf = p.resPool2.Get().(*Writer)
	defer func() {
		if err != nil {
			buf.Recycle()
		}
	}()

	size := deliverResBytes - defaultSizeBytes

	buf.WriteInt32s(binary.BigEndian, int32(size))
	buf.WriteUint8s(dataBit)
	buf.WriteInt8s(route.Deliver)
	buf.WriteUint64s(binary.BigEndian, seq)
	buf.WriteInt16s(binary.BigEndian, code)

	return
}

// UnpackRes 解包响应
// size + header + route + seq + code
func (p *DeliverPacker) UnpackRes(data []byte) (code int16, err error) {
	if len(data) != deliverResBytes {
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

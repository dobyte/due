package packet

import (
	"bufio"
	"io"
	"time"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/log"
)

const (
	dataBit      = 0 << 7 // Data frame flag
	handshakeBit = 1 << 0 // Handshake frame flag
	heartbeatBit = 1 << 7 // Heartbeat frame flag
)

// FrameType is the type of a frame read from the wire.
type FrameType uint8

const (
	// FrameData is a business data frame.
	FrameData FrameType = iota
	// FrameHeartbeat is a heartbeat frame.
	FrameHeartbeat
	// FrameHandshake is a handshake frame.
	FrameHandshake
)

// Packer defines the encoding and decoding capabilities of messages.
type Packer interface {
	// Read reads a frame as a buffer. It reports the frame type, the server-side timestamp in
	// nanoseconds, the frame buffer and any read error. The timestamp is meaningful only for
	// heartbeat frames.
	Read(reader io.Reader) (FrameType, int64, buffer.Buffer, error)
	// PackHandshake packs a handshake frame. A handshake frame carries neither a route nor a
	// sequence number, and it is marked by a reserved header bit.
	PackHandshake(payload []byte) (buffer.Buffer, error)
	// PackHeartbeat packs a heartbeat. Pass server as true to pack a server-side heartbeat.
	PackHeartbeat(server ...bool) buffer.Buffer
	// PackMessage packs message as a buffer and returns the packed buffer or a packing error.
	PackMessage(message *Message) (buffer.Buffer, error)
	// UnpackMessage unpacks the message buffer buf. It returns the route, the sequence number,
	// the message buffer and any unpacking error.
	UnpackMessage(buf buffer.Buffer) (int32, int32, buffer.Buffer, error)
	// ExtractHandshake extracts the payload of a handshake frame. It reports
	// [errors.ErrInvalidMessage] when buf is not a handshake frame.
	ExtractHandshake(buf buffer.Buffer) ([]byte, error)
	// ExtractRouteSeq extracts the route and sequence number from the message buffer buf. It
	// returns the route, the sequence number and any unpacking error.
	ExtractRouteSeq(buf buffer.Buffer) (int32, int32, error)
}

// defaultPacker is the default packer.
type defaultPacker struct {
	opts      *options      // Packing options
	heartbeat buffer.Buffer // Prebuilt heartbeat packet
}

var _ Packer = (*defaultPacker)(nil)

// NewPacker returns a new default packer.
//
// It validates the options and prebuilds the heartbeat packet. The program terminates when the
// given options are invalid.
func NewPacker(opts ...Option) *defaultPacker {
	o := defaultOptions()
	for _, opt := range opts {
		opt(o)
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

	p := &defaultPacker{}
	p.opts = o
	p.init()

	return p
}

// Read reads a frame as a buffer.
//
// It takes the zero-allocation fast path for *bufio.Reader and falls back to pooled header-buffer
// reading for other sources.
func (p *defaultPacker) Read(reader io.Reader) (FrameType, int64, buffer.Buffer, error) {
	if r, ok := reader.(*bufio.Reader); ok {
		return p.readBuffered(r)
	}

	return p.readPooled(reader)
}

// readBuffered reads from r without allocating.
//
// It parses the message header directly from the read buffer through Peek/Discard, performs a
// single pooled allocation of the message body for data messages, and allocates nothing for
// heartbeat messages.
func (p *defaultPacker) readBuffered(r *bufio.Reader) (FrameType, int64, buffer.Buffer, error) {
	head, err := r.Peek(defaultSizeBytes + defaultHeaderBytes)
	if err != nil {
		return FrameData, 0, nil, err
	}

	header := head[defaultSizeBytes]

	if header&heartbeatBit == heartbeatBit {
		switch size := p.opts.byteOrder.Uint32(head[:defaultSizeBytes]); size {
		case defaultHeaderBytes:
			_, _ = r.Discard(defaultSizeBytes + defaultHeaderBytes)
			return FrameHeartbeat, 0, nil, nil
		case defaultHeaderBytes + defaultHeartbeatTimeBytes:
			data, err := r.Peek(defaultSizeBytes + defaultHeaderBytes + defaultHeartbeatTimeBytes)
			if err != nil {
				return FrameData, 0, nil, err
			}

			heartbeatTime := int64(p.opts.byteOrder.Uint64(data[defaultSizeBytes+defaultHeaderBytes:]))

			_, _ = r.Discard(defaultSizeBytes + defaultHeaderBytes + defaultHeartbeatTimeBytes)

			return FrameHeartbeat, heartbeatTime, nil, nil
		default:
			return FrameData, 0, nil, errors.ErrInvalidMessage
		}
	}

	size := int64(p.opts.byteOrder.Uint32(head[:defaultSizeBytes]))

	if size <= 0 {
		return FrameData, 0, nil, errors.ErrInvalidMessage
	}

	if size > int64(defaultHeaderBytes+p.opts.routeBytes+p.opts.seqBytes+p.opts.bufferBytes) {
		return FrameData, 0, nil, errors.ErrMessageTooLarge
	}

	size -= defaultHeaderBytes

	buf := buffer.MallocBytes(int(defaultSizeBytes + defaultHeaderBytes + size))

	if buf == nil {
		return FrameData, 0, nil, errors.ErrMessageTooLarge
	}

	data := buf.Bytes()

	copy(data[:defaultSizeBytes+defaultHeaderBytes], head)

	_, _ = r.Discard(defaultSizeBytes + defaultHeaderBytes)

	if _, err = io.ReadFull(r, data[defaultSizeBytes+defaultHeaderBytes:]); err != nil {
		buf.Release()
		return FrameData, 0, nil, err
	}

	return frameType(header), 0, buf, nil
}

// readPooled reads through a pooled header buffer.
//
// The header is read from the byte pool and explicitly released; data messages additionally
// perform one pooled allocation of the message body and a header copy.
func (p *defaultPacker) readPooled(reader io.Reader) (FrameType, int64, buffer.Buffer, error) {
	buf1 := buffer.MallocBytes(defaultSizeBytes + defaultHeaderBytes)

	if buf1 == nil {
		return FrameData, 0, nil, errors.ErrMessageTooLarge
	}

	data1 := buf1.Bytes()

	if _, err := io.ReadFull(reader, data1); err != nil {
		buf1.Release()
		return FrameData, 0, nil, err
	}

	header := data1[defaultSizeBytes]

	if header&heartbeatBit == heartbeatBit {
		switch size := p.opts.byteOrder.Uint32(data1[:defaultSizeBytes]); size {
		case defaultHeaderBytes:
			buf1.Release()
			return FrameHeartbeat, 0, nil, nil
		case defaultHeaderBytes + defaultHeartbeatTimeBytes:
			var data [defaultHeartbeatTimeBytes]byte

			if _, err := io.ReadFull(reader, data[:]); err != nil {
				buf1.Release()
				return FrameData, 0, nil, err
			}

			heartbeatTime := int64(p.opts.byteOrder.Uint64(data[:]))

			buf1.Release()

			return FrameHeartbeat, heartbeatTime, nil, nil
		default:
			buf1.Release()
			return FrameData, 0, nil, errors.ErrInvalidMessage
		}
	}

	size := int64(p.opts.byteOrder.Uint32(data1[:defaultSizeBytes]))

	if size <= 0 {
		buf1.Release()
		return FrameData, 0, nil, errors.ErrInvalidMessage
	}

	if size > int64(defaultHeaderBytes+p.opts.routeBytes+p.opts.seqBytes+p.opts.bufferBytes) {
		buf1.Release()
		return FrameData, 0, nil, errors.ErrMessageTooLarge
	}

	size -= defaultHeaderBytes

	buf2 := buffer.MallocBytes(int(defaultSizeBytes + defaultHeaderBytes + size))

	if buf2 == nil {
		buf1.Release()
		return FrameData, 0, nil, errors.ErrMessageTooLarge
	}

	data2 := buf2.Bytes()

	copy(data2[:defaultSizeBytes+defaultHeaderBytes], data1)

	buf1.Release()

	if _, err := io.ReadFull(reader, data2[defaultSizeBytes+defaultHeaderBytes:]); err != nil {
		buf2.Release()
		return FrameData, 0, nil, err
	}

	return frameType(header), 0, buf2, nil
}

// frameType maps the header byte to the frame type. The heartbeat flag takes precedence.
func frameType(header byte) FrameType {
	if header&handshakeBit == handshakeBit {
		return FrameHandshake
	}

	return FrameData
}

// PackMessage packs message as a buffer and returns the packed buffer or a packing error.
func (p *defaultPacker) PackMessage(message *Message) (buffer.Buffer, error) {
	if message.Route > int32(1<<(8*p.opts.routeBytes-1)-1) || message.Route < int32(-1<<(8*p.opts.routeBytes-1)) {
		return nil, errors.ErrRouteOverflow
	}

	if p.opts.seqBytes > 0 {
		if message.Seq > int32(1<<(8*p.opts.seqBytes-1)-1) || message.Seq < int32(-1<<(8*p.opts.seqBytes-1)) {
			return nil, errors.ErrSeqOverflow
		}
	}

	if len(message.Buffer) > p.opts.bufferBytes {
		return nil, errors.ErrMessageTooLarge
	}

	writer := buffer.MallocWriter(defaultSizeBytes + defaultHeaderBytes + p.opts.routeBytes + p.opts.seqBytes)
	writer.WriteInt32s(p.opts.byteOrder, int32(defaultHeaderBytes+p.opts.routeBytes+p.opts.seqBytes+len(message.Buffer)))
	writer.WriteInt8s(int8(dataBit))

	switch p.opts.routeBytes {
	case 1:
		writer.WriteInt8s(int8(message.Route))
	case 2:
		writer.WriteInt16s(p.opts.byteOrder, int16(message.Route))
	case 4:
		writer.WriteInt32s(p.opts.byteOrder, message.Route)
	default:
		writer.Release()
		return nil, errors.ErrInvalidMessage
	}

	switch p.opts.seqBytes {
	case 0:
		// ignore seq
	case 1:
		writer.WriteInt8s(int8(message.Seq))
	case 2:
		writer.WriteInt16s(p.opts.byteOrder, int16(message.Seq))
	case 4:
		writer.WriteInt32s(p.opts.byteOrder, message.Seq)
	default:
		writer.Release()
		return nil, errors.ErrInvalidMessage
	}

	return buffer.NewNocopyBuffer(writer, message.Buffer), nil
}

// PackHandshake packs a handshake frame.
//
// The frame layout is the header followed by the handshake data, so it carries neither a route nor
// a sequence number and does not consume a route.
func (p *defaultPacker) PackHandshake(payload []byte) (buffer.Buffer, error) {
	if len(payload) > p.opts.bufferBytes {
		return nil, errors.ErrMessageTooLarge
	}

	writer := buffer.MallocWriter(defaultSizeBytes + defaultHeaderBytes)
	writer.WriteInt32s(p.opts.byteOrder, int32(defaultHeaderBytes+len(payload)))
	writer.WriteInt8s(int8(handshakeBit))

	return buffer.NewNocopyBuffer(writer, payload), nil
}

// ExtractHandshake extracts the payload of a handshake frame.
func (p *defaultPacker) ExtractHandshake(buf buffer.Buffer) ([]byte, error) {
	data := buf.Bytes()
	if len(data) < defaultSizeBytes+defaultHeaderBytes {
		return nil, errors.ErrInvalidMessage
	}

	if data[defaultSizeBytes]&handshakeBit != handshakeBit {
		return nil, errors.ErrInvalidMessage
	}

	if uint64(len(data))-defaultSizeBytes != uint64(p.opts.byteOrder.Uint32(data[:defaultSizeBytes])) {
		return nil, errors.ErrInvalidMessage
	}

	return data[defaultSizeBytes+defaultHeaderBytes:], nil
}

// ExtractRouteSeq extracts the route and sequence number from the message buffer buf. It returns
// the route, the sequence number and any unpacking error.
func (p *defaultPacker) ExtractRouteSeq(buf buffer.Buffer) (int32, int32, error) {
	var (
		ln   = defaultSizeBytes + defaultHeaderBytes + p.opts.routeBytes + p.opts.seqBytes
		data = buf.Bytes()
	)

	if len(data) < ln {
		return 0, 0, errors.ErrInvalidMessage
	}

	size := p.opts.byteOrder.Uint32(data[:defaultSizeBytes])

	if uint64(len(data))-defaultSizeBytes != uint64(size) {
		return 0, 0, errors.ErrInvalidMessage
	}

	header := data[defaultSizeBytes]

	if header&heartbeatBit == heartbeatBit || header&handshakeBit == handshakeBit {
		return 0, 0, errors.ErrInvalidMessage
	}

	var route int32

	switch p.opts.routeBytes {
	case 1:
		route = int32(int8(data[defaultSizeBytes+defaultHeaderBytes]))
	case 2:
		route = int32(int16(p.opts.byteOrder.Uint16(data[defaultSizeBytes+defaultHeaderBytes : defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes])))
	case 4:
		route = int32(p.opts.byteOrder.Uint32(data[defaultSizeBytes+defaultHeaderBytes : defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes]))
	default:
		return 0, 0, errors.ErrInvalidMessage
	}

	var seq int32

	switch p.opts.seqBytes {
	case 0:
		// ignore seq
	case 1:
		seq = int32(int8(data[defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes]))
	case 2:
		seq = int32(int16(p.opts.byteOrder.Uint16(data[defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes : defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes+p.opts.seqBytes])))
	case 4:
		seq = int32(p.opts.byteOrder.Uint32(data[defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes : defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes+p.opts.seqBytes]))
	default:
		return 0, 0, errors.ErrInvalidMessage
	}

	return route, seq, nil
}

// ExtractMessage returns the route, the sequence number and the payload of buf without modifying
// the buffer.
func (p *defaultPacker) ExtractMessage(buf buffer.Buffer) (int32, int32, []byte, error) {
	var (
		ln   = defaultSizeBytes + defaultHeaderBytes + p.opts.routeBytes + p.opts.seqBytes
		data = buf.Bytes()
	)

	if len(data) < ln {
		return 0, 0, nil, errors.ErrInvalidMessage
	}

	size := p.opts.byteOrder.Uint32(data[:defaultSizeBytes])

	if uint64(len(data))-defaultSizeBytes != uint64(size) {
		return 0, 0, nil, errors.ErrInvalidMessage
	}

	if header := data[defaultSizeBytes]; header&heartbeatBit == heartbeatBit || header&handshakeBit == handshakeBit {
		return 0, 0, nil, errors.ErrInvalidMessage
	}

	var route int32

	switch p.opts.routeBytes {
	case 1:
		route = int32(int8(data[defaultSizeBytes+defaultHeaderBytes]))
	case 2:
		route = int32(int16(p.opts.byteOrder.Uint16(data[defaultSizeBytes+defaultHeaderBytes : defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes])))
	case 4:
		route = int32(p.opts.byteOrder.Uint32(data[defaultSizeBytes+defaultHeaderBytes : defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes]))
	default:
		return 0, 0, nil, errors.ErrInvalidMessage
	}

	var seq int32

	switch p.opts.seqBytes {
	case 0:
		// ignore seq
	case 1:
		seq = int32(int8(data[defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes]))
	case 2:
		seq = int32(int16(p.opts.byteOrder.Uint16(data[defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes : defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes+p.opts.seqBytes])))
	case 4:
		seq = int32(p.opts.byteOrder.Uint32(data[defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes : defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes+p.opts.seqBytes]))
	default:
		return 0, 0, nil, errors.ErrInvalidMessage
	}

	return route, seq, data[ln:], nil
}

// UnpackMessage unpacks the message buffer buf. It returns the route, the sequence number,
// the message buffer and any unpacking error.
func (p *defaultPacker) UnpackMessage(buf buffer.Buffer) (int32, int32, buffer.Buffer, error) {
	var (
		ln   = defaultSizeBytes + defaultHeaderBytes + p.opts.routeBytes + p.opts.seqBytes
		data = buf.Bytes()
	)

	if len(data) < ln {
		return 0, 0, nil, errors.ErrInvalidMessage
	}

	size := p.opts.byteOrder.Uint32(data[:defaultSizeBytes])

	if uint64(len(data))-defaultSizeBytes != uint64(size) {
		return 0, 0, nil, errors.ErrInvalidMessage
	}

	header := data[defaultSizeBytes]

	if header&heartbeatBit == heartbeatBit || header&handshakeBit == handshakeBit {
		return 0, 0, nil, errors.ErrInvalidMessage
	}

	var route int32

	switch p.opts.routeBytes {
	case 1:
		route = int32(int8(data[defaultSizeBytes+defaultHeaderBytes]))
	case 2:
		route = int32(int16(p.opts.byteOrder.Uint16(data[defaultSizeBytes+defaultHeaderBytes : defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes])))
	case 4:
		route = int32(p.opts.byteOrder.Uint32(data[defaultSizeBytes+defaultHeaderBytes : defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes]))
	default:
		return 0, 0, nil, errors.ErrInvalidMessage
	}

	var seq int32

	switch p.opts.seqBytes {
	case 0:
		// ignore seq
	case 1:
		seq = int32(int8(data[defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes]))
	case 2:
		seq = int32(int16(p.opts.byteOrder.Uint16(data[defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes : defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes+p.opts.seqBytes])))
	case 4:
		seq = int32(p.opts.byteOrder.Uint32(data[defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes : defaultSizeBytes+defaultHeaderBytes+p.opts.routeBytes+p.opts.seqBytes]))
	default:
		return 0, 0, nil, errors.ErrInvalidMessage
	}

	buf.Slide(ln)

	return route, seq, buf, nil
}

// PackHeartbeat packs a heartbeat.
//
// When the heartbeat time is enabled it carries the current timestamp; otherwise it returns the
// prebuilt heartbeat packet. Note that the packet returned without a heartbeat time is a shared,
// prebuilt buffer that the caller must neither modify nor release.
func (p *defaultPacker) PackHeartbeat(server ...bool) buffer.Buffer {
	if p.opts.heartbeatTime && len(server) > 0 && server[0] {
		writer := buffer.MallocWriter(defaultSizeBytes + defaultHeaderBytes + defaultHeartbeatTimeBytes)
		writer.WriteUint32s(p.opts.byteOrder, uint32(defaultHeaderBytes+defaultHeartbeatTimeBytes))
		writer.WriteUint8s(uint8(heartbeatBit))
		writer.WriteUint64s(p.opts.byteOrder, uint64(time.Now().UnixNano()))

		return writer
	} else {
		return p.heartbeat
	}
}

// init initializes the packer by building the base heartbeat packet without a timestamp using the
// configured byte order.
func (p *defaultPacker) init() {
	writer := buffer.NewWriter(make([]byte, defaultSizeBytes+defaultHeaderBytes), true)
	writer.WriteUint32s(p.opts.byteOrder, uint32(defaultHeaderBytes))
	writer.WriteUint8s(uint8(heartbeatBit))

	p.heartbeat = writer
}

package packet

import (
	"io"

	"github.com/dobyte/due/v2/core/buffer"
)

var globalPacker Packer

// MessageExtractor is implemented by packers that can extract the payload of a packed message
// without modifying the buffer.
type MessageExtractor interface {
	// ExtractMessage returns the route, the sequence number and the payload of buf.
	ExtractMessage(buf buffer.Buffer) (int32, int32, []byte, error)
}

// MessageBufferPacker packs a pooled payload without copying it. On success the returned buffer
// owns payload; on error the caller retains ownership.
type MessageBufferPacker interface {
	PackMessageBuffer(route, seq int32, payload *buffer.Bytes) (buffer.Buffer, error)
}

// init initializes the global packer.
func init() {
	globalPacker = NewPacker()
}

// SetPacker sets the global packer.
//
// It overrides the global packer and is used to replace the default packing implementation.
func SetPacker(packer Packer) {
	globalPacker = packer
}

// GetPacker returns the global packer.
func GetPacker() Packer {
	return globalPacker
}

// Read reads a frame as a buffer. It reports the frame type, the server-side timestamp in
// nanoseconds, the frame buffer and any read error.
func Read(reader io.Reader) (FrameType, int64, buffer.Buffer, error) {
	return globalPacker.Read(reader)
}

// PackMessage packs message as a buffer and returns the packed buffer or a packing error.
func PackMessage(message *Message) (buffer.Buffer, error) {
	return globalPacker.PackMessage(message)
}

// PackMessageBuffer packs a pooled payload, transferring ownership only on success. Packers
// without MessageBufferPacker support receive a copy so the pooled memory can be released safely.
func PackMessageBuffer(route, seq int32, payload *buffer.Bytes) (buffer.Buffer, error) {
	if packer, ok := globalPacker.(MessageBufferPacker); ok {
		return packer.PackMessageBuffer(route, seq, payload)
	}
	packed, err := globalPacker.PackMessage(&Message{
		Route: route, Seq: seq, Buffer: append([]byte(nil), payload.Bytes()...),
	})
	if err == nil {
		payload.Release()
	}
	return packed, err
}

// PackHandshake packs a handshake frame.
//
// Handshake frames carry neither a route nor a sequence number, and they are marked by a reserved
// header bit, so they do not consume a route number.
func PackHandshake(payload []byte) (buffer.Buffer, error) {
	return globalPacker.PackHandshake(payload)
}

// ExtractHandshake extracts the payload of a handshake frame.
func ExtractHandshake(buf buffer.Buffer) ([]byte, error) {
	return globalPacker.ExtractHandshake(buf)
}

// ExtractRouteSeq extracts the route and sequence number from the message buffer buf. It returns
// the route, the sequence number and any unpacking error.
func ExtractRouteSeq(buf buffer.Buffer) (int32, int32, error) {
	return globalPacker.ExtractRouteSeq(buf)
}

// UnpackMessage unpacks the message buffer buf. It returns the route, the sequence number, the
// message buffer and any unpacking error.
func UnpackMessage(buf buffer.Buffer) (int32, int32, buffer.Buffer, error) {
	return globalPacker.UnpackMessage(buf)
}

// ExtractMessage returns the route, the sequence number and the payload of buf without modifying
// the buffer, so it is safe to call on a shared buffer. The payload may alias buf and must not be
// used after buf is released or modified.
func ExtractMessage(buf buffer.Buffer) (int32, int32, []byte, error) {
	if extractor, ok := globalPacker.(MessageExtractor); ok {
		return extractor.ExtractMessage(buf)
	}

	clone := buffer.NewBytes(append([]byte(nil), buf.Bytes()...), true)

	route, seq, data, err := globalPacker.UnpackMessage(clone)
	if err != nil {
		return 0, 0, nil, err
	}

	return route, seq, append([]byte(nil), data.Bytes()...), nil
}

// PackHeartbeat packs a heartbeat.
//
// The returned heartbeat buffer must neither be modified nor released. Pass server as true to pack
// a server-side heartbeat.
func PackHeartbeat(server ...bool) buffer.Buffer {
	return globalPacker.PackHeartbeat(server...)
}

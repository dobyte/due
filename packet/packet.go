package packet

import (
	"io"

	"github.com/dobyte/due/v2/core/buffer"
)

var globalPacker Packer

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

// Read reads a message as a buffer. It reports whether the message is a heartbeat, the server-side
// timestamp in nanoseconds, the message buffer and any read error.
func Read(reader io.Reader) (bool, int64, buffer.Buffer, error) {
	return globalPacker.Read(reader)
}

// PackMessage packs message as a buffer and returns the packed buffer or a packing error.
func PackMessage(message *Message) (buffer.Buffer, error) {
	return globalPacker.PackMessage(message)
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

// PackHeartbeat packs a heartbeat.
//
// The returned heartbeat buffer must neither be modified nor released. Pass server as true to pack
// a server-side heartbeat.
func PackHeartbeat(server ...bool) buffer.Buffer {
	return globalPacker.PackHeartbeat(server...)
}

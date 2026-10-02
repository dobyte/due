package protocol

import (
	"encoding/binary"

	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/def"
)

// heartbeat is the shared read-only heartbeat packet.
var heartbeat []byte

func init() {
	writer := buffer.NewWriterWithCapacity(def.SizeBytes + def.HeaderBytes)
	writer.WriteUint32s(binary.BigEndian, uint32(def.HeaderBytes))
	writer.WriteUint8s(def.HeartbeatBit)
	heartbeat = writer.Bytes()
}

func Heartbeat() []byte {
	return heartbeat
}

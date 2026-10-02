package def

const (
	B8 = 1 << iota
	B16
	B32
	B64
)

const (
	SizeBytes   = B32 // Number of bytes in the packet length
	HeaderBytes = B8  // Number of bytes in the header
	SeqBytes    = B64 // Number of bytes in the sequence number
	RouteBytes  = B8  // Number of bytes in the route number
	CodeBytes   = B16 // Number of bytes in the error code
)

// MaxFrameSize is the maximum length of a single frame, including the 4-byte size field.
const MaxFrameSize = 16 << 20 // 16MB

// MinFrameSize is the minimum length of a non-heartbeat frame: size field + header + route + seq.
const MinFrameSize = SizeBytes + HeaderBytes + RouteBytes + SeqBytes

const (
	DataBit       uint8 = 0 << 7 // Data flag
	HeartbeatBit  uint8 = 1 << 7 // Heartbeat flag
	DisconnectBit uint8 = 1 << 6 // Disconnect flag
)

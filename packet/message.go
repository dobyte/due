package packet

// Message is a message.
//
// It is used to pass plain data messages between encoding and decoding.
type Message struct {
	Seq    int32  // Sequence number
	Route  int32  // Route ID
	Buffer []byte // Message content
}

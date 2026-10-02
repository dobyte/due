package quic

import "time"

const (
	protocol            = "quic"
	alpn                = "due-quic"
	defaultCloseTimeout = 5 * time.Second
)

// maxBatchWriteNum is the maximum number of tasks written in a single batch.
const maxBatchWriteNum = 64

// minWriteQueueSize is the minimum write queue size.
const minWriteQueueSize = 128

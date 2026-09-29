package quic

import "time"

const (
	protocol            = "quic"
	alpn                = "due-quic"
	defaultCloseTimeout = 5 * time.Second
)

// maxBatchWriteNum 单次批量写入的最大任务数
const maxBatchWriteNum = 64

// minWriteQueueSize 最小写入队列大小
const minWriteQueueSize = 128

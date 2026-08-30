package internal

// Format 日志输出格式
type Format string

const (
	FormatText Format = "text" // 文本格式
	FormatJson Format = "json" // JSON格式
)

// defaultBufferSize 日志缓冲区初始容量
const defaultBufferSize = 2048

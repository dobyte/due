package internal

// Format 日志输出格式
type Format string

const (
	FormatText Format = "text" // 文本格式
	FormatJson Format = "json" // JSON格式
)

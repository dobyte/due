package log

// Format 日志输出格式
type Format string

const (
	FormatText Format = "text" // 文本格式
	FormatJson Format = "json" // JSON格式
)

// Terminal 日志输出终端
type Terminal string

const (
	TerminalConsole Terminal = "console" // 控制台
	TerminalFile    Terminal = "file"    // 文件
)

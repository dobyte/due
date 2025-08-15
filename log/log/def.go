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

// 日志翻转规则
type FileRotate string

const (
	FileRotateNone   FileRotate = "none"   // 不翻转
	FileRotateYear   FileRotate = "year"   // 按年翻转
	FileRotateMonth  FileRotate = "month"  // 按月翻转
	FileRotateWeek   FileRotate = "week"   // 按周翻转
	FileRotateDay    FileRotate = "day"    // 按天翻转
	FileRotateHour   FileRotate = "hour"   // 按时翻转
	FileRotateMinute FileRotate = "minute" // 按分翻转
	FileRotateSecond FileRotate = "second" // 按秒翻转
)

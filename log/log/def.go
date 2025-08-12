package log

// 日志翻转规则
type FileRotate int

const (
	FileRotateNone     FileRotate = iota // 不翻转
	FileRotateByYear                     // 按年翻转
	FileRotateByMonth                    // 按月翻转
	FileRotateByDay                      // 按天翻转
	FileRotateByHour                     // 按时翻转
	FileRotateByMinute                   // 按分翻转
	FileRotateBySecond                   // 按秒翻转
)

// Format 日志输出格式
type Format int

const (
	FormatText Format = iota // 文本格式
	FormatJson               // JSON格式
)

func (f Format) String() string {
	switch f {
	case FormatJson:
		return "json"
	default:
		return "text"
	}
}

// Terminal 日志输出终端
type Terminal int

const (
	TerminalConsole Terminal = iota // 控制台
	TerminalFile                    // 文件
)

func (t Terminal) String() string {
	switch t {
	case TerminalConsole:
		return "console"
	case TerminalFile:
		return "file"
	default:
		return "console"
	}
}

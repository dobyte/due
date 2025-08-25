package file

import "github.com/dobyte/due/v2/log/internal"

// 日志翻转规则
type Rotate string

const (
	RotateNone   Rotate = "none"   // 不翻转
	RotateYear   Rotate = "year"   // 按年翻转
	RotateMonth  Rotate = "month"  // 按月翻转
	RotateWeek   Rotate = "week"   // 按周翻转
	RotateDay    Rotate = "day"    // 按天翻转
	RotateHour   Rotate = "hour"   // 按时翻转
	RotateMinute Rotate = "minute" // 按时翻转
)

// Format 日志输出格式
type Format = internal.Format

const (
	FormatText = internal.FormatText // 文本格式
	FormatJson = internal.FormatJson // JSON格式
)

package log

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

/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/7/13 1:45 上午
 * @Desc: TODO
 */

package log

//go:generate stringer -type Level -linecomment
const (
	DebugLevel Level = iota + 1 // DEBUG
	InfoLevel                   // INFO
	WarnLevel                   // WARN
	ErrorLevel                  // ERROR
	FatalLevel                  // FATAL
	PanicLevel                  // PANIC
)

type Level int

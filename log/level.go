/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/7/13 1:45 上午
 * @Desc: TODO
 */

package log

//go:generate stringer -type Level -linecomment
const (
	LevelInfo  Level = iota + 1 // INFO
	LevelDebug                  // DEBUG
	LevelWarn                   // WARN
	LevelError                  // ERROR
	LevelFatal                  // FATAL
)

type Level int

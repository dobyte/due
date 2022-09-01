/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/8/30 5:08 下午
 * @Desc: TODO
 */

package log

// 日志输出格式
type Format int

const (
	TextFormat Format = iota // 文本格式
	JsonFormat               // JSON格式
)

// 日志切割规则
type CutRule int

const (
	YearCutRule   CutRule = iota // 按照年切割
	MonthCutRule                 // 按照月切割
	DayCutRule                   // 按照日切割
	HourCutRule                  // 按照时切割
	MinuteCutRule                // 按照分切割
	SecondCutRule                // 按照秒切割
)

package xtime

import (
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/dobyte/due/v2/etc"
)

const (
	Layout      = time.Layout // The reference time, in numerical order.
	ANSIC       = time.ANSIC
	UnixDate    = time.UnixDate
	RubyDate    = time.RubyDate
	RFC822      = time.RFC822
	RFC822Z     = time.RFC822Z
	RFC850      = time.RFC850
	RFC1123     = time.RFC1123
	RFC1123Z    = time.RFC1123Z
	RFC3339     = time.RFC3339
	RFC3339Nano = time.RFC3339Nano
	Kitchen     = time.Kitchen

	Stamp      = time.Stamp
	StampMilli = time.StampMilli
	StampMicro = time.StampMicro
	StampNano  = time.StampNano
	DateTime   = time.DateTime
	DateOnly   = time.DateOnly
	TimeOnly   = time.TimeOnly
	MonthOnly  = "2006-01"
	YearOnly   = "2006"
)

const (
	TimeFormat     = "H:i:s"
	DateFormat     = "Y-m-d"
	DatetimeFormat = "Y-m-d H:i:s"
)

var (
	location             atomic.Value
	defaultTransformRule = []TransformRule{
		{Max: 60, PastTpl: "%d秒前", FutureTpl: "%d秒后"},
		{Max: 3600, PastTpl: "%d分前", FutureTpl: "%d分后"},
		{Max: 86400, PastTpl: "%d小时前", FutureTpl: "%d小时后"},
		{Max: 2592000, PastTpl: "%d天前", FutureTpl: "%d天后"},
		{Max: 31536000, PastTpl: "%d月前", FutureTpl: "%d月后"},
		{Max: 0, PastTpl: "%d年前", FutureTpl: "%d年后"},
	}
)

// TransformRule 时间转换规则
type TransformRule struct {
	Max       uint   // 时间上限（秒），0 表示无上限
	PastTpl   string // 过去时间的显示模板
	FutureTpl string // 未来时间的显示模板
}

// Time 标准时间类型别名
type Time = time.Time

func init() {
	if loc, err := time.LoadLocation(etc.Get("etc.timezone", "Local").String()); err != nil {
		SetLocation(time.Local)
	} else {
		SetLocation(loc)
	}
}

// SetLocation 设置时间区域（线程安全）
// @param loc *time.Location 待设置的时间区域
func SetLocation(loc *time.Location) {
	if loc != nil {
		location.Store(loc)
	}
}

// GetLocation 获取当前时间区域
// @return @1 *time.Location 当前使用的时间区域
func GetLocation() *time.Location {
	if loc, ok := location.Load().(*time.Location); ok && loc != nil {
		return loc
	}

	return time.Local
}

// Parse 解析日期时间字符串
// 使用当前时区按指定布局解析日期时间
// @param layout string 时间布局
// @param value string 待解析的日期时间字符串
// @return @1 Time 解析后的时间
// @return @2 error 解析错误
func Parse(layout string, value string) (Time, error) {
	return time.ParseInLocation(layout, value, GetLocation())
}

// Now 当前时间
// @return @1 Time 当前时间（使用当前时区）
func Now() Time {
	return time.Now().In(GetLocation())
}

// Today 今天
// @return @1 Time 当前时间
func Today() Time {
	return Now()
}

// Yesterday 昨天
// @return @1 Time 昨天的当前时刻
func Yesterday() Time {
	return Day(-1)
}

// Tomorrow 明天
// @return @1 Time 明天的当前时刻
func Tomorrow() Time {
	return Day(1)
}

// Transform 时间转换
// @param t Time 目标时间
// @param rule ...[]TransformRule 可选自定义转换规则
// @return @1 string 转换后的时间描述
func Transform(t Time, rule ...[]TransformRule) string {
	var (
		dur           = Now().Unix() - t.Unix()
		future        bool
		transformRule = defaultTransformRule
		molecular     = uint64(1)
	)

	if dur < 0 {
		future = true
		dur = -dur
	}

	if len(rule) != 0 && len(rule[0]) != 0 {
		transformRule = rule[0]
	}

	for i, r := range transformRule {
		if i == len(transformRule)-1 || r.Max == 0 || uint64(dur) < uint64(r.Max) {
			tpl := r.PastTpl
			if future && r.FutureTpl != "" {
				tpl = r.FutureTpl
			}

			if strings.Contains(tpl, "%d") {
				return fmt.Sprintf(tpl, uint64(dur)/molecular)
			}

			return tpl
		}

		molecular = uint64(r.Max)
	}

	return ""
}

// Unix 时间戳转标准时间
// @param sec int64 秒级时间戳
// @param nsec ...int64 可选，纳秒部分
// @return @1 Time 转换后的标准时间
func Unix(sec int64, nsec ...int64) Time {
	if len(nsec) > 0 {
		return time.Unix(sec, nsec[0]).In(GetLocation())
	} else {
		return time.Unix(sec, 0).In(GetLocation())
	}
}

// UnixMilli 时间戳（毫秒）转标准时间
// @param msec int64 毫秒级时间戳
// @return @1 Time 转换后的标准时间
func UnixMilli(msec int64) Time {
	return time.Unix(msec/1e3, (msec%1e3)*1e6).In(GetLocation())
}

// UnixMicro 时间戳（微秒）转标准时间
// @param usec int64 微秒级时间戳
// @return @1 Time 转换后的标准时间
func UnixMicro(usec int64) Time {
	return time.Unix(usec/1e6, (usec%1e6)*1e3).In(GetLocation())
}

// UnixNano 时间戳（纳秒）转标准时间
// @param nsec int64 纳秒级时间戳
// @return @1 Time 转换后的标准时间
func UnixNano(nsec int64) Time {
	return time.Unix(nsec/1e9, nsec%1e9).In(GetLocation())
}

// Day 获取某一天的当前时刻
// @param offset ...int 偏移天数，例如：-1：前一天 0：当前 1：明天
// @return @1 Time 偏移后的时间
func Day(offset ...int) Time {
	now := Now()

	if len(offset) > 0 {
		now = now.AddDate(0, 0, offset[0])
	}

	return now
}

// DayHead 获取一天中的第一秒
// @param offset ...int 偏移天数，例如：-1：前一天 0：当前 1：明天
// @return @1 Time 当天第一秒
func DayHead(offset ...int) Time {
	date := Day(offset...)

	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
}

// DayTail 获取一天中的最后一秒
// @param offset ...int 偏移天数，例如：-1：前一天 0：当前 1：明天
// @return @1 Time 当天最后一秒
func DayTail(offset ...int) Time {
	date := Day(offset...)

	return time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 999999999, date.Location())
}

// Week 获取一周中的当前时刻
// @param offset ...int 偏移周数，例如：-1：上一周 0：本周 1：下一周
// @return @1 Time 偏移后的时间
func Week(offset ...int) Time {
	if len(offset) > 0 {
		return Now().AddDate(0, 0, offset[0]*7)
	} else {
		return Now()
	}
}

// WeekHead 获取一周中的第一天的第一秒（以周一为一周第一天）
// @param offset ...int 偏移周数，例如：-1：上一周 0：本周 1：下一周
// @return @1 Time 目标周的周一第一秒
func WeekHead(offset ...int) Time {
	var (
		now        = Now()
		offsetDays = int(time.Monday - now.Weekday())
	)

	if offsetDays == 1 {
		offsetDays = -6
	}

	if len(offset) > 0 {
		offsetDays += offset[0] * 7
	}

	date := now.AddDate(0, 0, offsetDays)

	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
}

// WeekTail 获取一周中的最后一天的最后一秒（以周日为一周最后一天）
// @param offset ...int 偏移周数，例如：-1：上一周 0：本周 1：下一周
// @return @1 Time 目标周的周日最后一秒
func WeekTail(offset ...int) Time {
	var (
		now        = Now()
		offsetDays = int(time.Sunday - now.Weekday() + 7)
	)

	if len(offset) > 0 {
		offsetDays += offset[0] * 7
	}

	date := now.AddDate(0, 0, offsetDays)

	return time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 999999999, date.Location())
}

// Month 获取某一月的当前时刻
// @param offset ...int 偏移月数，例如：-1：前一月 0：当前月 1：下一月
// @return @1 Time 偏移后的时间
func Month(offset ...int) Time {
	now := Now()

	if len(offset) == 0 || offset[0] == 0 {
		return now
	}

	offsetYears := offset[0] / 12
	offsetMonths := offset[0] % 12
	year := now.Year() + offsetYears
	month := int(now.Month()) + offsetMonths
	day := now.Day()

	if month <= 0 {
		year--
		month += 12
	} else if month > 12 {
		year++
		month -= 12
	}

	switch time.Month(month) {
	case time.April, time.June, time.September, time.November:
		if day > 30 {
			day = 30
		}
	case time.February:
		if IsLeapYear(year) {
			if day > 29 {
				day = 29
			}
		} else {
			if day > 28 {
				day = 28
			}
		}
	}

	return time.Date(year, time.Month(month), day, now.Hour(), now.Minute(), now.Second(), now.Nanosecond(), now.Location())
}

// MonthHead 获取一月中的第一天的第一秒
// @param offset ...int 偏移月数，例如：-1：前一月 0：当前月 1：下一月
// @return @1 Time 目标月第一天第一秒
func MonthHead(offset ...int) Time {
	now := Now()

	if len(offset) == 0 || offset[0] == 0 {
		return time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	}

	offsetYears := offset[0] / 12
	offsetMonths := offset[0] % 12
	year := now.Year() + offsetYears
	month := int(now.Month()) + offsetMonths

	if month <= 0 {
		year--
		month += 12
	} else if month > 12 {
		year++
		month -= 12
	}

	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, now.Location())
}

// MonthTail 获取一月中的最后一天的最后一秒
// @param offset ...int 偏移月数，例如：-1：前一月 0：当前月 1：下一月
// @return @1 Time 目标月最后一天最后一秒
func MonthTail(offset ...int) Time {
	var (
		now          = Now()
		offsetYears  int
		offsetMonths int
	)

	if len(offset) > 0 {
		offsetYears = offset[0] / 12
		offsetMonths = offset[0] % 12
	}

	year := now.Year() + offsetYears
	month := int(now.Month()) + offsetMonths

	if month <= 0 {
		year--
		month += 12
	} else if month > 12 {
		year++
		month -= 12
	}

	var day int
	switch time.Month(month) {
	case time.January, time.March, time.May, time.July, time.August, time.October, time.December:
		day = 31
	case time.April, time.June, time.September, time.November:
		day = 30
	case time.February:
		if IsLeapYear(year) {
			day = 29
		} else {
			day = 28
		}
	}

	return time.Date(year, time.Month(month), day, 23, 59, 59, 999999999, now.Location())
}

// IsLeapYear 是否是闰年
// @param year int 年份
// @return @1 bool 是否为闰年
func IsLeapYear(year int) bool {
	return (year%4 == 0 && year%100 != 0) || year%400 == 0
}

package xtime

import (
	"fmt"
	"math"
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
)

const (
	TimeFormat     = "H:i:s"
	DateFormat     = "Y-m-d"
	DatetimeFormat = "Y-m-d H:i:s"
)

var (
	location             *time.Location
	defaultTransformRule = []TransformRule{
		{
			Max: 60,
			Tpl: "刚刚",
		}, {
			Max: 3600,
			Tpl: "%d分钟前",
		}, {
			Max: 86400,
			Tpl: "%d小时前",
		}, {
			Max: 2592000,
			Tpl: "%d天前",
		}, {
			Max: 31536000,
			Tpl: "%d个月前",
		}, {
			Max: 0,
			Tpl: "%d年前",
		},
	}
)

type TransformRule struct {
	Max uint
	Tpl string
}

type Time = time.Time

func init() {
	timezone := etc.Get("etc.timezone", "Local").String()
	if loc, err := time.LoadLocation(timezone); err != nil {
		location = time.Local
	} else {
		location = loc
	}
}

// Parse 解析日期时间
func Parse(layout string, value string) (Time, error) {
	return time.ParseInLocation(layout, value, location)
}

// Now 当前时间
func Now() Time {
	return time.Now().In(location)
}

// Today 今天
func Today() Time {
	return Now()
}

// Yesterday 昨天
func Yesterday() Time {
	return Day(-1)
}

// Tomorrow 明天
func Tomorrow() Time {
	return Day(1)
}

// Transform 时间转换
func Transform(t Time, rule ...[]TransformRule) string {
	var (
		dur                = uint(Now().Unix() - t.In(location).Unix())
		molecular     uint = 1
		transformRule      = defaultTransformRule
	)

	if len(rule) != 0 {
		transformRule = rule[0]
	}

	for i, r := range defaultTransformRule {
		if i == len(transformRule)-1 || dur < r.Max {
			return fmt.Sprintf(r.Tpl, int(math.Floor(float64(dur/molecular))))
		} else {
			molecular = r.Max
		}
	}

	return ""
}

// Unix 时间戳转标准时间
func Unix(sec int64, nsec ...int64) Time {
	if len(nsec) > 0 {
		return time.Unix(sec, nsec[0]).In(location)
	} else {
		return time.Unix(sec, 0).In(location)
	}
}

// UnixMilli 时间戳（毫秒）转标准时间
func UnixMilli(msec int64) Time {
	return time.Unix(msec/1e3, (msec%1e3)*1e6).In(location)
}

// UnixMicro 时间戳（微秒）转标准时间
func UnixMicro(usec int64) Time {
	return time.Unix(usec/1e6, (usec%1e6)*1e3).In(location)
}

// UnixNano 时间戳（纳秒）转标准时间
func UnixNano(nsec int64) Time {
	return time.Unix(nsec/1e9, nsec%1e9).In(location)
}

// Day 获取某一天的当前时刻
// offsetDays 偏移天数，例如：-1：前一天 0：当前 1：明天
func Day(offset ...int) Time {
	now := Now()

	if len(offset) > 0 {
		now = now.AddDate(0, 0, offset[0])
	}

	return now
}

// DayHead 获取一天中的第一秒
// offsetDays 偏移天数，例如：-1：前一天 0：当前 1：明天
func DayHead(offset ...int) Time {
	date := Day(offset...)

	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
}

// DayTail 获取一天中的最后一秒
// offsetDays 偏移天数，例如：-1：前一天 0：当前 1：明天
func DayTail(offset ...int) Time {
	date := Day(offset...)

	return time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 999999999, date.Location())
}

// Week 获取一周中的当前时刻
// offsetWeeks 偏移周数，例如：-1：上一周 0：本周 1：下一周
func Week(offset ...int) Time {
	if len(offset) > 0 {
		return Now().AddDate(0, 0, offset[0]*7)
	} else {
		return Now()
	}
}

// WeekHead 获取一周中的第一天的第一秒
// offsetWeeks 偏移周数，例如：-1：上一周 0：本周 1：下一周
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

// WeekTail 获取一周中的最后一天的最后一秒
// offsetWeeks 偏移周数，例如：-1：上一周 0：本周 1：下一周
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
// offsetMonths 偏移月数，例如：-1：前一月 0：当前月 1：下一月
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
	}

	switch time.Month(month) {
	case time.April, time.June, time.September, time.November:
		if day > 30 {
			day = 30
		}
	case time.February:
		if (year%4 == 0 && year%100 != 0) || year%400 == 0 {
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
// offset 偏移月数，例如：-1：前一月 0：当前月 1：下一月
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
	}

	return time.Date(year, time.Month(month), 1, 0, 0, 0, 0, now.Location())
}

// MonthTail 获取一月中的最后一天的最后一秒
// offset 偏移月数，例如：-1：前一月 0：当前月 1：下一月
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
	}

	var day int
	switch time.Month(month) {
	case time.January, time.March, time.May, time.July, time.August, time.October, time.December:
		day = 31
	case time.April, time.June, time.September, time.November:
		day = 30
	case time.February:
		if (year%4 == 0 && year%100 != 0) || year%400 == 0 {
			day = 29
		} else {
			day = 28
		}
	}

	return time.Date(year, time.Month(month), day, 23, 59, 59, 999999999, now.Location())
}

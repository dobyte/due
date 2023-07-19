package xtime

import (
	"fmt"
	"github.com/dobyte/due/v2/config"
	"math"
	"time"
)

const (
	TimeLayout     = "15:04:05"
	DateLayout     = "2006-01-02"
	DatetimeLayout = "2006-01-02 15:04:05"
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

func init() {
	timezone := config.Get("config.timezone", "Local").String()
	if loc, err := time.LoadLocation(timezone); err != nil {
		location = time.Local
	} else {
		location = loc
	}
}

// Now 当前时间
func Now() time.Time {
	return time.Now().In(location)
}

// Today 今天
func Today() time.Time {
	now := Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// Yesterday 昨天
func Yesterday() time.Time {
	return Today().AddDate(0, 0, -1)
}

// Tomorrow 明天
func Tomorrow() time.Time {
	return Today().AddDate(0, 0, 1)
}

// Transform 时间转换
func Transform(t time.Time, rule ...[]TransformRule) string {
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
func Unix(sec, nsec int64) time.Time {
	return time.Unix(sec, nsec).In(location)
}

// UnixMilli 时间戳（毫秒）转标准时间
func UnixMilli(msec int64) time.Time {
	return time.Unix(msec/1e3, (msec%1e3)*1e6).In(location)
}

// UnixMicro 时间戳（微秒）转标准时间
func UnixMicro(usec int64) time.Time {
	return time.Unix(usec/1e6, (usec%1e6)*1e3).In(location)
}

// UnixNano 时间戳（纳秒）转标准时间
func UnixNano(nsec int64) time.Time {
	return time.Unix(nsec/1e9, nsec%1e9).In(location)
}

// GetFirstSecondOfDay 获取一天中的第一秒
// offsetDays 		   偏移天数，例如：-1：前一天 0：当前 1：明天
func GetFirstSecondOfDay(offsetDays ...int) time.Time {
	now := Now()
	if len(offsetDays) > 0 {
		now = now.AddDate(0, 0, offsetDays[0])
	}

	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
}

// GetLastSecondOfDay 获取一天中的最后一秒
// offsetDays 		  偏移天数，例如：-1：前一天 0：当前 1：明天
func GetLastSecondOfDay(offsetDays ...int) time.Time {
	now := Now()
	if len(offsetDays) > 0 {
		now = now.AddDate(0, 0, offsetDays[0])
	}

	return time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 0, now.Location())
}

// GetFirstDayOfWeek 获取一周中的第一天
// offsetWeeks       偏移周数，例如：-1：上一周 0：本周 1：下一周
func GetFirstDayOfWeek(offsetWeeks ...int) time.Time {
	var (
		now        = Now()
		offsetDays = int(time.Monday - now.Weekday())
	)

	if offsetDays == 1 {
		offsetDays = -6
	}

	if len(offsetWeeks) > 0 {
		offsetDays += offsetWeeks[0] * 7
	}

	return now.AddDate(0, 0, offsetDays)
}

// GetLastDayOfWeek 获取一周中的最后一天
// offsetWeeks      偏移周数，例如：-1：上一周 0：本周 1：下一周
func GetLastDayOfWeek(offsetWeeks ...int) time.Time {
	var (
		now        = Now()
		offsetDays = int(time.Sunday - now.Weekday() + 7)
	)

	if len(offsetWeeks) > 0 {
		offsetDays += offsetWeeks[0] * 7
	}

	return now.AddDate(0, 0, offsetDays)
}

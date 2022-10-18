package xtime

import (
	"fmt"
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

var defaultTransformRule = []TransformRule{
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

type TransformRule struct {
	Max uint
	Tpl string
}

// Today 今天
func Today() time.Time {
	now := time.Now()
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
		dur                = uint(time.Now().Unix() - t.Unix())
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

// UnixToString unix时间转字符串
func UnixToString(unix int64, format string) string {
	return time.Unix(unix, 0).Local().Format(format)
}

// FirstSecondOfDay 获取一天中的第一秒
func FirstSecondOfDay(offset ...int) time.Time {
	var t = time.Now()

	if len(offset) > 0 {
		t = t.AddDate(0, 0, offset[0])
	}

	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// LastSecondOfDay 获取一天中的最后一秒
func LastSecondOfDay(offset ...int) time.Time {
	var t = time.Now()

	if len(offset) > 0 {
		t = t.AddDate(0, 0, offset[0])
	}

	return time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 0, t.Location())
}

// GetFirstDayOfWeek 获取一周中的第一天
// offsetWeek        偏移周，例如：-1：上一周 1：下一周
func GetFirstDayOfWeek(offsetWeek ...int) time.Time {
	var (
		now       = time.Now()
		offsetDay = int(time.Monday - now.Weekday())
	)

	if offsetDay == 1 {
		offsetDay = -6
	}

	if len(offsetWeek) > 0 {
		offsetDay += offsetWeek[0] * 7
	}

	return now.Local().AddDate(0, 0, offsetDay)
}

// GetLastDayOfWeek 获取一周中的最后一天
// offsetWeek       偏移周，例如：-1：上一周 1：下一周
func GetLastDayOfWeek(offsetWeek ...int) time.Time {
	var (
		now       = time.Now()
		offsetDay = int(time.Sunday - now.Weekday() + 7)
	)

	if len(offsetWeek) > 0 {
		offsetDay += offsetWeek[0] * 7
	}

	return now.Local().AddDate(0, 0, offsetDay)
}

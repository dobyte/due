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

// TransformRule describes how a time difference is rendered into a human-readable string.
type TransformRule struct {
	Max       uint   // Upper time bound in seconds, 0 means no upper bound
	PastTpl   string // Display template for a time in the past
	FutureTpl string // Display template for a time in the future
}

// Time is an alias of [time.Time].
type Time = time.Time

func init() {
	if loc, err := time.LoadLocation(etc.Get("etc.timezone", "Local").String()); err != nil {
		SetLocation(time.Local)
	} else {
		SetLocation(loc)
	}
}

// SetLocation sets the time zone. It is safe for concurrent use.
func SetLocation(loc *time.Location) {
	if loc != nil {
		location.Store(loc)
	}
}

// GetLocation returns the currently used time zone.
func GetLocation() *time.Location {
	if loc, ok := location.Load().(*time.Location); ok && loc != nil {
		return loc
	}

	return time.Local
}

// Parse parses a date-time string with the given layout in the current time zone.
func Parse(layout string, value string) (Time, error) {
	return time.ParseInLocation(layout, value, GetLocation())
}

// Now returns the current time in the current time zone.
func Now() Time {
	return time.Now().In(GetLocation())
}

// Today returns the current time.
func Today() Time {
	return Now()
}

// Yesterday returns the same time of day yesterday.
func Yesterday() Time {
	return Day(-1)
}

// Tomorrow returns the same time of day tomorrow.
func Tomorrow() Time {
	return Day(1)
}

// Transform renders t as a human-readable string relative to the current time. The optional rule
// provides a custom set of [TransformRule] values in place of the default rules.
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

// Unix returns the local time corresponding to the given Unix time in seconds. The optional nsec
// is the nanosecond offset.
func Unix(sec int64, nsec ...int64) Time {
	if len(nsec) > 0 {
		return time.Unix(sec, nsec[0]).In(GetLocation())
	} else {
		return time.Unix(sec, 0).In(GetLocation())
	}
}

// UnixMilli returns the local time corresponding to the given Unix time in milliseconds.
func UnixMilli(msec int64) Time {
	return time.Unix(msec/1e3, (msec%1e3)*1e6).In(GetLocation())
}

// UnixMicro returns the local time corresponding to the given Unix time in microseconds.
func UnixMicro(usec int64) Time {
	return time.Unix(usec/1e6, (usec%1e6)*1e3).In(GetLocation())
}

// UnixNano returns the local time corresponding to the given Unix time in nanoseconds.
func UnixNano(nsec int64) Time {
	return time.Unix(nsec/1e9, nsec%1e9).In(GetLocation())
}

// Day returns the current time shifted by the given number of days. For example, -1 is the previous
// day, 0 is the current day and 1 is the next day.
func Day(offset ...int) Time {
	now := Now()

	if len(offset) > 0 {
		now = now.AddDate(0, 0, offset[0])
	}

	return now
}

// DayHead returns the first second of the day, shifted by offset days.
func DayHead(offset ...int) Time {
	date := Day(offset...)

	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, date.Location())
}

// DayTail returns the last second of the day, shifted by offset days.
func DayTail(offset ...int) Time {
	date := Day(offset...)

	return time.Date(date.Year(), date.Month(), date.Day(), 23, 59, 59, 999999999, date.Location())
}

// Week returns the current time shifted by the given number of weeks. For example, -1 is the
// previous week, 0 is the current week and 1 is the next week.
func Week(offset ...int) Time {
	if len(offset) > 0 {
		return Now().AddDate(0, 0, offset[0]*7)
	} else {
		return Now()
	}
}

// WeekHead returns the first second of the first day of the week, with Monday as the first day.
// offset shifts the result by whole weeks.
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

// WeekTail returns the last second of the last day of the week, with Sunday as the last day. offset
// shifts the result by whole weeks.
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

// Month returns the current time shifted by the given number of months. For example, -1 is the
// previous month, 0 is the current month and 1 is the next month.
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

// MonthHead returns the first second of the first day of the month, shifted by offset months.
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

// MonthTail returns the last second of the last day of the month, shifted by offset months.
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

// IsLeapYear reports whether year is a leap year.
func IsLeapYear(year int) bool {
	return (year%4 == 0 && year%100 != 0) || year%400 == 0
}

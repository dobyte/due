package xtime_test

import (
	"strings"
	"testing"
	"time"

	"github.com/dobyte/due/v2/utils/xtime"
)

func TestNow(t *testing.T) {
	t.Log(xtime.Now().Format(xtime.DateTime))
}

func TestToday(t *testing.T) {
	t.Log(xtime.Today())
}

func TestDay(t *testing.T) {
	t.Log(xtime.Day())
	t.Log(xtime.Day(-1))
	t.Log(xtime.Day(1))
}

func TestDayHead(t *testing.T) {
	t.Log(xtime.DayHead())
	t.Log(xtime.DayHead(-1))
	t.Log(xtime.DayHead(1))
}

func TestDayTail(t *testing.T) {
	t.Log(xtime.DayTail())
	t.Log(xtime.DayTail(-1))
	t.Log(xtime.DayTail(1))
}

func TestWeek(t *testing.T) {
	t.Log(xtime.Week())
	t.Log(xtime.Week(-1))
	t.Log(xtime.Week(1))
}

func TestWeekHead(t *testing.T) {
	t.Log(xtime.WeekHead())
	t.Log(xtime.WeekHead(-1))
	t.Log(xtime.WeekHead(1))
}

func TestWeekTail(t *testing.T) {
	t.Log(xtime.WeekTail())
	t.Log(xtime.WeekTail(-1))
	t.Log(xtime.WeekTail(1))
}

func TestMonth(t *testing.T) {
	for i := 0; i <= 100; i++ {
		t.Log(xtime.Month(0 - i))
	}
}

func TestMonthHead(t *testing.T) {
	for i := 0; i <= 100; i++ {
		t.Log(xtime.MonthHead(0 - i))
	}
}

func TestMonthTail(t *testing.T) {
	for i := 0; i <= 100; i++ {
		t.Log(xtime.MonthTail(0 - i))
	}
}

func TestTransform(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name string
		t    time.Time
		want string
	}{
		{"过去5分钟", now.Add(-5 * time.Minute), "5分前"},
		{"过去5小时", now.Add(-5 * time.Hour), "5小时前"},
		{"过去5天", now.Add(-5 * 24 * time.Hour), "5天前"},
		{"过去5个月", now.Add(-150 * 24 * time.Hour), "5月前"},
		{"过去5年", now.Add(-5 * 365 * 24 * time.Hour), "5年前"},
		{"未来5分钟", now.Add(5 * time.Minute), "5分后"},
		{"未来5小时", now.Add(5 * time.Hour), "5小时后"},
		{"未来5天", now.Add(5 * 24 * time.Hour), "5天后"},
		{"未来5个月", now.Add(150 * 24 * time.Hour), "5月后"},
		{"未来5年", now.Add(5 * 365 * 24 * time.Hour), "5年后"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := xtime.Transform(tt.t); got != tt.want {
				t.Errorf("Transform() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestTransformSecond(t *testing.T) {
	if got := xtime.Transform(time.Now().Add(-30 * time.Second)); !strings.Contains(got, "秒前") {
		t.Errorf("Transform() = %q, want contains 秒前", got)
	}

	if got := xtime.Transform(time.Now().Add(30 * time.Second)); !strings.Contains(got, "秒后") {
		t.Errorf("Transform() = %q, want contains 秒后", got)
	}
}

func TestTransformCustomRule(t *testing.T) {
	// 自定义规则真正生效
	if got := xtime.Transform(time.Now().Add(-100*time.Second), []xtime.TransformRule{{Max: 0, PastTpl: "自定义"}}); got != "自定义" {
		t.Errorf("Transform() = %q, want 自定义", got)
	}

	// 无 %d 占位符模板
	rule := []xtime.TransformRule{{Max: 60, PastTpl: "刚刚"}, {Max: 0, PastTpl: "%d秒前"}}
	if got := xtime.Transform(time.Now().Add(-30*time.Second), rule); got != "刚刚" {
		t.Errorf("Transform() = %q, want 刚刚", got)
	}

	// 空规则回退默认规则
	if got := xtime.Transform(time.Now().Add(-5*time.Minute), []xtime.TransformRule{}); got != "5分前" {
		t.Errorf("Transform() = %q, want 5分前", got)
	}

	// 未来时间且自定义规则未设置 FutureTpl 时回退使用 PastTpl
	if got := xtime.Transform(time.Now().Add(100*time.Second), []xtime.TransformRule{{Max: 0, PastTpl: "自定义"}}); got != "自定义" {
		t.Errorf("Transform() = %q, want 自定义", got)
	}
}

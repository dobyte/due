package xtime_test

import (
	"github.com/dobyte/due/v2/utils/xtime"
	"testing"
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

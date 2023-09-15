package xtime_test

import (
	"github.com/symsimmy/due/utils/xtime"
	"testing"
)

func TestNow(t *testing.T) {
	t.Log(xtime.Now().Format(xtime.DatetimeLayout))
}

func TestToday(t *testing.T) {
	t.Log(xtime.Today())
}

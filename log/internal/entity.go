package internal

import (
	"runtime"
	"time"
)

type Entity struct {
	Now     time.Time
	Time    string
	Level   Level
	Message string
	Caller  string
	Frames  []runtime.Frame
}

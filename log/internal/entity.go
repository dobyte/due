package internal

import (
	"runtime"
)

type Entity struct {
	Time    string
	Level   Level
	Message string
	Caller  string
	Frames  []runtime.Frame
}

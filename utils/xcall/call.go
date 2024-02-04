package xcall

import (
	"github.com/dobyte/due/v2/log"
	"runtime"
)

func Call(fn func()) {
	if fn == nil {
		return
	}

	defer func() {
		if err := recover(); err != nil {
			switch err.(type) {
			case runtime.Error:
				log.Panic(err)
			default:
				log.Panicf("panic error: %v", err)
			}
		}
	}()

	fn()
}

func Go(fn func()) {
	go Call(fn)
}

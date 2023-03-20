package xcall

import (
	"github.com/dobyte/due/log"
	"runtime"
)

func Call(fn func()) {
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

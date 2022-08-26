package xcall

import "time"

// Call 调用函数
func Call(fn func() error, retry int, sleep time.Duration) error {
	if retry <= 0 {
		return fn()
	}

	var err error
	for i := 0; i <= retry; i++ {
		if err = fn(); err == nil {
			break
		}
		if i != retry && sleep > 0 {
			time.Sleep(sleep)
		}
	}

	return err
}

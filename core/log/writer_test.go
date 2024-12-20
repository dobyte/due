package log_test

import (
	"github.com/dobyte/due/v2/core/log"
	"github.com/dobyte/due/v2/utils/xrand"
	"testing"
)

func TestWriter_Write(t *testing.T) {
	str := xrand.Letters(log.KB) + "\n"

	w := log.NewWriter(
		log.WithFileMaxSize(2*log.KB),
		log.WithFileRotate(log.FileRotateByMinute),
		log.WithCompress(false),
	)

	for i := 0; i < 10; i++ {
		if _, err := w.Write([]byte(str)); err != nil {
			t.Fatal(err)
		}
	}
}

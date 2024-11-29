package writer_test

import (
	"github.com/dobyte/due/v2/core/writer"
	"github.com/dobyte/due/v2/utils/xrand"
	"testing"
)

func TestWriter_Write(t *testing.T) {
	str := xrand.Letters(writer.KB)

	w := writer.NewWriter(writer.WithFileMaxSize(2 * writer.MB))

	for i := 0; i < 1024; i++ {
		_, err := w.Write([]byte(str))
		if err != nil {
			t.Fatal(err)
		}
	}
}

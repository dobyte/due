package due_test

import (
	"flag"
	"github.com/dobyte/due"
	"testing"
)

func TestGetMode(t *testing.T) {
	flag.Parse()

	t.Log(due.GetMode())
}

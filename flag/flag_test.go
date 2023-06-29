package flag_test

import (
	"github.com/dobyte/due/flag"
	"testing"
)

func TestString(t *testing.T) {
	t.Log(flag.Bool("test.v"))
	t.Log(flag.String("config", "./config"))
}

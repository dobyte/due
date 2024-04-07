package xos_test

import (
	"github.com/dobyte/due/v2/utils/xos"
	"testing"
)

func TestCreate(t *testing.T) {
	_, err := xos.Create("./pprof/server/cpu_profile")
	if err != nil {
		t.Fatal(err)
	}

}

package drpc_test

import (
	"github.com/dobyte/due/v2/transport/drpc"
	"testing"
)

func Test_PackBindCMD(t *testing.T) {
	data, err := drpc.PackBindCMD(1, 2)
	if err != nil {
		t.Fatal(err)
	}

	t.Log(data)
}

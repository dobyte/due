package xnet_test

import (
	"github.com/dobyte/due/v2/utils/xnet"
	"testing"
)

func TestIP2Long(t *testing.T) {
	str1 := "218.108.212.34"
	ip := xnet.IP2Long(str1)
	str2 := xnet.Long2IP(ip)

	t.Logf("str format: %s", str1)
	t.Logf("long format: %d", ip)
	t.Logf("str format: %s", str2)
}

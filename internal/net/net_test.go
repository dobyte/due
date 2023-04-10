package net_test

import (
	"github.com/dobyte/due/internal/net"
	"testing"
)

func TestParseAddr(t *testing.T) {
	listenAddr, exposeAddr, err := net.ParseAddr(":0")
	if err != nil {
		t.Fatal(err)
	}

	t.Log(listenAddr, exposeAddr)
}

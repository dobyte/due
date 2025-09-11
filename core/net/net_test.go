package net_test

import (
	"fmt"
	"testing"

	"github.com/dobyte/due/v2/core/net"
)

func TestParseAddr(t *testing.T) {
	listenAddr, exposeAddr, err := net.ParseAddr("0.0.0.0:0", true)
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(listenAddr, exposeAddr)
}

func TestInternalIP(t *testing.T) {
	ip, err := net.InternalIP()
	if err != nil {
		t.Fatal(err)
	}

	t.Log(ip)
}

func TestExternalIP(t *testing.T) {
	for range 100 {
		ip, err := net.ExternalIP()
		if err != nil {
			t.Fatal(err)
		}

		fmt.Println(ip)
	}
}

func TestPublicIP(t *testing.T) {
	if ip, err := net.PublicIP(); err != nil {
		t.Fatal(err)
	} else {
		t.Log(ip)
	}

	net.SetPublicIPResolver(customPublicIPResolver)

	if ip, err := net.PublicIP(); err != nil {
		t.Fatal(err)
	} else {
		t.Log(ip)
	}
}

func TestPrivateIP(t *testing.T) {
	if ip, err := net.PrivateIP(); err != nil {
		t.Fatal(err)
	} else {
		t.Log(ip)
	}

	net.SetPrivateIPResolver(customPrivateIPResolver)

	if ip, err := net.PrivateIP(); err != nil {
		t.Fatal(err)
	} else {
		t.Log(ip)
	}
}

func customPublicIPResolver() (string, error) {
	return "1.1.1.1", nil
}

func customPrivateIPResolver() (string, error) {
	return "192.168.1.1", nil
}

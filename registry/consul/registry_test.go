package consul_test

import (
	"context"
	"fmt"
	"github.com/dobyte/due/internal/xnet"
	"github.com/dobyte/due/registry"
	"github.com/dobyte/due/registry/consul"
	"net"
	"testing"
)

const (
	port        = 3553
	serviceName = "node"
)

var reg = consul.NewRegistry()

func TestServe(t *testing.T) {
	ls, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		t.Fatal(err)
	}
	
	go func(ls net.Listener) {
		for {
			conn, err := ls.Accept()
			if err != nil {
				t.Error(err)
				return
			}
			var buff []byte
			if _, err = conn.Read(buff); err != nil {
				t.Error(err)
			}
		}
	}(ls)
	
	select {}
}

func TestRegistry_Register1(t *testing.T) {
	host, err := xnet.ExternalIP()
	if err != nil {
		t.Fatal(err)
	}
	
	if err = reg.Register(context.Background(), &registry.ServiceInstance{
		ID:       "test-1",
		Name:     serviceName,
		Endpoint: fmt.Sprintf("grpc://%s:%d", host, port),
	}); err != nil {
		t.Fatal(err)
	}
	
	select {}
}

func TestRegistry_Register2(t *testing.T) {
	host, err := xnet.ExternalIP()
	if err != nil {
		t.Fatal(err)
	}
	
	if err = reg.Register(context.Background(), &registry.ServiceInstance{
		ID:       "test-2",
		Name:     serviceName,
		Endpoint: fmt.Sprintf("grpc://%s:%d", host, port),
	}); err != nil {
		t.Fatal(err)
	}
	
	select {}
}

func TestRegistry_Services(t *testing.T) {
	services, err := reg.Services(context.Background(), serviceName)
	if err != nil {
		t.Fatal(err)
	}
	
	for _, service := range services {
		t.Logf("%+v", service)
	}
}

func TestRegistry_Watch(t *testing.T) {
	watcher, err := reg.Watch(context.Background(), serviceName)
	if err != nil {
		t.Fatal(err)
	}
	
	for {
		services, err := watcher.Next()
		if err != nil {
			t.Error(err)
		}
		
		for _, service := range services {
			t.Logf("%+v", service)
		}
	}
}

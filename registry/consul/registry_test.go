package consul_test

import (
	"context"
	"fmt"
	"net"
	"testing"
	"time"

	"github.com/dobyte/due/registry/consul/v2"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/errors"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xnet"
)

const (
	port        = 3553
	serviceName = "node"
)

var reg = consul.NewRegistry()

func server(t *testing.T) {
	ls, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		t.Fatal(err)
	}

	t.Cleanup(func() { _ = ls.Close() })

	go func() {
		for {
			conn, err := ls.Accept()
			if err != nil {
				return
			}

			go func(conn net.Conn) {
				defer conn.Close()

				buff := make([]byte, 1024)

				for {
					if _, err := conn.Read(buff); err != nil {
						return
					}
				}
			}(conn)
		}
	}()
}

func TestRegistry_Register(t *testing.T) {
	server(t)

	host, err := xnet.ExternalIP()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	ins := &registry.ServiceInstance{
		ID:       "test-1",
		Name:     serviceName,
		Kind:     cluster.Node.String(),
		Alias:    "mahjong",
		State:    cluster.Work.String(),
		Endpoint: fmt.Sprintf("grpc://%s:%d", host, port),
		Metadata: map[string]string{
			"key": "value",
		},
	}

	for i := range 100 {
		ins.Routes = append(ins.Routes, registry.Route{
			ID:       int32(i),
			Stateful: true,
			Internal: true,
		})
	}

	if err = reg.Register(ctx, ins); err != nil {
		t.Fatal(err)
	}

	// 等待 Consul 完成健康检查后再更新实例状态
	time.Sleep(5 * time.Second)

	ins.State = cluster.Busy.String()
	if err = reg.Register(ctx, ins); err != nil {
		t.Fatal(err)
	}

	time.Sleep(5 * time.Second)
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
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	watcher1, err := reg.Watch(ctx, serviceName)
	if err != nil {
		t.Fatal(err)
	}

	watcher2, err := reg.Watch(context.Background(), serviceName)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		time.Sleep(5 * time.Second)
		watcher1.Stop()
		time.Sleep(5 * time.Second)
		watcher2.Stop()
	}()

	go func() {
		for {
			services, err := watcher1.Next()
			if err != nil {
				if errors.Is(err, errors.ErrWatcherStopped) {
					return
				}

				t.Errorf("goroutine 1: %v", err)
				return
			}

			fmt.Println("goroutine 1: new event entity")

			for _, service := range services {
				t.Logf("goroutine 1: %+v", service)
			}
		}
	}()

	go func() {
		for {
			services, err := watcher2.Next()
			if err != nil {
				if errors.Is(err, errors.ErrWatcherStopped) {
					return
				}

				t.Errorf("goroutine 2: %v", err)
				return
			}

			fmt.Println("goroutine 2: new event entity")

			for _, service := range services {
				t.Logf("goroutine 2: %+v", service)
			}
		}
	}()

	time.Sleep(15 * time.Second)
}

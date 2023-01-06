/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/15 5:37 下午
 * @Desc: TODO
 */

package etcd_test

import (
	"context"
	"fmt"
	"github.com/dobyte/due/cluster"
	"testing"
	"time"

	"github.com/dobyte/due/registry"
	"github.com/dobyte/due/registry/etcd"
	"github.com/dobyte/due/utils/xnet"
)

const (
	port        = 3553
	serviceName = "node"
)

var reg = etcd.NewRegistry()

func TestRegistry_Register1(t *testing.T) {
	host, err := xnet.ExternalIP()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	ins := &registry.ServiceInstance{
		ID:       "test-1",
		Name:     serviceName,
		Kind:     cluster.Node,
		Alias:    "login-server",
		State:    cluster.Work,
		Endpoint: fmt.Sprintf("grpc://%s:%d", host, port),
	}

	rctx, rcancel := context.WithTimeout(ctx, 2*time.Second)
	err = reg.Register(rctx, ins)
	rcancel()
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(2 * time.Second)

	ins.State = cluster.Busy
	rctx, rcancel = context.WithTimeout(ctx, 2*time.Second)
	err = reg.Register(rctx, ins)
	rcancel()
	if err != nil {
		t.Fatal(err)
	} else {
		t.Log("register")
	}

	time.Sleep(20 * time.Second)

	if err = reg.Deregister(ctx, ins); err != nil {
		t.Fatal(err)
	} else {
		t.Log("deregister")
	}

	time.Sleep(40 * time.Second)
}

func TestRegistry_Register2(t *testing.T) {
	host, err := xnet.ExternalIP()
	if err != nil {
		t.Fatal(err)
	}

	if err = reg.Register(context.Background(), &registry.ServiceInstance{
		ID:       "test-2",
		Name:     serviceName,
		Kind:     cluster.Node,
		State:    cluster.Work,
		Endpoint: fmt.Sprintf("grpc://%s:%d", host, port),
	}); err != nil {
		t.Fatal(err)
	}

	go func() {
		time.Sleep(5 * time.Second)
		reg.Stop()
	}()

	time.Sleep(30 * time.Second)
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	watcher1, err := reg.Watch(ctx, serviceName)
	cancel()
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	watcher2, err := reg.Watch(ctx, serviceName)
	cancel()
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		//time.Sleep(5 * time.Second)
		//watcher1.Stop()
		//time.Sleep(5 * time.Second)
		//watcher2.Stop()
		//time.Sleep(5 * time.Second)
		//reg.Stop()
	}()

	go func() {
		for {
			services, err := watcher1.Next()
			if err != nil {
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
				t.Errorf("goroutine 2: %v", err)
				return
			}

			fmt.Println("goroutine 2: new event entity")

			for _, service := range services {
				t.Logf("goroutine 2: %+v", service)
			}
		}
	}()

	time.Sleep(60 * time.Second)
}

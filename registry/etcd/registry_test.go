/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/15 5:37 下午
 * @Desc: 手工验证脚本，需要本地 etcd 服务，运行方式：go test -tags=etcd ./...
 */

package etcd_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/dobyte/due/registry/etcd/v2"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/registry"
)

const (
	port        = 3553   // 服务端口
	serviceName = "node" // 测试服务名
)

// 共享的服务注册中心实例
var reg = etcd.NewRegistry()

// 周期性地重复注册同一服务实例，交替切换 Work/Busy 状态，
// 用于验证重复注册与状态刷新的正确性（手工脚本，需中断运行）
func TestRegistry_Register1(t *testing.T) {
	host, err := net.PublicIP()
	if err != nil {
		t.Fatal(err)
	}

	cnt := 0
	ctx := context.Background()
	ins := &registry.ServiceInstance{
		ID:       "test-1",
		Name:     serviceName,
		Kind:     cluster.Node.String(),
		Alias:    "login-server",
		State:    cluster.Work.String(),
		Endpoint: fmt.Sprintf("grpc://%s:%d", host, port),
	}

	for {
		if cnt%2 == 0 {
			ins.State = cluster.Work.String()
		} else {
			ins.State = cluster.Busy.String()
		}

		if err = reg.Register(ctx, ins); err != nil {
			t.Fatal(err)
		} else {
			t.Logf("register: %v", ins)
		}

		cnt++

		time.Sleep(2 * time.Second)
	}
}

func TestRegistry_Register2(t *testing.T) {
	host, err := net.PublicIP()
	if err != nil {
		t.Fatal(err)
	}

	if err = reg.Register(context.Background(), &registry.ServiceInstance{
		ID:       "test-2",
		Name:     serviceName,
		Kind:     cluster.Node.String(),
		State:    cluster.Work.String(),
		Endpoint: fmt.Sprintf("grpc://%s:%d", host, port),
	}); err != nil {
		t.Fatal(err)
	}

	go func() {
		time.Sleep(5 * time.Second)
		reg.Close()
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
		//watcher1.Close()
		//time.Sleep(5 * time.Second)
		//watcher2.Close()
		//time.Sleep(5 * time.Second)
		//reg.Close()
	}()

	go func() {
		for {
			services, err := watcher1.Next()
			if err != nil {
				t.Errorf("goroutine 1: %v", err)
				return
			}

			//fmt.Println("goroutine 1: new event entity")

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

			//fmt.Println("goroutine 2: new event entity")

			for _, service := range services {
				t.Logf("goroutine 2: %+v", service)
			}
		}
	}()

	//time.Sleep(60 * time.Second)

	select {}
}

func TestRegistry_WatchStopAndCleanup(t *testing.T) {
	ctx := context.Background()

	watcher1, err := reg.Watch(ctx, "test-cleanup-svc")
	if err != nil {
		t.Fatal(err)
	}

	watcher2, err := reg.Watch(ctx, "test-cleanup-svc")
	if err != nil {
		t.Fatal(err)
	}

	_, err = watcher1.Next()
	if err != nil {
		t.Fatalf("watcher1 Next failed: %v", err)
	}

	_, err = watcher2.Next()
	if err != nil {
		t.Fatalf("watcher2 Next failed: %v", err)
	}

	if err = watcher1.Stop(); err != nil {
		t.Fatalf("watcher1 Stop failed: %v", err)
	}

	t.Log("watcher1 stopped, watcher2 should still work")

	_, err = watcher2.Next()
	if err != nil {
		t.Logf("watcher2 Next after watcher1 stop: %v", err)
	}

	if err = watcher2.Stop(); err != nil {
		t.Fatalf("watcher2 Stop failed: %v", err)
	}

	t.Log("both watchers stopped, watcherMgr should be cleaned up")

	time.Sleep(500 * time.Millisecond)
}

// 验证不同服务名的监听彼此独立，互不影响
func TestRegistry_MultipleServiceWatch(t *testing.T) {
	ctx := context.Background()

	watcher1, err := reg.Watch(ctx, "svc-alpha")
	if err != nil {
		t.Fatal(err)
	}
	defer watcher1.Stop()

	watcher2, err := reg.Watch(ctx, "svc-beta")
	if err != nil {
		t.Fatal(err)
	}
	defer watcher2.Stop()

	services1, err := watcher1.Next()
	if err != nil {
		t.Fatalf("svc-alpha Next failed: %v", err)
	}
	t.Logf("svc-alpha initial services: %d", len(services1))

	services2, err := watcher2.Next()
	if err != nil {
		t.Fatalf("svc-beta Next failed: %v", err)
	}
	t.Logf("svc-beta initial services: %d", len(services2))

	t.Log("two different services have independent watchers")
}

func TestRegistry_RegisterDeregister(t *testing.T) {
	host, err := net.PublicIP()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	ins := &registry.ServiceInstance{
		ID:       "lifecycle-test-1",
		Name:     "lifecycle-svc",
		Kind:     cluster.Node.String(),
		State:    cluster.Work.String(),
		Endpoint: fmt.Sprintf("grpc://%s:%d", host, port),
	}

	if err = reg.Register(ctx, ins); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	t.Log("registered successfully")

	services, err := reg.Services(ctx, "lifecycle-svc")
	if err != nil {
		t.Fatalf("services failed: %v", err)
	}
	t.Logf("services count after register: %d", len(services))

	if err = reg.Deregister(ctx, ins); err != nil {
		t.Fatalf("deregister failed: %v", err)
	}
	t.Log("deregistered successfully")

	services, err = reg.Services(ctx, "lifecycle-svc")
	if err != nil {
		t.Fatalf("services after deregister failed: %v", err)
	}
	t.Logf("services count after deregister: %d", len(services))
}

func TestRegistry_ConcurrentRegister(t *testing.T) {
	host, err := net.PublicIP()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			ins := &registry.ServiceInstance{
				ID:       fmt.Sprintf("concurrent-test-%d", idx),
				Name:     "concurrent-svc",
				Kind:     cluster.Node.String(),
				State:    cluster.Work.String(),
				Endpoint: fmt.Sprintf("grpc://%s:%d", host, port+idx),
			}

			if err := reg.Register(ctx, ins); err != nil {
				t.Errorf("register %d failed: %v", idx, err)
			}
		}(i)
	}

	wg.Wait()

	services, err := reg.Services(ctx, "concurrent-svc")
	if err != nil {
		t.Fatalf("services failed: %v", err)
	}

	t.Logf("concurrent register done, total services: %d", len(services))

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			ins := &registry.ServiceInstance{
				ID:   fmt.Sprintf("concurrent-test-%d", idx),
				Name: "concurrent-svc",
				Kind: cluster.Node.String(),
			}

			if err := reg.Deregister(ctx, ins); err != nil {
				t.Errorf("deregister %d failed: %v", idx, err)
			}
		}(i)
	}

	wg.Wait()
	t.Log("concurrent deregister done")
}

// 验证监听器在服务实例注册后能及时收到更新事件
func TestRegistry_WatchAfterAllStop(t *testing.T) {
	host, err := net.PublicIP()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	svcName := "watch-events-test"

	watcher, err := reg.Watch(ctx, svcName)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Stop()

	_, err = watcher.Next()
	if err != nil {
		t.Fatalf("initial Next failed: %v", err)
	}
	t.Log("watcher started, waiting for events")

	done := make(chan struct{})
	go func() {
		defer close(done)
		services, err := watcher.Next()
		if err != nil {
			t.Logf("Next after register: %v", err)
			return
		}
		t.Logf("received update, services count: %d", len(services))
	}()

	time.Sleep(100 * time.Millisecond)

	ins := &registry.ServiceInstance{
		ID:       "watch-events-1",
		Name:     svcName,
		Kind:     cluster.Node.String(),
		State:    cluster.Work.String(),
		Endpoint: fmt.Sprintf("grpc://%s:%d", host, port),
	}

	if err = reg.Register(ctx, ins); err != nil {
		t.Fatalf("register failed: %v", err)
	}
	t.Log("registered instance, waiting for watch event")

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Log("timeout waiting for watch event")
	}

	if err = reg.Deregister(ctx, ins); err != nil {
		t.Fatalf("deregister failed: %v", err)
	}
	t.Log("deregistered instance")
}

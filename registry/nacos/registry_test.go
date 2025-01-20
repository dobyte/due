/**
 * @Author: fuxiao
 * @Email: 576101059@qq.com
 * @Date: 2022/9/15 5:37 下午
 * @Desc: TODO
 */

package nacos_test

import (
	"context"
	"fmt"
	"github.com/dobyte/due/registry/nacos/v2"
	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/net"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/registry"
	"github.com/dobyte/due/v2/utils/xconv"
	"golang.org/x/sync/errgroup"
	"testing"
	"time"
)

const (
	port        = 3553
	serviceName = "gate"
)

var reg = nacos.NewRegistry()

func TestRegistry_Register1(t *testing.T) {
	host, err := net.ExternalIP()
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()
	ins := &registry.ServiceInstance{
		ID:       "test-11",
		Name:     serviceName,
		Kind:     cluster.Node.String(),
		Alias:    "login-server",
		State:    cluster.Work.String(),
		Endpoint: fmt.Sprintf("grpc://%s:%d", host, port),
	}

	rctx, rcancel := context.WithTimeout(ctx, 2*time.Second)
	err = reg.Register(rctx, ins)
	rcancel()
	if err != nil {
		t.Fatal(err)
	}

	time.Sleep(10 * time.Second)

	ins.State = cluster.Busy.String()
	rctx, rcancel = context.WithTimeout(ctx, 2*time.Second)
	err = reg.Register(rctx, ins)
	rcancel()
	if err != nil {
		t.Fatal(err)
	} else {
		t.Log("register")
	}

	time.Sleep(40 * time.Second)

	if err = reg.Deregister(ctx, ins); err != nil {
		t.Fatal(err)
	} else {
		t.Log("deregister")
	}

	time.Sleep(40 * time.Second)
}

func TestRegistry_Register2(t *testing.T) {
	host, err := net.ExternalIP()
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

func TestMultipleNodeRegister(t *testing.T) {
	for i := 0; i < 5; i++ {
		go func(i int) {
			n := newNode(xconv.String(i))
			n.start()
		}(i)
	}

	time.Sleep(10 * time.Second)
}

const (
	defaultTimeout = 3 * time.Second // 默认超时时间
)

type node struct {
	id        string
	ctx       context.Context
	registry  registry.Registry
	instances []*registry.ServiceInstance
}

func newNode(id string) *node {
	n := &node{}
	n.id = id
	n.ctx = context.Background()
	n.registry = nacos.NewRegistry()
	n.instances = make([]*registry.ServiceInstance, 0)

	n.instances = append(n.instances, &registry.ServiceInstance{
		ID:    id,
		Name:  cluster.Node.String(),
		Kind:  cluster.Node.String(),
		Alias: fmt.Sprintf("node-%s", id),
		State: cluster.Work.String(),
		Routes: []registry.Route{
			{ID: 1, Stateful: true, Internal: false},
			{ID: 2, Stateful: true, Internal: false},
			{ID: 3, Stateful: true, Internal: false},
			{ID: 4, Stateful: true, Internal: false},
		},
		Endpoint: fmt.Sprintf("grpc://%s:%d", id, port),
	})

	return n
}

func (n *node) start() {
	n.watch()

	if err := n.register(); err != nil {
		log.Fatalf("register cluster instances failed: %v", err)
	}

}

// 执行注册操作
func (n *node) register() error {
	eg, ctx := errgroup.WithContext(n.ctx)

	for i := range n.instances {
		instance := n.instances[i]
		eg.Go(func() error {
			ctx, cancel := context.WithTimeout(ctx, defaultTimeout)
			defer cancel()
			return n.registry.Register(ctx, instance)
		})
	}

	return eg.Wait()
}

func (n *node) watch() {
	ctx, cancel := context.WithTimeout(n.ctx, 3*time.Second)
	watcher, err := n.registry.Watch(ctx, cluster.Node.String())
	cancel()
	if err != nil {
		log.Fatalf("the dispatcher instance watch failed: %v", err)
	}

	go func() {
		defer watcher.Stop()
		for {
			select {
			case <-n.ctx.Done():
				return
			default:
				// exec watch
			}

			services, err := watcher.Next()
			if err != nil {
				continue
			}

			fmt.Printf("node: %v services: %v\n", n.id, len(services))

			for _, service := range services {
				fmt.Printf("service id: %v\n", service.ID)
			}

			fmt.Println()
		}
	}()
}

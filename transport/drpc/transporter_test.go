package drpc_test

import (
	"context"
	"fmt"
	endpoints "github.com/dobyte/due/v2/core/endpoint"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/transport"
	"github.com/dobyte/due/v2/transport/drpc"
	"github.com/dobyte/due/v2/utils/xuuid"
	"testing"
	"time"
)

func TestTransporter_NewGateServer(t *testing.T) {
	transporter := drpc.NewTransporter(drpc.WithServerListenAddr(":3779"))

	server, err := transporter.NewGateServer(&gateProvider{})
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(server.Endpoint())

	if err = server.Start(); err != nil {
		t.Fatal(err)
	}
}

func TestTransporter_NewGateClient(t *testing.T) {
	transporter := drpc.NewTransporter()

	ep := endpoints.NewEndpoint("drpc", "127.0.0.1:3779", false)

	client, err := transporter.NewGateClient(ep)
	if err != nil {
		t.Fatal(err)
	}

	//for i := 0; i < 2; i++ {
	//	go func() {
	//		miss, err := client.Bind(context.Background(), 1, 2)
	//		if err != nil {
	//			t.Fatal(err)
	//		}
	//
	//		t.Log(miss)
	//	}()
	//}

	_, err = client.Push(context.Background(), session.Conn, 1, &packet.Message{
		Seq:    1,
		Route:  1,
		Buffer: []byte("hello world"),
	})
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("push ok")

	time.Sleep(10 * time.Second)
}

func TestTransporter_NewNodeServer(t *testing.T) {
	transporter := drpc.NewTransporter(drpc.WithServerListenAddr(":3778"))

	server, err := transporter.NewNodeServer(&nodeProvider{})
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(server.Endpoint())

	if err = server.Start(); err != nil {
		t.Fatal(err)
	}
}

func TestTransporter_NewNodeClient(t *testing.T) {
	transporter := drpc.NewTransporter()

	ep := endpoints.NewEndpoint("drpc", "127.0.0.1:3778", false)

	client, err := transporter.NewNodeClient(ep)
	if err != nil {
		t.Fatal(err)
	}

	_, err = client.Deliver(context.Background(), &transport.DeliverArgs{
		GID: xuuid.UUID(),
		CID: 10,
		UID: 20,
		Message: &packet.Message{
			Seq:    1,
			Route:  1,
			Buffer: []byte("hello world"),
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println("deliver ok")

	time.Sleep(10 * time.Second)
}

type gateProvider struct {
}

// Bind 绑定用户与网关间的关系
func (p *gateProvider) Bind(ctx context.Context, cid, uid int64) error {
	return nil
}

// Unbind 解绑用户与网关间的关系
func (p *gateProvider) Unbind(ctx context.Context, uid int64) error {
	return nil
}

// GetIP 获取客户端IP地址
func (p *gateProvider) GetIP(ctx context.Context, kind session.Kind, target int64) (ip string, err error) {
	return
}

// IsOnline 检测是否在线
func (p *gateProvider) IsOnline(ctx context.Context, kind session.Kind, target int64) (isOnline bool, err error) {
	return
}

// Push 发送消息（异步）
func (p *gateProvider) Push(ctx context.Context, kind session.Kind, target int64, message *packet.Message) error {
	//fmt.Println(kind, target, message)

	return nil
}

// Multicast 推送组播消息（异步）
func (p *gateProvider) Multicast(ctx context.Context, kind session.Kind, targets []int64, message *packet.Message) (total int64, err error) {
	return
}

// Broadcast 推送广播消息（异步）
func (p *gateProvider) Broadcast(ctx context.Context, kind session.Kind, message *packet.Message) (total int64, err error) {
	return
}

// Stat 统计会话总数
func (p *gateProvider) Stat(ctx context.Context, kind session.Kind) (total int64, err error) {
	return
}

// Disconnect 断开连接
func (p *gateProvider) Disconnect(ctx context.Context, kind session.Kind, target int64, isForce bool) error {
	return nil
}

type nodeProvider struct {
}

// Trigger 触发事件
func (p *nodeProvider) Trigger(ctx context.Context, args *transport.TriggerArgs) (miss bool, err error) {
	return
}

// Deliver 投递消息
func (p *nodeProvider) Deliver(ctx context.Context, args *transport.DeliverArgs) (miss bool, err error) {
	fmt.Printf("%+v", args)

	return
}

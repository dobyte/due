package drpc_test

import (
	"context"
	"fmt"
	"github.com/dobyte/due/v2/packet"
	"github.com/dobyte/due/v2/session"
	"github.com/dobyte/due/v2/transport/drpc"
	"testing"
	"time"
)

func TestTransporter_NewGateServer(t *testing.T) {
	transporter := drpc.NewTransporter()

	server, err := transporter.NewGateServer(&gateProvider{})
	if err != nil {
		t.Fatal(err)
	}

	fmt.Println(server.Addr())

	if err = server.Start(); err != nil {
		t.Fatal(err)
	}
}

func TestTransporter_NewGateClient(t *testing.T) {
	transporter := drpc.NewTransporter()

	client, err := transporter.NewGateClient(nil)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 2; i++ {
		go func() {
			miss, err := client.Bind(context.Background(), 1, 2)
			if err != nil {
				t.Fatal(err)
			}

			t.Log(miss)
		}()
	}

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

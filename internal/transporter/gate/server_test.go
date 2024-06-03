package gate_test

import (
	"context"
	"fmt"
	"github.com/dobyte/due/v2/internal/transporter/gate"
	"github.com/dobyte/due/v2/session"
	"testing"
)

func TestServer(t *testing.T) {
	server, err := gate.NewServer(":49899", &provider{})
	if err != nil {
		t.Fatal(err)
	}

	t.Logf("server listen on: %s", server.Addr())

	if err = server.Start(); err != nil {
		t.Fatal(err)
	}
}

type provider struct {
}

// Bind 绑定用户与网关间的关系
func (p *provider) Bind(ctx context.Context, cid, uid int64) error {
	return nil
}

// Unbind 解绑用户与网关间的关系
func (p *provider) Unbind(ctx context.Context, uid int64) error {
	return nil
}

// GetIP 获取客户端IP地址
func (p *provider) GetIP(ctx context.Context, kind session.Kind, target int64) (ip string, err error) {
	fmt.Println(kind, target)
	ip = "192.168.0.88"
	return
}

// IsOnline 检测是否在线
func (p *provider) IsOnline(ctx context.Context, kind session.Kind, target int64) (isOnline bool, err error) {
	return
}

// Push 发送消息（异步）
func (p *provider) Push(ctx context.Context, kind session.Kind, target int64, message []byte) error {
	//fmt.Println(kind, target, message)

	return nil
}

// Multicast 推送组播消息（异步）
func (p *provider) Multicast(ctx context.Context, kind session.Kind, targets []int64, message []byte) (total int64, err error) {
	return
}

// Broadcast 推送广播消息（异步）
func (p *provider) Broadcast(ctx context.Context, kind session.Kind, message []byte) (total int64, err error) {
	return
}

// Stat 统计会话总数
func (p *provider) Stat(ctx context.Context, kind session.Kind) (total int64, err error) {
	return
}

// Disconnect 断开连接
func (p *provider) Disconnect(ctx context.Context, kind session.Kind, target int64, isForce bool) error {
	return nil
}

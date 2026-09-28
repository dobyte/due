package drpc

import (
	"context"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/dobyte/due/v2/cluster"
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/internal/transporter/internal/codes"
	"github.com/dobyte/due/v2/internal/transporter/internal/protocol"
	"github.com/dobyte/due/v2/internal/transporter/internal/route"
)

// setHeartbeatInterval 注入测试用的心跳间隔，测试结束时恢复原值（原子访问，与心跳协程无竞态）
// @param t *testing.T 测试对象
// @param d time.Duration 心跳间隔
func setHeartbeatInterval(t *testing.T, d time.Duration) {
	t.Helper()

	old := heartbeatInterval.Swap(int64(d))

	t.Cleanup(func() { heartbeatInterval.Store(old) })
}

// frameStats 统计裸TCP接收端收到的帧数
type frameStats struct {
	heartbeat atomic.Int32 // 心跳帧数
	data      atomic.Int32 // 数据帧数
}

// startRawReceiver 启动裸TCP接收端：响应握手后持续统计接收到的帧
// @param t *testing.T 测试对象
// @param addr string 监听地址
// @return @1 *frameStats 帧统计
func startRawReceiver(t *testing.T, addr string) *frameStats {
	t.Helper()

	stats := &frameStats{}

	listener, err := net.Listen("tcp", addr)
	if err != nil {
		t.Fatalf("listen failed: %v", err)
	}

	t.Cleanup(func() { _ = listener.Close() })

	go func() {
		conn, err := listener.Accept()
		if err != nil {
			return
		}

		tcpConn := conn.(*net.TCPConn)
		reader := newReader(tcpConn)

		// 读取并响应握手请求
		isHeartbeat, rt, seq, buf, err := reader.read()
		if err != nil || isHeartbeat || rt != route.Handshake {
			return
		}
		buf.Release()

		res := protocol.EncodeHandshakeRes(seq, codes.OK)
		if _, err = tcpConn.Write(res.Bytes()); err != nil {
			res.Release()
			return
		}
		res.Release()

		for {
			isHeartbeat, _, _, buf, err = reader.read()
			if err != nil {
				return
			}

			if isHeartbeat {
				stats.heartbeat.Add(1)
			} else {
				stats.data.Add(1)
				buf.Release()
			}
		}
	}()

	return stats
}

// newTestClient 创建测试客户端
// @param t *testing.T 测试对象
// @param addr string 服务端地址
// @return @1 *Client 客户端实例
func newTestClient(t *testing.T, addr string) *Client {
	t.Helper()

	cli, err := NewClient(addr, &ClientOptions{
		ID:             "test-gate",
		Kind:           cluster.Gate,
		ConnNum:        1,
		DialTimeout:    time.Second,
		DialRetryTimes: 3,
	})
	if err != nil {
		t.Fatalf("create client failed: %v", err)
	}

	if err = cli.Establish(); err != nil {
		t.Fatalf("establish connection failed: %v", err)
	}

	return cli
}

// TestClientConnIdleHeartbeat 空闲连接必须按间隔发送心跳以维持对端活性判定
func TestClientConnIdleHeartbeat(t *testing.T) {
	setHeartbeatInterval(t, 100*time.Millisecond)

	addr := listenFreeAddr(t)
	stats := startRawReceiver(t, addr)

	cli := newTestClient(t, addr)
	defer cli.Close()

	deadline := time.Now().Add(2 * time.Second)
	for stats.heartbeat.Load() == 0 {
		if time.Now().After(deadline) {
			t.Fatal("expect heartbeat on idle connection")
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// TestClientConnHeartbeatSuppressedByTraffic 数据流持续期间心跳应被抑制，数据帧不受影响
func TestClientConnHeartbeatSuppressedByTraffic(t *testing.T) {
	setHeartbeatInterval(t, 100*time.Millisecond)

	addr := listenFreeAddr(t)
	stats := startRawReceiver(t, addr)

	cli := newTestClient(t, addr)
	defer cli.Close()

	const pushes = 20
	for i := 0; i < pushes; i++ {
		err := cli.Push(context.Background(), protocol.EncodeDeliverReq(0, int64(i+1), 0, buffer.NewBytes(nil)), int64(i+1))
		if err != nil {
			t.Fatalf("push failed: %v", err)
		}
		time.Sleep(20 * time.Millisecond)
	}

	// 等待全部数据帧到达
	deadline := time.Now().Add(2 * time.Second)
	for stats.data.Load() < pushes {
		if time.Now().After(deadline) {
			t.Fatalf("expect %d data frames, got %d", pushes, stats.data.Load())
		}
		time.Sleep(10 * time.Millisecond)
	}

	// 数据流持续期间心跳应被抑制（容忍建立连接初期的竞态心跳）
	if n := stats.heartbeat.Load(); n > 1 {
		t.Fatalf("heartbeat should be suppressed by traffic, got %d", n)
	}
}

// TestServerHeartbeatTimeout 静默连接应在2倍心跳间隔后被服务端强制关闭
func TestServerHeartbeatTimeout(t *testing.T) {
	setHeartbeatInterval(t, 100*time.Millisecond)

	addr := listenFreeAddr(t)

	s, err := NewServer(&ServerOptions{Addr: addr})
	if err != nil {
		t.Fatalf("create server failed: %v", err)
	}
	if err = s.Start(); err != nil {
		t.Fatalf("start server failed: %v", err)
	}
	defer s.Stop()

	// 连接保持静默（不握手、不发帧），服务端活性检查应将其强制关闭
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		t.Fatalf("dial failed: %v", err)
	}
	defer conn.Close()

	// 先等待连接进入服务端连接表，避免在accept完成前误判
	deadline := time.Now().Add(2 * time.Second)
	for {
		n := 0
		s.conns.Range(func(_, _ any) bool { n++; return true })

		if n == 1 {
			break
		}

		if time.Now().After(deadline) {
			t.Fatal("conn was not accepted by the server")
		}

		time.Sleep(10 * time.Millisecond)
	}

	deadline = time.Now().Add(2 * time.Second)
	for {
		n := 0
		s.conns.Range(func(_, _ any) bool { n++; return true })

		if n == 0 {
			break
		}

		if time.Now().After(deadline) {
			t.Fatalf("idle conn was not closed after heartbeat timeout, %d conns left", n)
		}

		time.Sleep(20 * time.Millisecond)
	}
}

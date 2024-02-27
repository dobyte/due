package gnet

import (
	"github.com/panjf2000/gnet/v2"
	"github.com/sasha-s/go-deadlock"
	"github.com/symsimmy/due/errors"
	"github.com/symsimmy/due/network"
)

const sizeBytes = 2
const sizeMax = int32(^(uint32(0)) >> 1)

const (
	closeSig   int = iota // 关闭信号
	dataPacket            // 数据包
)

type chWrite struct {
	typ int
	msg []byte
}

var (
	ErrIncompletePacket = errors.New("incomplete packet")
)

const defaultWriteChannelSize = 1024

type serverConn struct {
	rw                deadlock.RWMutex // 锁
	id                int64            // 连接ID
	uid               int64            // 用户ID
	state             int32            // 连接状态
	conn              gnet.Conn        // 源连接
	connMgr           *serverConnMgr   // 连接管理
	chWrite           chan chWrite     // 写入队列
	done              chan struct{}    // 写入完成信号
	lastHeartbeatTime int64            // 上次心跳时间
	close             chan struct{}    // 关闭信号
	rBlock            chan struct{}
	rRelease          chan struct{}
	wBlock            chan struct{}
	wRelease          chan struct{}
}

var _ network.Conn = &serverConn{}

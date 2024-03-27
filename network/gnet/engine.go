package gnet

import (
	"context"
	"github.com/panjf2000/gnet/v2"
	"time"
)

type engine struct {
	gnet.BuiltinEventEngine
	server  *server        // 服务器
	engine  *gnet.Engine   // 网络引擎
	connMgr *serverConnMgr // 连接管理器
	counter int32
}

// OnBoot 引擎启动
func (e *engine) OnBoot(engine gnet.Engine) gnet.Action {
	e.engine = &engine

	if e.server.startHandler != nil {
		e.server.startHandler()
	}

	return gnet.None
}

func (e *engine) OnShutdown(engine gnet.Engine) {
	if e.server.stopHandler != nil {
		e.server.stopHandler()
	}
}

// OnOpen 打开连接
func (e *engine) OnOpen(conn gnet.Conn) ([]byte, gnet.Action) {
	if err := e.connMgr.allocate(conn); err != nil {
		return nil, gnet.Close
	}

	return nil, gnet.None
}

// OnClose 关闭连接
func (e *engine) OnClose(conn gnet.Conn, err error) gnet.Action {
	e.connMgr.destroy(conn)

	return gnet.None
}

// OnTraffic 接受消息
func (e *engine) OnTraffic(c gnet.Conn) gnet.Action {
	if conn, ok := e.connMgr.load(c); ok {
		if err := conn.read(); err == nil {
			return gnet.None
		}
	}

	return gnet.None
}

// OnTick 定时器
func (e *engine) OnTick() (time.Duration, gnet.Action) {
	return 0, gnet.None

	//e.connMgr.checkHeartbeat()
	//
	//return e.server.opts.heartbeatInterval, gnet.None
}

// 停止引擎
func (e *engine) stop(ctx context.Context) error {
	if e.engine == nil {
		return nil
	}

	return e.engine.Stop(ctx)
}

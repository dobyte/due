package drpc

import (
	"github.com/dobyte/due/v2/core/buffer"
	"github.com/dobyte/due/v2/log"
)

// serverTask 待处理的消息任务
type serverTask struct {
	conn  *ServerConn
	route uint8
	seq   uint64
	buf   *buffer.Bytes
}

// ServerWorker 工作协程
type ServerWorker struct {
	svr   *Server
	tasks chan *serverTask
}

// run 运行工作协程
// 从任务队列中取出消息并处理，直至队列关闭
func (w *ServerWorker) run() {
	defer w.svr.workerWg.Done()

	for task := range w.tasks {
		if err := w.svr.handleMessage(task.conn, task.route, task.seq, task.buf); err != nil {
			log.Warnf("handle message error: %v", err)
		}
	}
}

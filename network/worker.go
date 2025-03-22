package network

import (
	"sync"
)

type WorkerPool struct {
	opts          *workerOptions
	workers       map[int32]struct{}
	workerMu      sync.Mutex
	taskQueue     []chan ConnTask
	extraWorkers  map[int32]struct{}
	extraWorkerMu sync.Mutex
}

func (wp *WorkerPool) IsOpen() bool {
	return wp.opts.workerNum > 0
}

func (wp *WorkerPool) isOpenExtra() bool {
	return wp.IsOpen() && wp.opts.workerNum < wp.opts.maxWorkerNum
}

func (wp *WorkerPool) StartWorkerPool(fn ReceiveHandler) {
	if !wp.IsOpen() {
		return
	}
	for i := int32(0); i < wp.opts.workerNum; i++ {
		wp.taskQueue[i] = make(chan ConnTask, wp.opts.taskNum)
		go wp.doWork(wp.taskQueue[i], fn)
	}
}
func (wp *WorkerPool) doWork(taskQueue chan ConnTask, fn ReceiveHandler) {
	if fn == nil {
		return
	}
	for {
		select {
		case ts, ok := <-taskQueue:
			if !ok {
				return
			}
			fn(ts.GetConn(), ts.GetMsg())
		}
	}
}
func (wp *WorkerPool) stopWork(workerID int32) {
	// 关闭掉对应的任务队列即可
	close(wp.taskQueue[workerID])
}

func RecycleWorker(conn Conn) {
	workerID := conn.GetWorkerID()
	wp := conn.GetWorkerPool()
	if wp == nil || !wp.IsOpen() {
		return
	}
	if workerID < wp.opts.workerNum {
		wp.workerMu.Lock()
		wp.workers[workerID] = struct{}{}
		wp.workerMu.Unlock()
	} else {
		// 说明是扩展工作池归还时要停止这个worker
		wp.workerMu.Lock()
		wp.stopWork(workerID)
		wp.extraWorkers[workerID] = struct{}{}
		wp.workerMu.Unlock()
	}

}

func BindWorker(conn Conn) (workerID int32) {
	wp := conn.GetWorkerPool()
	if !wp.IsOpen() {
		return
	}
	wp.workerMu.Lock()
	for workerId, _ := range wp.workers {
		delete(wp.workers, workerId)
		wp.workerMu.Unlock()
		return workerId
	}
	wp.workerMu.Unlock()
	if wp.isOpenExtra() {
		wp.extraWorkerMu.Lock()
		defer wp.extraWorkerMu.Unlock()
		for workerId, _ := range wp.extraWorkers {
			delete(wp.extraWorkers, workerId)
			return workerId
		}
	}
	if wp.opts.workerNum == 0 {
		workerID = 0
	} else {
		workerID = int32(conn.ID() % int64(wp.opts.workerNum))
	}
	return
}

func (wp *WorkerPool) AddTask(task ConnTask) {
	wp.taskQueue[task.GetConn().GetWorkerID()] <- task
}
func NewWorkerPool(opts ...WorkerOption) *WorkerPool {
	o := defaultWorkerOptions()
	for _, opt := range opts {
		opt(o)
	}
	wp := &WorkerPool{}
	wp.opts = o
	wp.workers = make(map[int32]struct{})
	wp.workerMu.Lock()
	for i := int32(0); i < wp.opts.workerNum; i++ {
		wp.workers[i] = struct{}{}
	}
	taskQueueSize := wp.opts.workerNum
	wp.workerMu.Unlock()
	if wp.isOpenExtra() {
		wp.extraWorkers = make(map[int32]struct{})
		wp.extraWorkerMu.Lock()
		for i := wp.opts.workerNum; i < wp.opts.maxWorkerNum; i++ {
			wp.extraWorkers[i] = struct{}{}
		}
		wp.extraWorkerMu.Unlock()
		taskQueueSize = wp.opts.maxWorkerNum
	}
	wp.taskQueue = make([]chan ConnTask, taskQueueSize)
	return wp
}

package network

import "github.com/dobyte/due/v2/etc"

const (
	defaultWorkerNum    = 0
	defaultMaxWorkerNum = 5000
	defaultTaskNum      = 50
)
const (
	defaultWorkerNumKey    = "etc.network.worker.workerNum"
	defaultMaxWorkerNumKey = "etc.network.worker.maxWorkerNum"
	defaultTaskNumKey      = "etc.network.worker.taskNum"
)

type WorkerOption func(o *workerOptions)

type workerOptions struct {
	workerNum    int32 // 工作池个数,0的话为不开启工作池
	maxWorkerNum int32 // 最大工作池个数
	taskNum      int32 // 每个任务队列任务个数,默认50
}

func defaultWorkerOptions() *workerOptions {
	return &workerOptions{
		workerNum:    etc.Get(defaultWorkerNumKey, defaultWorkerNum).Int32(),
		maxWorkerNum: etc.Get(defaultMaxWorkerNumKey, defaultMaxWorkerNum).Int32(),
		taskNum:      etc.Get(defaultTaskNumKey, defaultTaskNum).Int32(),
	}
}

// WithWorkerNum 设置工作池个数
func WithWorkerNum(workerNum int32) WorkerOption {
	return func(o *workerOptions) { o.workerNum = workerNum }
}

// WithMaxWorkerNum 设置最大工作池个数
func WithMaxWorkerNum(maxWorkerNum int32) WorkerOption {
	return func(o *workerOptions) { o.maxWorkerNum = maxWorkerNum }
}

// WithTaskNum 设置任务队列任务个数
func WithTaskNum(taskNum int32) WorkerOption {
	return func(o *workerOptions) { o.taskNum = taskNum }
}

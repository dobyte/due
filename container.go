package due

import (
	"context"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"

	"github.com/dobyte/due/v2/cache"
	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/core/info"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/eventbus"
	"github.com/dobyte/due/v2/lock"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/task"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/dobyte/due/v2/utils/xos"
)

const (
	defaultPIDKey                 = "etc.pid"                 // 进程文件路径
	defaultShutdownMaxWaitTimeKey = "etc.shutdownMaxWaitTime" // 容器关闭最大等待时间
)

type Container struct {
	components []component.Component
}

// NewContainer 创建一个容器
// @return @1 *Container 容器实例
func NewContainer() *Container {
	return &Container{}
}

// Add 添加组件
// @param components ...component.Component 待添加的组件
func (c *Container) Add(components ...component.Component) {
	c.components = append(c.components, components...)
}

// Serve 启动容器
// 依次初始化并启动所有组件；在等待系统信号后，关闭并销毁组件，最终清理相关模块
// @param isNonWaitSignal ...bool 是否不等待系统信号，true 时启动完成后直接进入关闭流程
func (c *Container) Serve(isNonWaitSignal ...bool) {
	c.doPrintFrameworkInfo()

	c.doInitComponents()

	c.doStartComponents()

	c.doSaveProcessID()

	if len(isNonWaitSignal) == 0 || !isNonWaitSignal[0] {
		c.doWaitSystemSignal()
	}

	c.doCloseComponents()

	c.doDestroyComponents()

	c.doRemoveProcessID()

	c.doClearModules()
}

// doInitComponents 初始化所有组件
func (c *Container) doInitComponents() {
	for _, comp := range c.components {
		comp.Init()
	}
}

// doStartComponents 启动所有组件
func (c *Container) doStartComponents() {
	for _, comp := range c.components {
		comp.Start()
	}
}

// doCloseComponents 关闭所有组件
// 所有组件在独立协程中并发关闭，整体受 etc.shutdownMaxWaitTime 超时控制
func (c *Container) doCloseComponents() {
	g := xcall.NewGoroutines()

	for _, comp := range c.components {
		g.Add(comp.Close)
	}

	g.Run(context.Background(), etc.Get(defaultShutdownMaxWaitTimeKey).Duration())
}

// doDestroyComponents 销毁所有组件
// 所有组件在独立协程中并发销毁，整体受 5 秒超时控制
func (c *Container) doDestroyComponents() {
	g := xcall.NewGoroutines()

	for _, comp := range c.components {
		g.Add(comp.Destroy)
	}

	g.Run(context.Background(), 5*time.Second)
}

// doWaitSystemSignal 等待系统信号
// 阻塞等待进程退出信号，收到后停止监听并记录日志
func (c *Container) doWaitSystemSignal() {
	sig := make(chan os.Signal, 1)

	switch runtime.GOOS {
	case `windows`:
		signal.Notify(sig, os.Interrupt)
	default:
		signal.Notify(sig, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGABRT, syscall.SIGTERM)
	}

	s := <-sig

	signal.Stop(sig)

	log.Warnf("process got signal %v, container will close", s)
}

// doClearModules 清理所有模块
func (c *Container) doClearModules() {
	if err := eventbus.Close(); err != nil {
		log.Warnf("eventbus close failed: %v", err)
	}

	if err := lock.Close(); err != nil {
		log.Warnf("lock-maker close failed: %v", err)
	}

	if err := cache.Close(); err != nil {
		log.Warnf("cache close failed: %v", err)
	}

	task.Release()

	config.Close()

	etc.Close()

	log.Close()
}

// doSaveProcessID 保存进程号
func (c *Container) doSaveProcessID() {
	filename := etc.Get(defaultPIDKey).String()
	if filename == "" {
		return
	}

	if err := xos.WriteFile(filename, []byte(strconv.Itoa(syscall.Getpid()))); err != nil {
		log.Fatalf("pid save failed: %v", err)
	}
}

// doRemoveProcessID 删除进程号文件
func (c *Container) doRemoveProcessID() {
	filename := etc.Get(defaultPIDKey).String()
	if filename == "" {
		return
	}

	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		log.Warnf("pid file remove failed: %v", err)
	}
}

// doPrintFrameworkInfo 打印框架信息
func (c *Container) doPrintFrameworkInfo() {
	info.PrintFrameworkInfo()

	info.PrintGlobalInfo()
}

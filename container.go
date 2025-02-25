package due

import (
	"context"
	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/core/info"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/eventbus"
	"github.com/dobyte/due/v2/lock"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/task"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/dobyte/due/v2/utils/xfile"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
	"time"
)

const (
	defaultPIDKey                 = "etc.pid"                 // 进程文件路径
	defaultShutdownMaxWaitTimeKey = "etc.shutdownMaxWaitTime" // 容器关闭最大等待时间
)

type Container struct {
	components []component.Component
}

// NewContainer 创建一个容器
func NewContainer() *Container {
	return &Container{}
}

// Add 添加组件
func (c *Container) Add(components ...component.Component) {
	c.components = append(c.components, components...)
}

// Serve 启动容器
func (c *Container) Serve() {
	c.doSaveProcessID()

	c.doPrintFrameworkInfo()

	c.doInitComponents()

	c.doStartComponents()

	c.doWaitSystemSignal()

	c.doCloseComponents()

	c.doDestroyComponents()

	c.doClearModules()
}

// 初始化所有组件
func (c *Container) doInitComponents() {
	for _, comp := range c.components {
		comp.Init()
	}
}

// 启动所有组件
func (c *Container) doStartComponents() {
	for _, comp := range c.components {
		comp.Start()
	}
}

// 关闭所有组件
func (c *Container) doCloseComponents() {
	g := xcall.NewGoroutines()

	for _, comp := range c.components {
		g.Add(comp.Close)
	}

	g.Run(context.Background(), etc.Get(defaultShutdownMaxWaitTimeKey).Duration())
}

// 销毁所有组件
func (c *Container) doDestroyComponents() {
	g := xcall.NewGoroutines()

	for _, comp := range c.components {
		g.Add(comp.Destroy)
	}

	g.Run(context.Background(), 5*time.Second)
}

// 等待系统信号
func (c *Container) doWaitSystemSignal() {
	sig := make(chan os.Signal)

	switch runtime.GOOS {
	case `windows`:
		signal.Notify(sig, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)
	default:
		signal.Notify(sig, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGABRT, syscall.SIGKILL, syscall.SIGTERM)
	}

	s := <-sig

	signal.Stop(sig)

	log.Warnf("process got signal %v, container will close", s)
}

// 清理所有模块
func (c *Container) doClearModules() {
	if err := eventbus.Close(); err != nil {
		log.Warnf("eventbus close failed: %v", err)
	}

	if err := lock.Close(); err != nil {
		log.Warnf("lock-maker close failed: %v", err)
	}

	task.Release()

	config.Close()

	etc.Close()

	log.Close()
}

// 保存进程号
func (c *Container) doSaveProcessID() {
	filename := etc.Get(defaultPIDKey).String()
	if filename == "" {
		return
	}

	if err := xfile.WriteFile(filename, []byte(strconv.Itoa(syscall.Getpid()))); err != nil {
		log.Fatalf("pid save failed: %v", err)
	}
}

// 打印框架信息
func (c *Container) doPrintFrameworkInfo() {
	info.PrintFrameworkInfo()

	info.PrintGlobalInfo()
}

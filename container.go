package due

import (
	"github.com/dobyte/due/v2/component"
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/core/info"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/eventbus"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/task"
	"github.com/dobyte/due/v2/utils/xfile"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"syscall"
)

type Container struct {
	sig        chan os.Signal
	components []component.Component
}

// NewContainer 创建一个容器
func NewContainer() *Container {
	return &Container{sig: make(chan os.Signal)}
}

// Add 添加组件
func (c *Container) Add(components ...component.Component) {
	c.components = append(c.components, components...)
}

// Serve 启动容器
func (c *Container) Serve() {
	for _, comp := range c.components {
		comp.Init()
	}

	info.PrintFrameworkInfo()

	info.PrintGlobalInfo()

	for _, comp := range c.components {
		comp.Start()
	}

	c.doSavePID()

	switch runtime.GOOS {
	case `windows`:
		signal.Notify(c.sig, syscall.SIGINT, syscall.SIGKILL, syscall.SIGTERM)
	default:
		signal.Notify(c.sig, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGABRT, syscall.SIGKILL, syscall.SIGTERM)
	}

	sig := <-c.sig

	log.Warnf("process got signal %v, container will close", sig)

	signal.Stop(c.sig)

	for _, comp := range c.components {
		comp.Destroy()
	}

	if err := eventbus.Close(); err != nil {
		log.Warnf("eventbus close failed: %v", err)
	}

	task.Release()

	config.Close()

	etc.Close()

	log.Close()
}

func (c *Container) doSavePID() {
	filename := etc.Get("etc.pid").String()
	if filename == "" {
		return
	}

	err := xfile.WriteFile(filename, []byte(strconv.Itoa(syscall.Getpid())))
	if err != nil {
		log.Fatalf("pid save failed: %v", err)
	}
}

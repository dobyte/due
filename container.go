package due

import (
	"github.com/dobyte/due/component"
	"github.com/dobyte/due/log"
	"github.com/dobyte/due/mode"

	"os"
	"os/signal"
	"syscall"
)

type Container struct {
	sig        chan os.Signal
	die        chan struct{}
	components []component.Component
}

// NewContainer 创建一个容器
func NewContainer() *Container {
	return &Container{sig: make(chan os.Signal), die: make(chan struct{})}
}

// Add 添加组件
func (c *Container) Add(components ...component.Component) {
	c.components = append(c.components, components...)
}

// Serve 启动容器
func (c *Container) Serve() {
	debugPrint()

	for _, comp := range c.components {
		comp.Init()
	}

	for _, comp := range c.components {
		comp.Start()
	}

	signal.Notify(c.sig, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGKILL, syscall.SIGTERM)

	select {
	case s := <-c.sig:
		log.Warnf("container got signal %v", s)
	case <-c.die:
		log.Warn("container will close")
	}

	for _, comp := range c.components {
		comp.Destroy()
	}
}

// Close 关闭容器
func (c *Container) Close() {
	c.die <- struct{}{}
}

func debugPrint() {
	if !mode.IsDebugMode() {
		return
	}

	log.Debug("Welcome to the due framework, Learn more at https://github.com/dobyte/due")
}

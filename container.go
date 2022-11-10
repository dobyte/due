package due

import (
	"github.com/dobyte/due/component"
	"github.com/dobyte/due/config"
	"github.com/dobyte/due/log"

	"os"
	"os/signal"
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
	log.Debug("Welcome to the due framework, Learn more at https://github.com/dobyte/due")

	for _, comp := range c.components {
		comp.Init()
	}

	for _, comp := range c.components {
		comp.Start()
	}

	signal.Notify(c.sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGUSR2)

	sig := <-c.sig

	log.Warnf("process got signal %v, container will close", sig)

	signal.Stop(c.sig)
	config.Close()

	for _, comp := range c.components {
		comp.Destroy()
	}
}

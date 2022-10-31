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

	defer func() {
		log.Warn("container will close")
		signal.Stop(c.sig)
		close(c.die)
		for _, comp := range c.components {
			comp.Destroy()
		}
	}()

	signal.Notify(c.sig, syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGUSR1, syscall.SIGUSR2)

RESERVE:
	select {
	case sig := <-c.sig:
		log.Warnf("container got signal %v", sig)
		switch sig {
		case syscall.SIGHUP, syscall.SIGINT, syscall.SIGQUIT, syscall.SIGKILL, syscall.SIGTERM: // 监听关闭
			return
		case syscall.SIGUSR1, syscall.SIGUSR2: // 监听重启
			for _, comp := range c.components {
				comp.Restart()
			}

			goto RESERVE
		}
	case <-c.die:
		return
	}
}

// Close 关闭容器
func (c *Container) Close() {
	c.die <- struct{}{}
	config.Close()
}

func debugPrint() {
	log.Debug("Welcome to the due framework, Learn more at https://github.com/dobyte/due")
}

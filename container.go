package due

import (
	"fmt"
	"github.com/dobyte/due/component"
	"github.com/dobyte/due/config"
	"github.com/dobyte/due/eventbus"
	"github.com/dobyte/due/log"
	"github.com/dobyte/due/task"
	"github.com/dobyte/due/utils/xfile"
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
	log.Debug(fmt.Sprintf("Welcome to the due framework %s, Learn more at %s", Version, Website))

	for _, comp := range c.components {
		comp.Init()
	}

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
		log.Errorf("eventbus close failed: %v", err)
	}

	task.Release()

	config.Close()
}

func (c *Container) doSavePID() {
	filename := config.Get("config.pid").String()
	if filename == "" {
		return
	}

	err := xfile.WriteFile(filename, []byte(strconv.Itoa(syscall.Getpid())))
	if err != nil {
		log.Fatalf("pid save failed: %v", err)
	}
}

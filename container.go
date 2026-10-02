package due

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strconv"
	"strings"
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
	"github.com/dobyte/due/v2/mode"
	"github.com/dobyte/due/v2/task"
	"github.com/dobyte/due/v2/utils/xcall"
	"github.com/dobyte/due/v2/utils/xos"
	"github.com/dobyte/due/v2/utils/xtime"
)

const (
	defaultPIDKey                 = "etc.pid"                 // PID file path
	defaultShutdownMaxWaitTimeKey = "etc.shutdownMaxWaitTime" // Maximum wait time for container shutdown
)

type Container struct {
	components []component.Component
}

// NewContainer returns a new container.
func NewContainer() *Container {
	return &Container{}
}

// Add appends the given components to the container.
func (c *Container) Add(components ...component.Component) {
	c.components = append(c.components, components...)
}

// Serve starts the container.
//
// It initializes and starts every component in order, waits for a system signal, then closes and
// destroys the components and finally cleans up the related modules. When isNonWaitSignal is true,
// the container skips waiting for the system signal and enters the shutdown flow right after
// startup.
func (c *Container) Serve(isNonWaitSignal ...bool) {
	c.printFrameworkInfo()

	c.initAllComponents()

	c.startAllComponents()

	c.savePidToFile()

	if len(isNonWaitSignal) == 0 || !isNonWaitSignal[0] {
		c.waitSystemSignal()
	}

	c.closeAllComponents()

	c.destroyAllComponents()

	c.deletePidFile()

	c.clearAllModules()
}

// initAllComponents initializes every component.
func (c *Container) initAllComponents() {
	for _, comp := range c.components {
		comp.Init()
	}
}

// startAllComponents starts every component.
func (c *Container) startAllComponents() {
	for _, comp := range c.components {
		comp.Start()
	}
}

// closeAllComponents closes every component.
//
// All components are closed concurrently in separate goroutines, bounded overall by the
// etc.shutdownMaxWaitTime timeout.
func (c *Container) closeAllComponents() {
	g := xcall.NewGoroutines()

	for _, comp := range c.components {
		g.Add(comp.Close)
	}

	g.Run(context.Background(), etc.Get(defaultShutdownMaxWaitTimeKey).Duration())
}

// destroyAllComponents destroys every component.
//
// All components are destroyed concurrently in separate goroutines, bounded overall by a
// five-second timeout.
func (c *Container) destroyAllComponents() {
	g := xcall.NewGoroutines()

	for _, comp := range c.components {
		g.Add(comp.Destroy)
	}

	g.Run(context.Background(), 5*time.Second)
}

// waitSystemSignal waits for a system signal.
//
// It blocks until a process-exit signal arrives, then stops listening and logs the signal.
func (c *Container) waitSystemSignal() {
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

// clearAllModules cleans up every module.
func (c *Container) clearAllModules() {
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

// savePidToFile writes the process ID to the configured file.
func (c *Container) savePidToFile() {
	filename := etc.Get(defaultPIDKey).String()
	if filename == "" {
		return
	}

	if err := xos.WriteFile(filename, []byte(strconv.Itoa(syscall.Getpid()))); err != nil {
		log.Fatalf("pid save failed: %v", err)
	}
}

// deletePidFile removes the process ID file.
func (c *Container) deletePidFile() {
	filename := etc.Get(defaultPIDKey).String()
	if filename == "" {
		return
	}

	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		log.Warnf("pid file remove failed: %v", err)
	}
}

// printFrameworkInfo prints the framework information.
func (c *Container) printFrameworkInfo() {
	fmt.Println(strings.TrimSuffix(strings.TrimPrefix(Logo, "\n"), "\n"))

	info.Print("",
		fmt.Sprintf("[Website] %s", Website),
		fmt.Sprintf("[Version] %s", Version),
	)

	info.Print("",
		fmt.Sprintf("OS: %s %s", runtime.GOOS, runtime.GOARCH),
		fmt.Sprintf("Go: %s", "v"+strings.TrimPrefix(runtime.Version(), "go")),
		fmt.Sprintf("PID: %d", os.Getpid()),
		fmt.Sprintf("Mode: %s", mode.GetMode()),
		fmt.Sprintf("Time: %s", xtime.Now()),
	)
}

package internal

import (
	"context"
	"github.com/dobyte/due/alert"
	"github.com/dobyte/due/log"
	"golang.org/x/sync/errgroup"
	"sync"
	"sync/atomic"
)

type Manager struct {
	ctx      context.Context
	cancel   context.CancelFunc
	alerters []alert.Alerter
	messages chan string
	once     sync.Once
}

func NewManager() *Manager {

	m := &Manager{}
	m.alerters = make([]alert.Alerter, 0)
	m.messages = make(chan string, 2048)
	m.ctx, m.cancel = context.WithCancel(context.Background())

	return m
}

// AddAlerter 添加报警器
func (m *Manager) AddAlerter(alerter alert.Alerter) {
	m.alerters = append(m.alerters, alerter)
}

// 启动报警器
func (m *Manager) run() {
	for {
		select {
		case <-m.ctx.Done():
			return
		case msg, ok := <-m.messages:
			if !ok {
				return
			}

			_, err := m.Alert(msg)
			if err != nil {
				log.Warnf("send alert message failed: %v", err)
			}
		}
	}
}

// Alert 报警
func (m *Manager) Alert(msg string) (int, error) {
	var (
		eg    errgroup.Group
		count int32
	)

	for _, alerter := range m.alerters {
		func(alerter alert.Alerter) {
			eg.Go(func() error {
				err := alerter.Alert(msg)
				if err != nil {
					return err
				}

				atomic.AddInt32(&count, 1)

				return nil
			})
		}(alerter)
	}

	err := eg.Wait()

	return int(count), err
}

// AsyncAlert 异步报警
func (m *Manager) AsyncAlert(msg string) {
	m.once.Do(func() {
		go m.run()
	})
	m.messages <- msg
}

// Close 关闭报警器
func (m *Manager) Close() {
	m.cancel()
}

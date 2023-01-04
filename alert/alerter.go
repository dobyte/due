package alert

import (
	"github.com/dobyte/due/alert/feishu"
	"github.com/dobyte/due/log"
)

type Alerter interface {
	// Name 名称
	Name() string
	// Alert 报警
	Alert(msg string) error
}

func init() {
	Register(feishu.NewAlerter())
}

var alerters = make(map[string]Alerter)

// Register 注册报警器
func Register(alerter Alerter) {
	if alerter == nil {
		log.Fatal("can't register a invalid alerter")
	}

	name := alerter.Name()

	if name == "" {
		log.Fatal("can't register a alerter without name")
	}

	if _, ok := alerters[name]; ok {
		log.Warnf("the old %s alerter will be overwritten", name)
	}

	alerters[name] = alerter
}

// Invoke 调用报警器
func Invoke(name string) Alerter {
	alerter, ok := alerters[name]
	if !ok {
		log.Fatalf("%s alerter is not registered", name)
	}

	return alerter
}

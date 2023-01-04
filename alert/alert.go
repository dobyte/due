package alert

import "github.com/dobyte/due/alert/internal"

var globalManager *internal.Manager

func init() {
	globalManager = internal.NewManager()
}

// AddAlerter 添加报警器
func AddAlerter(alerter Alerter) {
	globalManager.AddAlerter(alerter)
}

// Alert 报警
func Alert(msg string) (int, error) {
	return globalManager.Alert(msg)
}

// AsyncAlert 异步报警
func AsyncAlert(msg string) {
	globalManager.AsyncAlert(msg)
}

// Close 关闭报警器
func Close() {
	globalManager.Close()
}

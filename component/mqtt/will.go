package mqtt

import mqtt "github.com/mochi-mqtt/server/v2"

type Will struct {
	cli *mqtt.Client
}

// Topic 获取Will主题
func (w *Will) Topic() string {
	return w.cli.Properties.Will.TopicName
}

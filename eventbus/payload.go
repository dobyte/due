package eventbus

import "github.com/dobyte/due/utils/xuuid"

//ID        string    `json:"id"`        // 消息ID
//Topic     string    `json:"topic"`     // 消息主题
//Message   string    `json:"message"`   // 消息内容
//Timestamp time.Time `json:"timestamp"` // 消息时间

func BuildPayload(topic string, payload interface{}) ([]byte, error) {
	id, err := xuuid.UUID()
	if err != nil {
		return nil, err
	}

}

func ParsePayload(payload interface{}) (*Event, error) {

}

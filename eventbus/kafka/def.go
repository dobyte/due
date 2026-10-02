package kafka

// data is the internal data format of the eventbus.
type data struct {
	ID        string `json:"id"`
	Topic     string `json:"topic"`
	Payload   string `json:"payload"`
	Timestamp int64  `json:"timestamp"`
}

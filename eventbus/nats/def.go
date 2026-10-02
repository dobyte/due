package nats

type data struct {
	ID        string `json:"id"`        // Event ID
	Topic     string `json:"topic"`     // Event topic
	Payload   string `json:"payload"`   // Event payload
	Timestamp int64  `json:"timestamp"` // Event timestamp
}

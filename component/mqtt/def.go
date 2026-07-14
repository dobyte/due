package mqtt

type ListenerType string

const (
	ListenerTypeTCP ListenerType = "tcp" // TCP监听类型
	ListenerTypeWS  ListenerType = "ws"  // WebSocket监听类型
)

package mqtt

type Event int

const (
	Connect    Event = iota + 1 // 连接成功
	Disconnect                  // 断开连接
	Subscribe
	Unsubscribe
	Publish
	Received
)

type EventHandler func(cli *Client)

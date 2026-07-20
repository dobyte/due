package mqtt

type Event int

const (
	Connect      Event = iota + 1 // 连接成功
	Disconnect                    // 断开连接
	Subscribed                    // 订阅成功
	Unsubscribed                  // 取消订阅
)

type EventHandler func(cli *Client)

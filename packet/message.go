package packet

type Message struct {
	Route  int32  // 路由ID
	Buffer []byte // 消息内容
}

package packet

// Message 消息
// 用于在编解码之间传递普通数据消息
type Message struct {
	Seq    int32  // 序列号
	Route  int32  // 路由ID
	Buffer []byte // 消息内容
}

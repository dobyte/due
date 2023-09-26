package packet

type Message struct {
	Seq      int32 // 序列号
	Route    int32 // 路由ID
	Compress bool
	Buffer   []byte // 消息内容
}

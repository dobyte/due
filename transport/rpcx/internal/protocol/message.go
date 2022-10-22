package protocol

type Message struct {
	Seq    int32  // 序列号
	Route  int32  // 路由
	Buffer []byte // 消息内容
}

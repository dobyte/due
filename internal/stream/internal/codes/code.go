package codes

const (
	OK              uint16 = iota // 成功
	NotFoundSession               // 未找到会话连接
	Internal
)

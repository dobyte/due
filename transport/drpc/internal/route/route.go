package route

const (
	Bind          int8 = iota + 1 // 绑定用户请求
	bindRes                       // 绑定用户响应
	unbindReq                     // 解绑用户请求
	unbindRes                     // 解绑用户响应
	getIPReq                      // 获取IP地址请求
	getIPRes                      // 获取IP地址响应
	statReq                       // 统计在线人数请求
	statRes                       // 统计在线人数响应
	disconnectReq                 // 断开连接请求
	disconnectRes                 // 断开连接响应
	pushReq                       // 推送消息请求
	pushRes                       // 推送消息响应
)

package packet

// 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7 0 1 2 3 4 5 6 7
// +---------------------------------------------------------------+-+-------------+-------------------------------+-------------------------------+
// |                              size                             |h|   extcode   |             route             |              seq              |
// +---------------------------------------------------------------+-+-------------+-------------------------------+-------------------------------+
// |                                                                message data ...                                                               |
// +-----------------------------------------------------------------------------------------------------------------------------------------------+

const (
	defaultSizeBytes   = 4 // 包长度字节数
	defaultHeaderBytes = 1 // 头信息字节数
	defaultSeqBytes    = 8 // 序列号字节数
	defaultRouteBytes  = 1 // 路由号字节数
	defaultCodeBytes   = 2 // 错误码字节数
)

const (
	dataBit      uint8 = 0 << 7 // 数据标识位
	heartbeatBit uint8 = 1 << 7 // 心跳标识位
)

const (
	cmdBind       int8 = iota // 绑定用户
	cmdUnbind                 // 解绑用户
	cmdGetIP                  // 获取IP地址
	cmdIsOnline               // 检测是否在线
	cmdStat                   // 统计在线人数
	cmdDisconnect             // 断开连接
	cmdPush                   // 推送消息
	cmdMulticast              // 推送组播消息
	cmdBroadcast              // 推送广播消息
)

const (
	bindReq       int8 = iota + 1 // 绑定用户请求
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

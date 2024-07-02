package route

const (
	Handshake  uint8 = iota + 1 // 握手
	Bind                        // 绑定用户
	Unbind                      // 解绑用户
	GetIP                       // 获取IP地址
	Stat                        // 统计在线人数
	IsOnline                    // 检测用户是否在线
	Disconnect                  // 断开连接
	Push                        // 推送单个消息
	Multicast                   // 推送组播消息
	Broadcast                   // 推送广播消息
	Trigger                     // 触发事件
	Deliver                     // 投递消息
	GetState                    // 获取状态
	SetState                    // 设置状态
)

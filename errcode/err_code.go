package errcode

const (
	Succeed               = 0   // 成功
	Invalid_pb_message    = 20  // proto消息格式非法
	Bad_request           = 400 // 请求失败
	Unauthorized          = 401 // 未验证
	Forbidden             = 403 // 被禁止
	Not_found             = 404 // 未找到
	Internal_server_error = 500 // 服务器内部错误
)

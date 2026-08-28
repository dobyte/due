package digestauth

import (
	"time"

	"github.com/dobyte/due/component/http/v2"
)

// Config Digest认证中间件配置
type Config struct {
	// Next 定义跳过中间件的判断函数，返回true时跳过本中间件
	//
	// 可选。默认: nil
	Next func(ctx http.Context) bool

	// Users 允许的凭据列表
	// key为用户名，value为预计算的HA1:
	// HA1 = MD5(username:realm:password)
	//
	// 若Users为空，则必须设置Authorizer
	//
	// 可选。默认: map[string]string{}
	Users map[string]string

	// Authorizer 自定义凭据校验函数
	// 使用用户名校验凭据，返回对应的HA1字符串及是否通过
	// 用户不存在时返回"", false
	//
	// 可选。默认: nil
	Authorizer func(username string) (ha1 string, ok bool)

	// Unauthorized 未授权响应处理函数
	// 默认返回401 Unauthorized并携带正确的WWW-Authenticate头
	//
	// 可选。默认: nil
	Unauthorized http.Handler

	// BadRequest 错误Authorization头响应处理函数
	// 默认返回400 Bad Request且不携带WWW-Authenticate头
	//
	// 可选。默认: nil
	BadRequest http.Handler

	// Realm 定义DigestAuth的realm属性，用于标识认证系统
	//
	// 可选。默认: "Restricted"
	Realm string

	// HeaderLimit Authorization头的最大长度限制
	// 超过该长度的请求将被拒绝
	//
	// 可选。默认: 8192
	HeaderLimit int

	// NonceTTL nonce值的有效期
	// 在该窗口内，每个请求必须携带单调递增的nonce计数(nc)，否则将被视为重放而拒绝
	//
	// 可选。默认: 5 * time.Minute
	NonceTTL time.Duration

	// ContextUsernameKey 用户名在上下文中的存储key
	//
	// 可选。默认: "username"
	ContextUsernameKey string
}

// ConfigDefault 默认配置
var ConfigDefault = Config{
	Next:               nil,
	Users:              map[string]string{},
	Realm:              "Restricted",
	HeaderLimit:        8192,
	NonceTTL:           5 * time.Minute,
	Authorizer:         nil,
	Unauthorized:       nil,
	BadRequest:         nil,
	ContextUsernameKey: "username",
}

// 填充配置默认值
// 未提供的配置项使用默认值补齐
// @param config ...Config 待处理的配置
// @return @1 Config 填充默认值后的配置
func configDefault(config ...Config) Config {
	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigDefault
	}

	// Override default config
	cfg := config[0]

	// Set default values
	if cfg.Next == nil {
		cfg.Next = ConfigDefault.Next
	}

	if cfg.Users == nil {
		cfg.Users = ConfigDefault.Users
	}

	if cfg.Realm == "" {
		cfg.Realm = ConfigDefault.Realm
	}

	if cfg.HeaderLimit <= 0 {
		cfg.HeaderLimit = ConfigDefault.HeaderLimit
	}

	if cfg.NonceTTL <= 0 {
		cfg.NonceTTL = ConfigDefault.NonceTTL
	}

	if cfg.ContextUsernameKey == "" {
		cfg.ContextUsernameKey = ConfigDefault.ContextUsernameKey
	}

	return cfg
}

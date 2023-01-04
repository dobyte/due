package feishu

import "github.com/dobyte/due/config"

const (
	feishuSecretKey  = "config.alert.feishu.secret"
	feishuWebhookKey = "config.alert.feishu.webhook"
)

type Option func(o *options)

type options struct {
	secret  string // 签名秘钥
	webhook string // webhook地址
}

func defaultOptions() *options {
	return &options{
		secret:  config.Get(feishuSecretKey).String(),
		webhook: config.Get(feishuWebhookKey).String(),
	}
}

// WithSecret 设置签名秘钥
func WithSecret(secret string) Option {
	return func(o *options) { o.secret = secret }
}

// WithWebhook 设置webhook地址
func WithWebhook(webhook string) Option {
	return func(o *options) { o.webhook = webhook }
}

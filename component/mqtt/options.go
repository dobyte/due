package mqtt

import (
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/log"
)

const (
	defaultName            = "mqtt" // 默认MQTT服务名称
	defaultReadBufferSize  = 4096   // 默认读取缓冲区大小
	defaultWriteBufferSize = 4096   // 默认写入缓冲区大小
)

const (
	defaultNameKey            = "etc.mqtt.name"
	defaultAuthKey            = "etc.mqtt.auth"
	defaultDebugKey           = "etc.mqtt.debug"
	defaultListensKey         = "etc.mqtt.listens"
	defaultReadBufferSizeKey  = "etc.mqtt.readBufferSize"
	defaultWriteBufferSizeKey = "etc.mqtt.writeBufferSize"
)

// Option MQTT服务器配置函数
type Option func(o *options)

// ListenOptions 监听器配置
type ListenOptions struct {
	ID       string `json:"id"`       // 监听器ID
	Type     string `json:"type"`     // 监听类型（tcp/ws）
	Addr     string `json:"addr"`     // 监听地址
	KeyFile  string `json:"keyFile"`  // 私钥文件路径（选填）
	CertFile string `json:"certFile"` // 证书文件路径（选填）
}

// MQTT服务器配置项
type options struct {
	name            string           // MQTT服务名称
	auth            string           // MQTT认证文件路径（支持json、yaml格式）
	debug           bool             // 是否开启调试模式
	listensOpts     []*ListenOptions // MQTT服务监听器
	readBufferSize  int              // 读取缓冲区大小，默认为4096
	writeBufferSize int              // 写入缓冲区大小，默认为4096
}

// 创建默认配置
// 从配置环境读取各参数并填充默认值
// @return @1 *options 默认配置项
func defaultOptions() *options {
	opts := &options{
		name:            etc.Get(defaultNameKey, defaultName).String(),
		auth:            etc.Get(defaultAuthKey).String(),
		debug:           etc.Get(defaultDebugKey).Bool(),
		listensOpts:     make([]*ListenOptions, 0),
		readBufferSize:  etc.Get(defaultReadBufferSizeKey, defaultReadBufferSize).Int(),
		writeBufferSize: etc.Get(defaultWriteBufferSizeKey, defaultWriteBufferSize).Int(),
	}

	if err := etc.Get(defaultListensKey).Scan(&opts.listensOpts); err != nil {
		opts.listensOpts = defaultListensOptions()

		log.Warnf("scan listen options failed: %v", err)
	}

	return opts
}

// 返回默认监听配置
func defaultListensOptions() []*ListenOptions {
	return []*ListenOptions{{
		ID:   "m1",
		Type: "tcp",
		Addr: ":1883",
	}, {
		ID:   "m2",
		Type: "ws",
		Addr: ":1884",
	}}
}

// WithName 设置实例名称
func WithName(name string) Option {
	return func(o *options) { o.name = name }
}

// WithAuth 设置认证文件路径
func WithAuth(auth string) Option {
	return func(o *options) { o.auth = auth }
}

// WithDebug 设置是否开启调试模式
func WithDebug(debug bool) Option {
	return func(o *options) { o.debug = debug }
}

// WithListensOptions 设置监听配置
func WithListensOptions(listensOpts ...*ListenOptions) Option {
	return func(o *options) { o.listensOpts = listensOpts }
}

// WithReadBufferSize 设置读取缓冲区大小
// @param size int 读取缓冲区大小
func WithReadBufferSize(size int) Option {
	return func(o *options) { o.readBufferSize = size }
}

// WithWriteBufferSize 设置写入缓冲区大小
func WithWriteBufferSize(size int) Option {
	return func(o *options) { o.writeBufferSize = size }
}

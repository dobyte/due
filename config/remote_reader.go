package config

import (
	"github.com/apolloconfig/agollo/v4"
	"github.com/apolloconfig/agollo/v4/storage"
	"github.com/apolloconfig/agollo/v4/utils"
	"github.com/symsimmy/due/apollo"
	"github.com/symsimmy/due/errors"
	"github.com/symsimmy/due/value"
)

const (
	RemoteReaderSetError = "cannot set key in remote reader"
)

const (
	defaultAppId      = "GameServices"
	defaultNamespace  = "application"
	defaultApolloHost = "http://127.0.0.1"
	defaultApolloPort = 18080
)

type ApolloReader struct {
	client agollo.Client
}

func NewApolloReader(opts ...RemoteReaderOption) *ApolloReader {
	o := &remoteReaderOptions{
		AppId:     defaultAppId,
		Namespace: defaultNamespace,
		Host:      defaultApolloHost,
		Port:      defaultApolloPort,
	}
	for _, opt := range opts {
		opt(o)
	}

	client := apollo.InitApolloClient(o.AppId, o.Namespace, o.Host, o.Port)
	r := &ApolloReader{client: client}

	return r
}

// Has 是否存在配置
func (r *ApolloReader) Has(pattern string) bool {
	return r.client.GetValue(pattern) != utils.Empty
}

// Get 获取配置值
func (r *ApolloReader) Get(pattern string, def ...interface{}) value.Value {
	v := r.client.GetValue(pattern)
	if v == utils.Empty {
		return value.NewValue(def...)
	}
	return value.NewValue(v)
}

// Set 设置配置值
func (r *ApolloReader) Set(pattern string, value interface{}) error {
	return errors.New(RemoteReaderSetError)
}

// AddChangeListener 设置远端配置变更监听
func (r *ApolloReader) AddChangeListener(listener storage.ChangeListener) {
	r.client.AddChangeListener(listener)
}

// RemoveChangeListener 取消远端配置变更监听
func (r *ApolloReader) RemoveChangeListener(listener storage.ChangeListener) {
	r.client.RemoveChangeListener(listener)
}

// Close 关闭配置监听
func (r *ApolloReader) Close() {
	r.client.Close()
}

// Range 遍历所有key
func (r *ApolloReader) Range(f func(key, value interface{}) bool) {
	r.client.GetConfigCache(storage.GetDefaultNamespace()).Range(f)
}

type RemoteReaderOption func(o *remoteReaderOptions)

type remoteReaderOptions struct {
	AppId     string
	Namespace string
	Host      string
	Port      int
}

// WithAppId 设置上下文
func WithAppId(appId string) RemoteReaderOption {
	return func(o *remoteReaderOptions) { o.AppId = appId }
}

// WithNamespace 设置上下文
func WithNamespace(namespace string) RemoteReaderOption {
	return func(o *remoteReaderOptions) { o.Namespace = namespace }
}

// WithHost 设置上下文
func WithHost(host string) RemoteReaderOption {
	return func(o *remoteReaderOptions) { o.Host = host }
}

// WithPort 设置上下文
func WithPort(port int) RemoteReaderOption {
	return func(o *remoteReaderOptions) { o.Port = port }
}

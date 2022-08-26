package grpc

import "google.golang.org/grpc"

type Option func(o *options)

type options struct {
	addr       string              // 地址
	certFile   string              // 证书文件
	keyFile    string              // 秘钥文件
	serverOpts []grpc.ServerOption // 服务器选项
}

//
func WithServerListenAddr(addr string) Option {
	return func(o *options) { o.addr = addr }
}

// WithServerCredentials 设置证书和秘钥
func WithServerCredentials(certFile, keyFile string) Option {
	return func(o *options) { o.keyFile, o.certFile = keyFile, certFile }
}

func WithServerOptions(opts ...grpc.ServerOption) Option {
	return func(o *options) { o.serverOpts = opts }
}

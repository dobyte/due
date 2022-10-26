package rsa

import "github.com/dobyte/due/utils/xconv"

type Option func(o *options)

type options struct {
	publicKey  string
	privateKey string
	label      []byte
}

func defaultOptions() *options {
	return &options{}
}

func WithPublicKey(publicKey string) Option {
	return func(o *options) { o.publicKey = publicKey }
}

func WithPrivateKey(privateKey string) Option {
	return func(o *options) { o.privateKey = privateKey }
}

func WithLabel(label string) Option {
	return func(o *options) { o.label = xconv.StringToBytes(label) }
}

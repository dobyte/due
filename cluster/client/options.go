package client

import (
	"context"

	"github.com/dobyte/due/v2/crypto"
	"github.com/dobyte/due/v2/encoding"
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/log"
	"github.com/dobyte/due/v2/network"
	"github.com/dobyte/due/v2/utils/xuuid"
)

const (
	defaultName  = "client" // default client name
	defaultCodec = "proto"  // default codec name
)

const (
	defaultIDKey    = "etc.cluster.client.id"
	defaultNameKey  = "etc.cluster.client.name"
	defaultCodecKey = "etc.cluster.client.codec"
)

// Option is a client option.
type Option func(o *options)

// options are the client options.
type options struct {
	id        string           // instance ID
	name      string           // instance name
	ctx       context.Context  // context
	codec     encoding.Codec   // codec
	client    network.Client   // network client
	encryptor crypto.Encryptor // message encryptor
}

// defaultOptions returns the default options. Options are read from the configuration center,
// falling back to the built-in defaults when they are absent.
func defaultOptions() *options {
	opts := &options{}
	opts.ctx = context.Background()

	if id := etc.Get(defaultIDKey).String(); id != "" {
		opts.id = id
	} else {
		opts.id = xuuid.UUID()
	}

	if name := etc.Get(defaultNameKey, defaultName).String(); name != "" {
		opts.name = name
	} else {
		opts.name = defaultName
	}

	if codec := etc.Get(defaultCodecKey, defaultCodec).String(); codec != "" {
		opts.codec = encoding.Invoke(codec)
	} else {
		opts.codec = encoding.Invoke(defaultCodec)
	}

	return opts
}

// WithID sets the instance ID.
func WithID(id string) Option {
	return func(o *options) {
		if id != "" {
			o.id = id
		} else {
			log.Warnf("the specified id is empty and will be automatically ignored")
		}
	}
}

// WithName sets the instance name.
func WithName(name string) Option {
	return func(o *options) {
		if name != "" {
			o.name = name
		} else {
			log.Warnf("the specified name is empty and will be automatically ignored")
		}
	}
}

// WithCodec sets the codec.
func WithCodec(codec encoding.Codec) Option {
	return func(o *options) {
		if codec != nil {
			o.codec = codec
		} else {
			log.Warnf("the specified codec is nil and will be automatically ignored")
		}
	}
}

// WithClient sets the network client.
func WithClient(client network.Client) Option {
	return func(o *options) {
		if client != nil {
			o.client = client
		} else {
			log.Warnf("the specified client is nil and will be automatically ignored")
		}
	}
}

// WithContext sets the context.
func WithContext(ctx context.Context) Option {
	return func(o *options) {
		if ctx != nil {
			o.ctx = ctx
		} else {
			log.Warnf("the specified ctx is nil and will be automatically ignored")
		}
	}
}

// WithEncryptor sets the message encryptor.
func WithEncryptor(encryptor crypto.Encryptor) Option {
	return func(o *options) {
		if encryptor != nil {
			o.encryptor = encryptor
		} else {
			log.Warnf("the specified encryptor is nil and will be automatically ignored")
		}
	}
}

// DialOption is a dial option.
type DialOption func(o *dialOptions)

// dialOptions are the dial options.
type dialOptions struct {
	addr  string         // dial address
	attrs map[string]any // connection attributes
}

// WithDialAddr sets the dial address.
func WithDialAddr(addr string) DialOption {
	return func(o *dialOptions) { o.addr = addr }
}

// WithConnAttr sets a connection attribute.
func WithConnAttr(key string, value any) DialOption {
	return func(o *dialOptions) { o.attrs[key] = value }
}

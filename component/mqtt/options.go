package mqtt

import (
	"github.com/dobyte/due/v2/etc"
	"github.com/dobyte/due/v2/log"
)

const (
	defaultName            = "mqtt" // Default MQTT service name
	defaultReadBufferSize  = 4096   // Default read buffer size
	defaultWriteBufferSize = 4096   // Default write buffer size
)

const (
	defaultNameKey            = "etc.mqtt.name"
	defaultAuthKey            = "etc.mqtt.auth"
	defaultDebugKey           = "etc.mqtt.debug"
	defaultListensKey         = "etc.mqtt.listens"
	defaultReadBufferSizeKey  = "etc.mqtt.readBufferSize"
	defaultWriteBufferSizeKey = "etc.mqtt.writeBufferSize"
)

// Option is an MQTT server configuration function.
type Option func(o *options)

// ListenOptions is the listener configuration.
type ListenOptions struct {
	ID       string `json:"id"`       // Listener ID
	Type     string `json:"type"`     // Listener type (tcp/ws)
	Addr     string `json:"addr"`     // Listen address
	KeyFile  string `json:"keyFile"`  // Private key file path (optional)
	CertFile string `json:"certFile"` // Certificate file path (optional)
}

// options is the MQTT server configuration.
type options struct {
	name            string           // MQTT service name
	auth            string           // MQTT auth file path (json and yaml are supported)
	debug           bool             // Whether debug mode is enabled
	listensOpts     []*ListenOptions // MQTT service listeners
	readBufferSize  int              // Read buffer size, defaults to 4096
	writeBufferSize int              // Write buffer size, defaults to 4096
}

// defaultOptions creates the default configuration.
//
// It reads each parameter from the configuration environment and fills in the default values.
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

// defaultListensOptions returns the default listener configuration.
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

// WithName sets the instance name.
func WithName(name string) Option {
	return func(o *options) { o.name = name }
}

// WithAuth sets the auth file path.
func WithAuth(auth string) Option {
	return func(o *options) { o.auth = auth }
}

// WithDebug sets whether debug mode is enabled.
func WithDebug(debug bool) Option {
	return func(o *options) { o.debug = debug }
}

// WithListensOptions sets the listener configuration.
func WithListensOptions(listensOpts ...*ListenOptions) Option {
	return func(o *options) { o.listensOpts = listensOpts }
}

// WithReadBufferSize sets the read buffer size.
func WithReadBufferSize(size int) Option {
	return func(o *options) { o.readBufferSize = size }
}

// WithWriteBufferSize sets the write buffer size.
func WithWriteBufferSize(size int) Option {
	return func(o *options) { o.writeBufferSize = size }
}

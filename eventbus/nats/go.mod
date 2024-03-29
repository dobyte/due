module github.com/symsimmy/due/eventbus/nats

go 1.21

toolchain go1.21.3

require (
	github.com/nats-io/nats.go v1.23.0
	github.com/symsimmy/due v0.0.8
)

require github.com/nats-io/nats-server/v2 v2.9.14 // indirect

replace github.com/symsimmy/due => ./../../

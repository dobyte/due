module github.com/dobyte/due/eventbus/nats

go 1.16

require (
	github.com/dobyte/due v0.0.13
	github.com/nats-io/nats-server/v2 v2.9.14 // indirect
	github.com/nats-io/nats.go v1.23.0
)

replace github.com/dobyte/due => ./../../

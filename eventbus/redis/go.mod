module github.com/symsimmy/due/eventbus/redis

go 1.21

toolchain go1.21.3

require (
	github.com/go-redis/redis/v8 v8.11.5
	github.com/symsimmy/due v0.0.8
)

replace github.com/symsimmy/due => ./../../

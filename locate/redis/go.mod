module github.com/symsimmy/due/locate/redis

go 1.16

require (
	github.com/symsimmy/due v0.0.4
	github.com/go-redis/redis/v8 v8.11.5
	github.com/jonboulle/clockwork v0.3.0 // indirect
	golang.org/x/sync v0.1.0
)

replace github.com/symsimmy/due => ./../../

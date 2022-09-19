module github.com/dobyte/due/locate/redis

go 1.16

require (
	github.com/dobyte/due v0.0.2
	github.com/go-redis/redis/v8 v8.11.5
	github.com/jonboulle/clockwork v0.3.0 // indirect
)

replace github.com/dobyte/due => ./../../

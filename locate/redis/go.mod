module github.com/dobyte/due/locate/redis

go 1.16

require (
	github.com/dobyte/due v0.0.4
	github.com/go-redis/redis/v8 v8.11.5
	github.com/jonboulle/clockwork v0.3.0 // indirect
	golang.org/x/sync v0.0.0-20220722155255-886fb9371eb4
)

replace github.com/dobyte/due => ./../../

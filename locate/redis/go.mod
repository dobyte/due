module github.com/dobyte/due/locate/redis/v2

go 1.20

require (
	github.com/dobyte/due/v2 v2.0.0
	github.com/go-redis/redis/v8 v8.11.5
	golang.org/x/sync v0.3.0
)

require github.com/jonboulle/clockwork v0.3.0 // indirect

replace github.com/dobyte/due/v2 => ../../

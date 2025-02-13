module github.com/dobyte/due/lock/redis/v2

go 1.22.9

require (
	github.com/go-redis/redis/v8 v8.11.5
	github.com/dobyte/due/v2 v2.2.5
)

require (
	github.com/cespare/xxhash/v2 v2.1.2 // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/fsnotify/fsnotify v1.7.0 // indirect
	golang.org/x/sys v0.30.0 // indirect
)

replace github.com/dobyte/due/v2 => ../../

module github.com/symsimmy/due/transport/grpc

go 1.16

require (
	github.com/dobyte/due v0.0.24
	github.com/golang/protobuf v1.5.2
	google.golang.org/grpc v1.50.1
	google.golang.org/protobuf v1.28.1 // indirect
)

replace github.com/dobyte/due => ./../../

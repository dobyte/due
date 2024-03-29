module github.com/symsimmy/due/transport/grpc

go 1.21

toolchain go1.21.5

require (
	github.com/golang/protobuf v1.5.3
	github.com/symsimmy/due v0.0.8
	google.golang.org/grpc v1.56.3
)

require github.com/gogo/protobuf v1.3.2 // indirect

replace github.com/symsimmy/due => ./../../

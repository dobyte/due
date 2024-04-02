module github.com/symsimmy/due/registry/etcd

go 1.21

require (
	github.com/symsimmy/due v0.0.8
	go.etcd.io/etcd/api/v3 v3.5.4
	go.etcd.io/etcd/client/v3 v3.5.4
)

replace github.com/symsimmy/due => ../../

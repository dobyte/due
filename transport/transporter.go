package transport

import (
	"github.com/dobyte/due/v2/registry"
)

type Transporter interface {
	// SetDefaultDiscovery 设置默认的服务发现组件
	SetDefaultDiscovery(discovery registry.Discovery)
	// NewMeshServer 新建微服务服务器
	NewMeshServer() (Server, error)
	// NewMeshClient 新建微服务客户端
	NewMeshClient(target string) (ServiceClient, error)
}

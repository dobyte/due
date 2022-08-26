package registry

import "context"

type Registry interface {
	// Register 注册服务实例
	Register(ins *ServiceInstance) error
	// Deregister 解注册服务实例
	Deregister(ins *ServiceInstance) error
	// Watch 监听相同服务名的服务实例变化
	Watch(ctx context.Context, serviceName string) (Watcher, error)
	// Services 获取服务实例列表
	Services(ctx context.Context, serviceName string) ([]*ServiceInstance, error)
}

type Watcher interface {
	// Next 返回服务实例列表
	Next() ([]*ServiceInstance, error)
	// Stop 停止监听
	Stop() error
}

type ServiceInstance struct {
	// 服务实体ID，每个服务实体ID唯一
	ID string `json:"id"`
	// 服务实体名
	Name string `json:"name"`
	// 服务路由ID
	Routes []Route `json:"routes"`
	// 服务器实体暴露端口
	Endpoint string `json:"endpoint"`
}

type Route struct {
	// 路由ID
	ID int32 `json:"id"`
	// 是否有状态
	Stateful bool `json:"stateful"`
}

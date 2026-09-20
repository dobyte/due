package main

import (
	"net/http"

	"github.com/dobyte/due/locate/redis/v2"
	"github.com/dobyte/due/network/ws/v2"
	"github.com/dobyte/due/registry/nacos/v2"
	"github.com/dobyte/due/v2"
	"github.com/dobyte/due/v2/cluster/gate"
	"github.com/dobyte/due/v2/component/pprof"
	"github.com/gorilla/websocket"
)

func main() {
	// 创建容器
	container := due.NewContainer()
	// 创建服务器
	server := ws.NewServer()

	// 在 WebSocket 升级前检查是否为合法的 WebSocket 请求，
	// 非 WebSocket 请求（如浏览器直接访问）直接返回 426，避免触发 upgrade error 日志
	server.OnUpgrade(func(w http.ResponseWriter, r *http.Request) (allowed bool) {
		if websocket.IsWebSocketUpgrade(r) {
			return true
		}
		http.Error(w, "websocket connection required", http.StatusUpgradeRequired)
		return false
	})

	// 创建用户定位器
	locator := redis.NewLocator()
	// 创建服务发现
	registry := nacos.NewRegistry()
	// 创建网关组件
	component1 := gate.NewGate(
		gate.WithServer(server),
		gate.WithLocator(locator),
		gate.WithRegistry(registry),
	)
	// 创建PProf组件
	component2 := pprof.NewPProf()
	// 添加网关组件
	container.Add(component1, component2)
	// 启动容器
	container.Serve()
}

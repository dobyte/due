# 配置中心-etcd

### 1.功能

* 支持配置的读取、修改、热更新
* 支持读写模式设置
* 支持集群内热更新
* 支持json、yaml、toml、xml等多种配置格式
* 支持监听配置文件变动

### 2.快速开始

1.安装

```shell
go get -u github.com/dobyte/due/config/etcd/v2@latest
```

2.etc配置项

```toml
# 配置中心
[config]
    # etcd配置中心
    [config.etcd]
        # 客户端连接地址
        addrs = ["127.0.0.1:2379"]
		# 客户端拨号超时时间，支持单位：纳秒（ns）、微秒（us | µs）、毫秒（ms）、秒（s）、分（m）、小时（h）、天（d）。默认为5s
        dialTimeout = 5
        # 路径。默认为/config
        path = "/config"
        # 读写模式。可选：read-only | write-only | read-write，默认为read-only
        mode = "read-write"
```

3.开始使用

```go
package main

import (
	"context"
	"github.com/dobyte/due/config/etcd/v2"
	"github.com/dobyte/due/v2/config"
	"github.com/dobyte/due/v2/log"
	"time"
)

func main() {
	// 设置全局配置器
	config.SetConfigurator(config.NewConfigurator(config.WithSources(etcd.NewSource())))

	ctx := context.Background()
	filepath := "config.toml"

	// 更新配置
	if err := config.Store(ctx, etcd.Name, filepath, map[string]interface{}{
		"timezone": "Local",
	}); err != nil {
		log.Errorf("store config failed: %v", err)
		return
	}

	time.Sleep(5 * time.Millisecond)

	// 读取配置
	timezone := config.Get("config.timezone", "UTC").String()
	log.Infof("timezone: %s", timezone)

	// 更新配置
	if err := config.Store(ctx, etcd.Name, filepath, map[string]interface{}{
		"timezone": "UTC",
	}); err != nil {
		log.Errorf("store config failed: %v", err)
		return
	}

	time.Sleep(5 * time.Millisecond)

	// 读取配置
	timezone = config.Get("config.timezone", "UTC").String()
	log.Infof("timezone: %s", timezone)
}
```

### 4.详细示例

更多详细示例请点击[due-examples](https://github.com/dobyte/due-examples/config/etcd)
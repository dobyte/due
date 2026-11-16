# 配置中心-consul

### 1.功能

* 支持配置的读取、修改、热更新
* 支持读写模式设置
* 支持集群内热更新
* 支持json、yaml、toml、xml等多种配置格式
* 支持监听配置文件变动

### 2.快速开始

1.安装

```shell
go get -u github.com/dobyte/due/config/consul/v2@latest
```

2.consul配置项

```toml
# 配置中心
[config]
    # consul配置中心
    [config.consul]
        # 客户端连接地址
        addr = "127.0.0.1:8500"
        # 路径。默认为config
        path = "config"
        # 读写模式。可选：read-only | write-only | read-write，默认为read-only
        mode = "read-write"
        # 需要加载、修改和监听的配置文件清单，相对 path 的路径。为空表示 path 下的所有文件
        files = ["config.json", "gateway/*.yaml"]
```

3.开始使用

```go
package main

import (
    "context"
    "github.com/dobyte/due/config/consul/v2"
    "github.com/dobyte/due/v2/config"
    "github.com/dobyte/due/v2/log"
    "time"
)

func init() {
    // 设置全局配置器
    config.SetConfigurator(config.NewConfigurator(config.WithSources(consul.NewSource())))
}

func main() {
    var (
        ctx  = context.Background()
        file = "config.toml"
        name = consul.Name
    )

    // 更新配置
    if err := config.Store(ctx, name, file, map[string]any{
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
    if err := config.Store(ctx, name, file, map[string]any{
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

### 3.文件清单

`files` 用于限定需要加载、修改和监听的配置，路径相对 `path`。为空时保持原有行为，`path` 下的所有配置均可用。

* 不含 `*` 的项为精确路径，必须与相对 `path` 的路径完全一致。
* 含 `*` 的项为模式，`*` 可匹配任意字符（包括 `/`），其他字符按字面值匹配；仅支持 `*` 一种通配符。
* 清单非空时，`Load` 只会返回命中清单的配置；显式加载或存储清单外的文件会返回 `errors.ErrFileNotConfigured`。
* 监听同样只会上报命中清单的配置变更。

### 4.详细示例

更多详细示例请点击[due-examples](https://github.com/dobyte/due-examples)
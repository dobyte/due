# 配置中心-polaris

### 1.功能

* 支持配置的读取、修改、热更新
* 支持读写模式设置
* 支持集群内热更新
* 支持json、yaml、toml、xml等多种配置格式
* 支持监听配置文件变动

### 2.快速开始

1.安装

```shell
go get -u github.com/dobyte/due/config/polaris/v2@latest
```

2.etc配置项

```toml
# 配置中心
[config]
    # polaris配置中心
    [config.polaris]
        # 读写模式。可选：read-only | write-only | read-write，默认为read-only
        mode = "read-only"
        # 服务器地址，格式为ip:port，默认为["127.0.0.1:8091"]
        urls = ["127.0.0.1:8091"]
        # 命名空间。默认为default
        namespace = "default"
        # 配置分组。默认为default
        group = "default"
        # 请求Polaris服务端超时时间，支持单位：纳秒（ns）、微秒（us | µs）、毫秒（ms）、秒（s）、分（m）、小时（h）、天（d）。默认为3s
        timeout = "3s"
        # 与Polaris服务端的通信协议，可选：grpc，默认为grpc
        protocol = "grpc"
```

3.开始使用

```go
package main

import (
    "context"
    "github.com/dobyte/due/config/polaris/v2"
    "github.com/dobyte/due/v2/config"
    "github.com/dobyte/due/v2/log"
    "time"
)

func main() {
    // 设置全局配置器
    config.SetConfigurator(config.NewConfigurator(config.WithSources(polaris.NewSource(
        polaris.WithMode(config.ReadWrite),
    ))))

    ctx := context.Background()
    filepath := "config.toml"

    // 更新配置
    if err := config.Store(ctx, polaris.Name, filepath, map[string]any{
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
    if err := config.Store(ctx, polaris.Name, filepath, map[string]any{
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

### 3.外部SDK上下文

polaris组件支持传入外部SDK上下文，用于复用已有的polaris-go SDK实例，此时SDK上下文的销毁由调用方负责。

```go
package main

import (
    "github.com/dobyte/due/config/polaris/v2"
    "github.com/dobyte/due/v2/config"
    "github.com/polarismesh/polaris-go/api"
)

func main() {
    // 构建外部SDK上下文
    ctx, err := api.InitContextByConfig(api.NewConfiguration())
    if err != nil {
        panic(err)
    }
    defer ctx.Destroy()

    // 设置全局配置器
    config.SetConfigurator(config.NewConfigurator(config.WithSources(polaris.NewSource(
        polaris.WithClient(ctx),
    ))))
}
```

### 4.服务端版本兼容性

polaris组件兼容不同版本的服务端，部分新接口在旧版本服务端上不可用时，组件会自动降级：

| 功能 | 新版本服务端 | 旧版本服务端（如2023年发布的v1.17.x） |
|---|---|---|
| 配置读取（Load） | 支持 | 支持 |
| 配置保存（Store） | UpsertAndPublishConfigFile一步完成 | 自动降级为创建（已存在则更新）后发布 |
| 分组文件发现 | 支持 | 不支持分组查询接口，自动禁用分组搜索 |
| 变更监听（Watch） | 分组搜索自动发现新文件 | 通过显式加载（Load）订阅文件变更 |

> 注意：旧版本服务端不支持配置分组查询接口，配置文件的变更监听需在加载（Load）该文件后生效。

### 5.详细示例

更多详细示例请点击[due-examples](https://github.com/dobyte/due-examples)

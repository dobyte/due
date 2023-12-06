# 配置中心-file

### 1.功能

* 支持本地配置文件或目录
* 支持配置的读取、修改、热更新
* 支持读写模式设置
* 支持json、yaml、toml、xml等多种配置格式
* 不支持集群内热更新
* 支持监听配置文件变动

### 2.快速开始

1.file配置项

```toml
# 配置中心
[config]
    # 文件配置
    [config.file]
        # 配置文件或配置目录路径
        path = "./config"
        # 读写模式。可选：read-only | write-only | read-write，默认为read-only
        mode = "read-write"
```

2.开始使用

```go
package main

import (
    "context"
    "github.com/dobyte/due/v2/config"
    "github.com/dobyte/due/v2/config/file"
    "github.com/dobyte/due/v2/log"
    "time"
)

func init() {
    // 设置全局配置器
    config.SetConfigurator(config.NewConfigurator(config.WithSources(file.NewSource())))
}

func main() {
    var (
        ctx      = context.Background()
        name     = file.Name
        filepath = "config.toml"
    )

    // 更新配置
    if err := config.Store(ctx, name, filepath, map[string]interface{}{
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
    if err := config.Store(ctx, name, filepath, map[string]interface{}{
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

### 3.详细示例

更多详细示例请点击[due-examples](https://github.com/dobyte/due-examples)
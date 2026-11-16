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
        # 需要加载、修改和监听的配置文件清单，相对 path 的路径。为空表示 path 下的所有文件
        files = ["config.toml", "gateway/*.yaml"]
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
    if err := config.Store(ctx, name, filepath, map[string]any{
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
    if err := config.Store(ctx, name, filepath, map[string]any{
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

`files` 用于限定需要加载、修改和监听的文件，路径相对 `path`。为空时保持原有行为，`path` 下的所有文件均可用。

* 不含 `*` 的项为精确路径，必须与相对 `path` 的路径完全一致。
* 条目会去除首尾空白、统一使用 `/` 分隔并进行路径清理，因此 `./config.toml` 与 `config.toml` 等价。
* 含 `*` 的项为模式，`*` 可匹配任意字符（包括 `/`），其他字符按字面值匹配；仅支持 `*` 一种通配符。
* 清单非空时，`Load` 只会返回命中清单的文件；显式加载或存储清单外的文件会返回 `errors.ErrFileNotConfigured`。
* 监听同样只会上报命中清单的文件变更；未命中的文件仍会被监听，以便后续命中时能够及时感知。

### 4.详细示例

更多详细示例请点击[due-examples](https://github.com/dobyte/due-examples)
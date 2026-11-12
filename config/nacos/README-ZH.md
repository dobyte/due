# 配置中心-nacos

### 1.功能

* 支持配置的读取、修改、热更新
* 支持读写模式设置
* 支持固定文件列表（files），只加载并监听指定的 dataId
* 支持 Nacos 3.x 的 v3 admin 接口，搜索配置时只返回元数据，降低传输与内存开销
* 支持 json、yaml、xml 配置格式
* 支持监听配置文件变动

### 2.快速开始

1.安装

```shell
go get -u github.com/dobyte/due/config/nacos/v2@latest
```

2.etc配置项

```toml
# 配置中心
[config]
    # nacos配置中心
    [config.nacos]
        # 读写模式。可选：read-only | write-only | read-write，默认为read-only
        mode = "read-only"
        # 服务地址列表，格式：[scheme://]ip:port[/nacos]，scheme省略时默认http。默认为["http://127.0.0.1:8848/nacos"]
        urls = ["http://127.0.0.1:8848/nacos"]
        # 集群名称。默认为DEFAULT
        clusterName = "DEFAULT"
        # 分组名称。默认为DEFAULT_GROUP
        groupName = "DEFAULT_GROUP"
        # 固定文件列表（dataId）。默认为空；为空时加载并监听分组下全部配置
        files = []
        # 请求超时时间，支持单位：纳秒（ns）、微秒（us | µs）、毫秒（ms）、秒（s）、分（m）、小时（h）、天（d）。默认为3s
        timeout = "3s"
        # ACM命名空间ID。默认为空
        namespaceId = ""
        # ACM端点，使用ACM时必填，参见https://help.aliyun.com/document_detail/130146.html。默认为空
        endpoint = ""
        # ACM&KMS地域ID。默认为空
        regionId = ""
        # ACM&KMS的AccessKey。默认为空
        accessKey = ""
        # ACM&KMS的SecretKey。默认为空
        secretKey = ""
        # 是否开启KMS，DataId需以cipher-开头。默认为false
        openKMS = false
        # 服务信息缓存目录。默认为./run/nacos/config/cache
        cacheDir = "./run/nacos/config/cache"
        # Nacos服务端API认证用户名。默认为空
        username = ""
        # Nacos服务端API认证密码。默认为空
        password = ""
        # 日志存储路径。默认为./run/nacos/config/log
        logDir = "./run/nacos/config/log"
        # 日志输出级别。默认为info
        logLevel = "info"
```

3.开始使用

```go
package main

import (
    "context"
    "github.com/dobyte/due/config/nacos/v2"
    "github.com/dobyte/due/v2/config"
    "github.com/dobyte/due/v2/log"
    "time"
)

func main() {
    // 设置全局配置器
    config.SetConfigurator(config.NewConfigurator(config.WithSources(nacos.NewSource())))

    ctx := context.Background()
    filepath := "config.toml"

    // 更新配置
    if err := config.Store(ctx, nacos.Name, filepath, map[string]any{
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
    if err := config.Store(ctx, nacos.Name, filepath, map[string]any{
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

### 3.files 固定文件列表

* `files` 为空时，加载并监听 `groupName` 下的全部配置；
* `files` 非空时，只加载并监听清单中的 dataId。显式调用 `Load`/`Store` 指定的文件不在清单内时，会返回错误；
* 清单中的空项与重复项会被自动忽略；
* 精确条目直接按 dataId 读取，不触发分组搜索；监听在启动时注册一次；
* 清单外的新增配置不会被加载或监听，也不会产生变更通知。

```go
source := nacos.NewSource(
    nacos.WithFiles("config.json", "gate.yaml"),
)
```

#### 3.1 匹配规则

当前版本按**精确 dataId** 匹配（大小写敏感，不含 group 与 namespace）。

规划中的模式匹配（glob 子集：`*`、`?`、`[...]`）遵循以下约定：

* Nacos 的 blur 是服务端 `LIKE '%关键字%'` 的**包含匹配**，不是 glob；`*`、`?`、`[]` 必须由客户端做最终校验；
* 服务端 blur 只用于**包含预筛**：每个模式提取一个连续字面片段作为关键字，结果再由客户端按模式过滤；预筛可关闭；
* 无字面锚点的模式（如 `*.json`）无法预筛，会退化为分组全量分页扫描，需评估分组规模；
* 多个模式按关键字去重查询、按 dataId 取并集；服务端不支持「或/交/非」组合；
* 精确条目始终按 dataId 直接读取，不触发搜索；
* 服务端过滤语义（v1 blur 与 v3 admin list 的 dataId 参数）在不同 Nacos 版本可能存在差异，上线前需在目标版本实测。

### 4.详细示例

更多详细示例请点击[due-examples](https://github.com/dobyte/due-examples)

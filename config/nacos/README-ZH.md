# 配置中心-nacos

### 1.功能

* 支持配置的读取、修改、热更新
* 支持读写模式设置
* 支持固定文件列表（files），只加载并监听指定的 dataId，支持 `*` 通配匹配
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
        # 文件列表（dataId），支持精确 dataId 与 `*` 通配。默认为空；为空时加载并监听分组下全部配置
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

### 3.files 文件列表

* `files` 为空时，加载并监听 `groupName` 下的全部配置；
* `files` 非空时，只加载并监听被清单选中的 dataId。显式调用 `Load`/`Store` 指定的文件未被清单选中时，会返回错误；
* 条目可以是精确 dataId，也可以包含 `*` 通配符（仅支持 `*`），两种写法可以混用；
* 清单中的空项与重复项会被自动忽略；
* 仅含精确条目的清单直接按 dataId 读取，不触发分组搜索，监听在启动时注册，注册失败时按刷新周期重试；这些条目不带 md5 元数据，显式加载或 watcher 重连触发的 reload 会重新读取清单内全部 dataId 的内容；
* 含模式条目的清单在启动时与每个刷新周期搜索分组，自动为新命中的配置注册监听、为不再命中的配置取消监听；
* 未被清单选中的配置不会被加载或监听，也不会产生变更通知。

```go
source := nacos.NewSource(
    nacos.WithFiles("config.json", "gate-*.json", "rooms/*.yaml"),
)
```

#### 3.1 匹配规则

* 匹配对象是 dataId（大小写敏感，不含 group 与 namespace），只有 `*` 是通配符，匹配任意字符序列（包括 `/`），其余字符均按字面量匹配；
* 不支持 `?`、`[...]` 与 `\` 转义；dataId 本身含 `*` 时无法用清单精确表达，只能通过前后缀组合匹配；
* Nacos 的 blur 搜索把 dataId 当作通配模式：`*` 由服务端按通配符匹配（等价 SQL `%`），不含 `*` 的字面量只做精确匹配；
* 因此模式条目会原样作为 dataId 下推给服务端（如 `gate-*.json`），结果按 dataId 去重后再由客户端按相同规则过滤，保证与清单一致；
* `files` 为空时不做 dataId 过滤，按分组全量分页扫描；
* 锚点较弱的模式（如 `*.json`）匹配范围大，接近全量扫描，需评估分组规模；
* 多个模式分别查询、按 dataId 取并集；服务端不支持「或/交/非」组合；
* 精确条目始终按 dataId 直接读取，不触发搜索；含模式时结果按 dataId 升序返回，保证确定性；
* 服务端过滤语义（v1 blur 与 v3 admin list 的 dataId 参数）在不同 Nacos 版本可能存在差异，上线前需在目标版本实测。

### 4.详细示例

更多详细示例请点击[due-examples](https://github.com/dobyte/due-examples)

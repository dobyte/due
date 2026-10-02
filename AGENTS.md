### 代码规范
1. 所有所有代码严格遵循uber-go/guide编码规范，详细规范请参考：https://github.com/uber-go/guide/blob/master/style.md
2. 每次提交到git仓库时，都使用英文提交信息
3. Go代码添加注释严格参考Go语言官方文档注释形式
4. 在所有要使用到json编码解码的场景中，都必须使用github.com/dobyte/due/v2/encoding/json包
5. 在所有要使用到errors包的场景中，都必须使用github.com/dobyte/due/v2/errors包
6. 在所有高性能场景中，能不用go defer就不用

### 发布版本

#### 变动内容参考

```markdown
## 新增模块

- network/quic：全新的 QUIC 网络实现（客户端/服务端、连接管理器、流写入器、TLS 与代理协议支持）
- config/polaris：新增 Polaris 配置源（source、watcher、配置项、测试）

## 核心模块

- buffer： 统一游标模型并新增`MoveTo` ；重构游标滑动与消息解包；新增`VisitBytes` 遍历与延迟释放；修复对象池分级边界问题；空指针安全加固；基于`bufio` 实现零分配收包。
- queue： 加固等待/队列并发，补充文档与单元测试；actor 销毁后不再派发队列消息。
```

#### 发布流程参考：

1. 首先，你要检查当前是否是main分支，如果不是则不执行任何操作，如果是main分支，则从./core/info/info.go文件中的version值，这个就是本次要发布的版本号
2. 然后，你需要从git的tag中找到上次合并的日志点位，并且整理上个日志点位到当前版本间的所有提交日志，最好能够清晰地表述清楚每个模块修改的内容，清理掉重复内容，要求全部使用中文
3. 接着，用当前版本号创建一个tag，格式为v1.0.0，例如v1.0.0，然后提交到git仓库
4. 最后，你需要将发布日志整理成md文档输出到面板上，并且为了方便我进行复制，你最好使用md富文本框

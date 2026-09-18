### 注意事项
1. 所有所有代码严格遵循uber-go/guide编码规范，详细规范请参考：https://github.com/uber-go/guide/blob/master/style.md
2. 每次提交到git仓库时，都使用英文提交信息
3. Go代码添加注释严格参考Go语言官方文档注释形式
4. 在所有要使用到json编码解码的场景中，都必须使用github.com/dobyte/due/v2/encoding/json包
5. 在所有要使用到errors包的场景中，都必须使用github.com/dobyte/due/v2/errors包
6. 在所有高性能场景中，能不用go defer就不用
# JobStore 一致性测试

`conformance.Run` 为 Redis、MySQL、MongoDB 提供相同的 JobStore 语义测试。适配器测试通过 `Factory` 创建隔离的存储实例，`name` 用作 Redis key 前缀、MySQL 表前缀或 MongoDB collection 前缀。

默认不会连接外部数据库。设置对应环境变量后运行适配器测试：

```powershell
$env:JOB_REDIS_ADDR = "127.0.0.1:6379"
Push-Location cluster/job/redis
go test ./...
Pop-Location

$env:JOB_MYSQL_DSN = "user:password@tcp(127.0.0.1:3306)/database"
Push-Location cluster/job/mysql
go test ./...
Pop-Location

$env:JOB_MONGO_URI = "mongodb://127.0.0.1:27017/?replicaSet=rs0"
Push-Location cluster/job/mongo
go test ./...
Pop-Location
```

MongoDB 的 `CreateRunIfDue` 事务测试要求 Replica Set 或分片集群。测试不会自动清理外部数据库中的测试表或 collection，建议使用专用测试数据库。

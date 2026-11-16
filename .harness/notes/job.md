# Due job 分布式任务系统设计

## 1. 文档状态

| 项目 | 内容 |
| --- | --- |
| 状态 | 设计方案 |
| 模块名 | job |
| 默认存储 | Redis |
| 可选存储 | MySQL、MongoDB |
| 暂不支持 | SQLite、Due 集群远程执行、DAG 工作流 |
| 执行方式 | job Worker 本地 Handler |
| 可靠性语义 | 至少执行一次，业务 Handler 负责幂等 |

本文描述建议方案，不表示仓库已经实现这些 API。实现时以当前源码和接口评审结果为准。

## 2. 已确定的范围

### 2.1 模块边界

新增模块统一命名为 job，不新增 scheduler 包或同名公共接口。

~~~text
job/
├── job.go                 # JobService 和生命周期
├── definition.go          # Definition 任务定义
├── run.go                 # Run 执行实例
├── store.go               # JobStore 存储接口
├── planner.go             # 根据计划生成 Run
├── runner.go              # 抢占并执行 Run
├── recovery.go            # 恢复过期租约
├── retry.go               # 重试策略
├── cron.go                # Cron 计算
├── notifier.go            # 可选唤醒通知
├── capabilities.go        # 后端能力声明
├── errors.go
├── redis/                 # Redis 适配器
├── mysql/                 # MySQL 适配器
├── mongo/                 # MongoDB 适配器
└── conformance/           # 后端一致性测试
~~~

job 不替代 Due 现有的进程内任务池和 Node 内部 Actor 执行逻辑。现有 task 包仍用于进程内并发任务；Node 内部的 Actor 管理仍由现有实现负责。参考：[task/pool.go](../../task/pool.go)、[cluster/node/scheduler.go](../../cluster/node/scheduler.go)。

### 2.2 执行边界

首版只执行本地注册的通用 Handler：

~~~text
job Worker -> Handler.Execute
~~~

首版不负责：

- 将任务投递到 Gate、Node、Mesh 或 Actor。
- 通过 Due 内部路由访问其他进程。
- 玩家、房间或 Actor 状态的定向执行。
- 跨服务事务。
- DAG、任务依赖和工作流编排。

如果业务以后需要远程执行，可以在 Handler 内部自行调用外部服务；这不属于 job 核心的路由能力。

### 2.3 存储后端

| 后端 | 定位 | 多实例模式 |
| --- | --- | --- |
| Redis | 默认后端、高吞吐、低延迟 | 支持 |
| MySQL | 强持久化、关键任务 | 支持 |
| MongoDB | 文档型 Payload、灵活查询 | 支持 |
| SQLite | 暂不考虑 | 不适用 |

三种后端都必须实现相同的任务语义：唯一触发、条件抢占、租约、fencing token、幂等完成和过期恢复。

## 3. 设计目标和非目标

### 3.1 目标

- 支持一次性、延迟、固定周期和 Cron 任务。
- 多个 job Worker 并行运行时只允许一个 Worker 持有同一 Run 的执行权。
- Worker 崩溃后，其他 Worker 可以在租约过期后恢复任务。
- 任务执行结果、失败原因、重试次数和租约状态可查询。
- Redis、MySQL、MongoDB 可以通过配置切换，不改变 Handler 代码。
- 对任务创建、抢占、续租、完成和恢复提供明确的原子语义。
- 支持暂停、恢复、取消、手工触发和死信处理。
- 支持优雅停机和滚动发布。

### 3.2 非目标

- 不承诺严格恰好执行一次。
- 不把分布式锁当作业务事务。
- 不把 Eventbus 当作任务权威队列。
- 不把 task.Add 当作持久队列或无限容量执行池。
- 不自动回滚 Handler 已经产生的外部副作用。
- 不为业务 Payload 规定唯一编码格式。

## 4. 核心语义

### 4.1 至少执行一次

任务可能出现以下情况：

~~~text
Handler 已经成功
Worker 在写回成功状态前崩溃
其他 Worker 看到租约过期后再次执行
~~~

因此 job 只保证任务可以被可靠地再次尝试，不能凭网络响应判断业务副作用是否只发生了一次。

业务 Handler 必须使用 RunID 或业务幂等键保护副作用，例如数据库唯一键、条件更新或业务账本。

### 4.2 租约和 fencing token

每次抢占都会生成递增的 FencingToken。所有续租、开始、成功和失败写回都必须携带：

~~~text
run_id + lease_owner + fencing_token
~~~

旧 Worker 即使在暂停后恢复，也不能使用旧 token 覆盖新 Worker 的结果。

### 4.3 时间

- 持久化时间统一使用 UTC。
- Cron 表达式可以声明时区。
- 后端适配器应尽量使用存储端时间完成到期和租约判断。
- time.Time 的序列化格式必须在各适配器中保持一致。

## 5. 总体架构

~~~mermaid
flowchart LR
    C[业务调用方] --> S[JobService]
    S --> D[JobStore]
    D --> X[(Redis / MySQL / MongoDB)]

    P[Planner] --> D
    R[Runner] --> D
    V[Recovery] --> D
    R --> H[本地 Handler]

    N[Notifier 可选] --> P
    D -.任务变更通知.-> N
~~~

一个 job 进程可以同时运行 JobService、Planner、Runner 和 Recovery。生产环境建议部署多个相同 Worker：

~~~text
job-1 ─┐
job-2 ─┼── Redis / MySQL / MongoDB
job-3 ─┘
~~~

各 Worker 都可以轮询和抢占，但真正的执行权由后端的原子操作决定。

## 6. 核心组件

### 6.1 JobService

负责对外管理任务定义和执行实例：

- 创建、修改、暂停、恢复和删除任务定义。
- 手工触发任务。
- 查询任务和执行记录。
- 校验任务类型、计划、Payload 和重试配置。

### 6.2 Planner

负责将到期的 Definition 转换成一次 Run：

- 查找 next_run_at <= now 的任务。
- 计算 scheduled_at。
- 创建唯一执行实例。
- 推进 next_run_at。
- 处理错过执行时间。

### 6.3 Runner

负责：

- 抢占 pending 或重试到期的 Run。
- 取得租约和 fencing token。
- 查找本地 Handler。
- 创建超时 Context。
- 启动租约续期。
- 写回成功或失败状态。

### 6.4 Recovery

负责发现 lease_until < now 的 Run，并将其恢复为可执行状态。

### 6.5 HandlerRegistry

维护任务类型和 Handler 的映射：

~~~go
registry.Register("cleanup_expired_data", cleanupHandler)
~~~

## 7. 数据模型

### 7.1 Definition

Definition 表示一个长期存在的任务定义。

~~~go
type Definition struct {
    ID                string
    Name              string
    Type              string
    Payload           []byte

    ScheduleType      ScheduleType
    ScheduleExpr      string
    Timezone          string
    NextRunAt         time.Time

    Status            DefinitionStatus
    MisfirePolicy     MisfirePolicy
    ConcurrencyPolicy ConcurrencyPolicy

    MaxAttempts       int
    RetryPolicy       RetryPolicy
    Timeout           time.Duration

    Version           int64
    CreatedAt         time.Time
    UpdatedAt         time.Time
}
~~~

支持的计划类型：

~~~go
type ScheduleType string

const (
    ScheduleOnce  ScheduleType = "once"
    ScheduleDelay ScheduleType = "delay"
    ScheduleFixed ScheduleType = "fixed"
    ScheduleCron  ScheduleType = "cron"
)
~~~

### 7.2 Run

Run 表示一次具体执行，不等同于任务定义。

~~~go
type Run struct {
    ID           string
    JobID        string
    ScheduledAt  time.Time

    Status       RunStatus
    Attempt      int

    LeaseOwner   string
    LeaseUntil   time.Time
    FencingToken int64

    StartedAt    *time.Time
    FinishedAt   *time.Time

    ErrorCode    string
    ErrorMessage string
    ResultRef    string

    CreatedAt    time.Time
    UpdatedAt    time.Time
}
~~~

必须保证：

~~~text
UNIQUE(job_id, scheduled_at)
~~~

### 7.3 状态

任务定义状态：

~~~text
enabled -> paused -> enabled
enabled -> deleted
~~~

执行实例状态：

~~~text
pending -> leased -> running -> succeeded
                         ├──> retry_wait -> pending
                         ├──> dead
                         └──> canceled

leased/running --租约过期--> pending
~~~

## 8. 完整调用链

### 8.1 启动

~~~text
JobService.Start
    ├── 校验 JobStore Capabilities
    ├── 启动 Planner Loop
    ├── 启动 Runner Loop
    ├── 启动 Recovery Loop
    ├── 启动可选 Notifier
    └── 设置 Ready
~~~

### 8.2 创建任务

~~~text
业务调用方
    -> JobService.Create
    -> 校验任务类型和计划
    -> JobStore.CreateDefinition
    -> Redis Hash / MySQL 行 / Mongo 文档
    -> 写入到期索引
    -> 可选发布 job.definition.created
    -> 返回 Definition
~~~

### 8.3 生成 Run

~~~text
Planner
    -> ListDueDefinitions(now)
    -> CreateRunIfDue(job_id)
    -> 校验 Definition 仍为 enabled
    -> 创建唯一 Run
    -> 推进 next_run_at
    -> 写入 pending 索引
~~~

CreateRunIfDue 必须将“创建 Run”和“推进 next_run_at”作为一个原子操作。两个 Planner 同时处理同一任务时，只有一个调用返回 created = true。

### 8.4 抢占和执行

~~~text
Runner
    -> JobStore.ClaimRuns
    -> 设置 lease_owner
    -> 设置 lease_until
    -> fencing_token + 1
    -> attempt + 1
    -> StartRun
    -> 创建带超时的 Context
    -> 启动租约续期
    -> Handler.Execute
~~~

### 8.5 成功

~~~text
Handler.Execute 返回成功
    -> JobStore.CompleteRun
    -> 校验 owner 和 fencing token
    -> status = succeeded
    -> 保存 result_ref
    -> 删除租约和 active 标记
~~~

### 8.6 失败

~~~text
Handler.Execute 返回错误
    -> 分类错误
    -> JobStore.FailRun
       ├── 可重试且未超过次数 -> retry_wait
       └── 不可重试或超过次数 -> dead
~~~

### 8.7 崩溃恢复

~~~text
Worker 崩溃
    -> lease_until 到期
    -> Recovery.RecoverExpiredRuns
    -> 清理旧 owner
    -> 恢复为 pending
    -> 其他 Worker 重新 ClaimRuns
~~~

### 8.8 停止

~~~text
JobService.Close
    -> 停止接受 Create / Trigger
    -> 停止 Planner
    -> 停止 ClaimRuns
    -> 停止 Recovery
    -> 等待本地 Handler 到达关闭期限
    -> 停止租约续期
    -> 关闭 Store 和 Notifier
~~~

## 9. JobStore 接口

接口表达任务语义，不暴露数据库 CRUD 细节：

~~~go
type JobStore interface {
    Close(ctx context.Context) error
    Capabilities() Capabilities

    CreateDefinition(ctx context.Context, definition *Definition) error
    GetDefinition(ctx context.Context, jobID string) (*Definition, error)
    UpdateDefinition(ctx context.Context, req UpdateDefinitionRequest) (*Definition, error)
    PauseDefinition(ctx context.Context, jobID string, version int64) error
    ResumeDefinition(ctx context.Context, jobID string, version int64) error
    DeleteDefinition(ctx context.Context, jobID string, version int64) error

    ListDueDefinitions(ctx context.Context, req ListDueRequest) ([]*Definition, error)
    CreateRunIfDue(ctx context.Context, req CreateRunRequest) (*Run, bool, error)

    GetRun(ctx context.Context, runID string) (*Run, error)
    ClaimRuns(ctx context.Context, req ClaimRequest) ([]*ClaimedRun, error)
    StartRun(ctx context.Context, req StartRunRequest) error
    RenewLease(ctx context.Context, req RenewLeaseRequest) error
    CompleteRun(ctx context.Context, req CompleteRunRequest) (CommitResult, error)
    FailRun(ctx context.Context, req FailRunRequest) (CommitResult, error)
    CancelRun(ctx context.Context, req CancelRunRequest) error
    RecoverExpiredRuns(ctx context.Context, req RecoverRequest) (int, error)

    GetStats(ctx context.Context, req StatsRequest) (*Stats, error)
}
~~~

抢占、续租、完成和恢复必须是后端条件原子操作，不能在业务层拆成“先查询，再更新”。

## 10. Redis 默认实现

### 10.1 Key

统一前缀建议为 due:job:：

~~~text
due:job:def:{job_id}          Definition Hash
due:job:def:due               Definition 到期 Sorted Set
due:job:run:{run_id}          Run Hash
due:job:run:pending            pending Run Sorted Set
due:job:run:retry              retry_wait Run Sorted Set
due:job:active:{job_id}       forbid 并发标记
due:job:worker:{worker_id}    Worker 心跳
due:job:cancel:{run_id}       协作式取消标记
~~~

### 10.2 Sorted Set

~~~text
due:job:def:due
score  = next_run_at Unix 时间戳
member = job_id
~~~

~~~text
due:job:run:pending
score  = ready_at Unix 时间戳
member = run_id
~~~

~~~text
due:job:run:retry
score  = retry_at Unix 时间戳
member = run_id
~~~

### 10.3 Lua 脚本

建议至少拆分为：

~~~text
create_definition.lua
update_definition.lua
create_run_if_due.lua
claim_runs.lua
start_run.lua
renew_lease.lua
complete_run.lua
fail_run.lua
cancel_run.lua
recover_expired_runs.lua
~~~

脚本要求：

- 返回稳定的错误码和结果结构。
- 不在脚本中执行网络调用。
- 带脚本版本标识。
- Redis Cluster 下使用相同 hash tag。
- 所有状态迁移都更新对应索引。

生产环境建议开启 AOF，并监控主从切换和复制延迟。关键任务的业务账本仍应使用业务数据库保护，Redis 不能替代业务事务。

## 11. MySQL 适配器

使用 InnoDB 和事务实现：

~~~text
job_definition 表
job_run 表
可选 job_outbox 表
~~~

需要的约束和索引：

~~~text
UNIQUE(job_id, scheduled_at)
INDEX(status, scheduled_at, lease_until)
INDEX(job_id, created_at)
~~~

如果数据库版本支持，可以使用 FOR UPDATE SKIP LOCKED 批量抢占；否则使用条件更新和唯一 token 实现等价语义。

## 12. MongoDB 适配器

建议使用两个集合：

~~~text
job_definition
job_run
~~~

建立唯一索引：

~~~javascript
db.job_run.createIndex(
  { job_id: 1, scheduled_at: 1 },
  { unique: true }
)
~~~

findOneAndUpdate 负责抢占和续租。生产环境建议使用 Replica Set，以便在“创建 Run + 推进 next_run_at”时使用事务。

## 13. 任务策略

### 13.1 错过执行

~~~go
type MisfirePolicy string

const (
    MisfireSkip       MisfirePolicy = "skip"
    MisfireFireOnce   MisfirePolicy = "fire_once"
    MisfireFireAll    MisfirePolicy = "fire_all"
    MisfireFireLatest MisfirePolicy = "fire_latest"
)
~~~

默认建议为 fire_once。

### 13.2 并发

~~~go
type ConcurrencyPolicy string

const (
    AllowConcurrent ConcurrencyPolicy = "allow"
    ForbidConcurrent ConcurrencyPolicy = "forbid"
    QueueConcurrent  ConcurrencyPolicy = "queue"
    ReplacePrevious  ConcurrencyPolicy = "replace"
)
~~~

默认建议为 forbid。并发限制必须在后端原子操作中判断，不能只使用 Worker 本地锁。

### 13.3 重试

~~~go
type RetryPolicy struct {
    MaxAttempts int
    Initial     time.Duration
    MaxDelay    time.Duration
    Multiplier  float64
    Jitter      float64
}
~~~

错误分为：

~~~text
retryable
non_retryable
canceled
lease_lost
~~~

## 14. Handler 接口

~~~go
type Handler interface {
    Execute(context.Context, *Request) (*Result, error)
}

type Request struct {
    RunID        string
    JobID        string
    TaskType     string
    Payload      []byte
    Attempt      int
    FencingToken int64
    Deadline     time.Time
}

type Result struct {
    Output    []byte
    ResultRef string
}
~~~

Handler 约束：

- 必须尊重 Context 的取消和超时。
- 必须使用 RunID 或业务键实现幂等。
- 不应自行修改 Run 状态。
- 不应自行续租。
- 不应把未完成的异步 goroutine 当作执行成功。

## 15. JobService API

~~~go
type Service interface {
    Create(ctx context.Context, req CreateRequest) (*Definition, error)
    Update(ctx context.Context, req UpdateRequest) (*Definition, error)
    Pause(ctx context.Context, jobID string) error
    Resume(ctx context.Context, jobID string) error
    Delete(ctx context.Context, jobID string) error
    Trigger(ctx context.Context, jobID string) (*Run, error)
    Get(ctx context.Context, jobID string) (*Definition, error)
    GetRun(ctx context.Context, runID string) (*Run, error)
}
~~~

Trigger 只创建一条新的手工 Run，不修改原有周期任务的 next_run_at。

## 16. 取消和停机

取消状态：

~~~text
pending/retry_wait/leased -> canceled
running -> cancel_requested -> canceled
~~~

运行中取消只能是协作式取消。Handler 已经完成的外部副作用不会自动回滚。

停机顺序：

1. 标记 Worker 不再接收新请求。
2. 停止 Planner 生成新 Run。
3. 停止 Runner 抢占新 Run。
4. 等待当前 Handler 到达停机期限。
5. 停止租约续期。
6. 关闭 Store 和 Notifier。

## 17. 配置建议

~~~toml
[job]
enabled = true
worker_id = "job-worker-01"
poll_interval = "1s"
claim_batch_size = 100
lease_duration = "30s"
recovery_interval = "5s"

[job.store]
driver = "redis"

[job.store.redis]
addr = "127.0.0.1:6379"
db = 0
key_prefix = "due:job"
~~~

可选驱动：

~~~text
redis
mysql
mongo
~~~

SQLite 不进入配置和适配器列表。

## 18. 能力声明

~~~go
type Capabilities struct {
    Transactions       bool
    MultiInstanceClaim bool
    ServerTime         bool
    AtomicSchedule     bool
    ConditionalWrite   bool
    BatchClaim         bool
    Durable            bool
    Outbox             bool
}
~~~

JobService 启动时至少要求：

~~~text
Durable = true
AtomicSchedule = true
ConditionalWrite = true
MultiInstanceClaim = true
~~~

如果后端不满足要求，应在启动阶段返回配置错误，而不是运行后降级为不明确的任务语义。

## 19. 观测

至少记录：

~~~text
job_definitions_created_total
job_runs_created_total
job_runs_claimed_total
job_runs_succeeded_total
job_runs_failed_total
job_runs_retried_total
job_runs_dead_total
job_runs_canceled_total
job_queue_lag_seconds
job_run_duration_seconds
job_lease_lost_total
job_recovery_total
job_handler_rejected_total
~~~

日志字段建议包含：

~~~text
job_id
run_id
task_type
attempt
worker_id
fencing_token
trace_id
~~~

## 20. 一致性测试

三个后端共用同一套 conformance 测试：

1. 两个 Worker 同时抢占同一 Run，只有一个成功。
2. 同一 job_id + scheduled_at 只能生成一条 Run。
3. 旧 fencing token 不能完成新租约。
4. 租约过期后 Run 可以恢复。
5. 暂停 Definition 后不会生成新 Run。
6. 取消和抢占并发时状态不会回退。
7. 重试次数和退避时间符合配置。
8. Worker 重启后 pending 和 expired Run 可以继续处理。
9. 数据库短暂不可用时不会把任务误标记为成功。
10. Redis、MySQL、Mongo 的公开语义一致。

## 21. 实施阶段

### Phase 1：核心契约

- 固定 Definition、Run 和状态机。
- 固定 JobStore 接口。
- 固定 Handler 和 Service 接口。
- 固定 Redis Key 和错误语义。

### Phase 2：Redis 实现

- Redis Store。
- Lua 脚本。
- 本地 Handler Registry。
- Planner、Runner 和 Recovery。
- 一次性和延迟任务。
- 重试、租约和恢复测试。

### Phase 3：计划能力

- 固定周期。
- Cron 和时区。
- MisfirePolicy。
- 并发策略。
- 暂停、恢复、取消和手工触发。

### Phase 4：MySQL 和 MongoDB

- 独立 Go module。
- 适配器迁移脚本。
- 复用 conformance 测试。
- 核对事务、条件写和批量抢占语义。

### Phase 5：生产能力

- 指标和告警。
- Outbox 或可选通知。
- 管理接口。
- 任务历史归档。
- Redis 故障切换和长任务演练。

## 22. 与现有 Due 模块的关系

| 模块 | 关系 |
| --- | --- |
| task | 可作为本地辅助并发工具，但不承担持久队列语义 |
| lock | 可用于辅助协调，不作为任务正确性的唯一依据 |
| eventbus | 可用于唤醒和通知，不能替代 Run 持久状态 |
| cache | 可用于非权威缓存，不保存唯一任务状态 |
| cluster/node | 首版不接入，任务只执行本地 Handler |
| container | job 组件遵循 Init/Start/Close/Destroy 生命周期 |

当前框架的生命周期、任务池、锁和事件边界可参考：[Due 架构设计](./architecture.md)。

## 23. 当前结论

job 的 V1 方案采用：

~~~text
Redis 默认
MySQL/MongoDB 可替换
本地 Handler 执行
至少一次语义
租约 + fencing token
任务定义与执行实例分离
数据库状态为权威
通知只用于唤醒
SQLite 暂不考虑
~~~

后续实现应先完成接口和状态机评审，再从 Redis 适配器开始编码，最后用同一套一致性测试验证 MySQL 和 MongoDB。

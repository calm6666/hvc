# HVC 单机与集群部署手册

> 更新日期：2026-05-10

## 1. 先明确两类配置

本项目现在严格分成两类配置：

- bootstrap 配置：只放在 `configs/config.yaml` 或外部 bootstrap 配置源里
- runtime 配置：只放在 MySQL `t_runtime_config`，通过后台接口维护，并通过 Redis 缓存加速读取

bootstrap 只负责：

- `server`
- `mysql`
- `redis`
- `internal_grpc`
- `config_center`
- `id`

runtime 负责：

- HTTP 对外模块启停
- public gRPC 启停、监听地址、etcd 注册
- MQ 创建任务消费
- callback 默认兜底
- storage
- scheduler / worker 并发与安全阈值

## 2. `server.node_mode` 的含义

`server.node_mode` 是当前部署的根语义，必须显式配置。

可选值：

- `standalone`
- `cluster-control`
- `cluster-worker`
- `cluster-allinone`

含义如下：

- `standalone`：单机一体化，适合开发机、小规模部署
- `cluster-control`：控制面节点，只跑后台/调度/管理，不承担转码执行
- `cluster-worker`：执行节点，只跑 worker，不直接对外暴露后台
- `cluster-allinone`：集群一体化节点，既有后台/调度，也能承担转码执行

注意：

- `node_mode` 是部署角色，属于 bootstrap 语义
- 后台热更新关闭某个模块后，`node_mode` 不会变化
- 后台接口里的 `mode` 是当前激活模块组合，`node_mode` 是部署角色，两者不要混用
- `node_mode` 还会对本节点最终生效的模块边界做硬约束：
  - `cluster-control` 不能开启 `worker`
  - `cluster-worker` 不能开启 `http/public gRPC/mq consumer/callback/scheduler`
- 也就是说 runtime config 是全局共享快照，但不会把控制面能力热改到执行节点上，反之亦然

## 3. 单机部署

### 3.1 适用场景

- 单台服务器
- 单机多卡
- 开发 / 测试 / 小规模生产

### 3.2 最小依赖

- MySQL 8.0+
- Redis 7+
- 可选 etcd 3.5+（如果要用 public gRPC etcd 注册或 gRPC callback etcd 发现）
- 可选 RabbitMQ（如果要用 MQ 创建任务或 MQ 回调）
- 可选 MinIO / S3

### 3.3 bootstrap 配置示例

```yaml
server:
  listen_address: ":8888"
  advertise_ip: "10.0.0.10"
  node_mode: "standalone"
  service_name: "hili-video-cloud"
  node_id: 1
  worker_id: "worker-standalone-1"

mysql:
  dsn: "root:123456@tcp(127.0.0.1:3306)/hvc?parseTime=true&charset=utf8mb4"

redis:
  addrs:
    - "127.0.0.1:6379"
  password: ""
  db: 0
  cluster_enabled: false

internal_grpc:
  enabled: true
  listen_address: ":19090"
  shared_token: "replace-me"

config_center:
  enabled: false

id:
  start_time: "2025-01-01T00:00:00Z"
  node_bits: 10
  sequence_bits: 12
```

### 3.4 初始化步骤

1. 创建数据库 `hvc`
2. 用 UTF-8 导入 `sql/000_full_project_schema.sql`
3. 启动 MySQL / Redis
4. 启动服务
5. 访问 `/healthz`
6. 登录后台发布第一版 runtime 配置

补充：

- 如果使用 `scripts/migrate/migrate.sh`，默认就是 `full` 模式，只执行 `sql/000_full_project_schema.sql`
- `sql/100_*.sql` 及之后的文件用于存量库增量升级，不建议在全新初始化时再混跑一遍

### 3.6 Docker Compose 单机演示

`deployments/docker/docker-compose.yml` 现在是“可直接启动的单机演示栈”，包含：

- `hvc-server`
- `mysql`
- `redis`
- `etcd`
- `rabbitmq`
- `minio`
- `minio-init`
- `nginx`

说明：

- 适合本地联调、单机验证、接口联调
- 不等价于真实多机集群
- 北向 HTTP 统一入口默认是 `127.0.0.1:8088`
- 北向 public gRPC 统一入口默认是 `127.0.0.1:9091`
- Compose 使用独立 bootstrap 配置 `deployments/docker/config.compose.yaml`，内部 MySQL/Redis 地址已对齐容器服务名
- Compose 首次初始化 MySQL 时只导入 `sql/000_full_project_schema.sql`，不会把历史增量 SQL 再混跑一遍

### 3.5 单机多卡

单机多卡不需要额外部署多个 worker 进程也能工作，当前实现会：

- 采集所有 GPU 能力
- 记录 `t_node_gpu_device`
- 调度时按 GPU 能力和会话占用挑卡

当前最佳实践：

- 一台机器一进程先跑通
- 通过 `max_node_transcode_sessions`、`max_node_upload_concurrency` 控制并发
- 依赖 GPU 实时指标避免把单卡打满

如果后续要做“一机多 worker 进程 + 每进程绑卡”，可以继续往下拆，但不是当前必须条件。

## 4. 集群部署

## 4.1 推荐拓扑

推荐至少拆成两类节点：

- 控制面节点：`cluster-control` 或 `cluster-allinone`
- 执行节点：`cluster-worker`

推荐依赖：

- MySQL：共享
- Redis：共享
- etcd：共享
- Nginx / SLB：对外统一入口

### 4.2 角色分工

#### `cluster-control`

负责：

- 后台管理 HTTP
- scheduler
- callback
- runtime config 同步
- cluster overview / topology / realtime

不负责：

- 真正执行转码

#### `cluster-worker`

负责：

- 拉取分配任务
- 执行转码
- 上传分片
- 上报 GPU / CPU / 内存指标

不负责：

- 对外后台管理入口

#### `cluster-allinone`

适合中小规模集群，既能控制又能执行。

### 4.3 控制面节点 bootstrap 示例

```yaml
server:
  listen_address: ":8888"
  advertise_ip: "10.0.0.11"
  node_mode: "cluster-control"
  service_name: "hili-video-cloud"
  node_id: 11
  worker_id: "worker-control-11"
```

### 4.4 执行节点 bootstrap 示例

```yaml
server:
  listen_address: ":8888"
  advertise_ip: "10.0.0.21"
  node_mode: "cluster-worker"
  service_name: "hili-video-cloud"
  node_id: 21
  worker_id: "worker-node-21"
```

说明：

- `cluster-worker` 的 `server.listen_address` 仍需要保留在 bootstrap 中，保持配置结构一致
- 但 worker 节点最终会被 `node_mode` 硬约束为不启动北向 HTTP；也不会启动 public gRPC、MQ consumer、callback、scheduler
- 因此后台管理入口、北向 RPC、MQ 创建任务消费应只落在 `cluster-control` 或 `cluster-allinone`

### 4.5 Kubernetes 注意事项

当前仓库里的 K8s 示例已经拆成：

- `deployments/k8s/deployment.yaml`：control-plane 单副本
- `deployments/k8s/worker-deployment.yaml`：worker 单副本

必须强调：

- 当前架构下 `node_id` 必须全局唯一
- `worker_id` 也必须唯一
- 因此不能把同一份带固定 `node_id` 的 Deployment 直接扩成多个副本

如果要做真正的 K8s 多副本集群，推荐：

1. control-plane 使用 `StatefulSet`
2. worker 使用 `StatefulSet` 或者为每个 Deployment 显式分配唯一 `node_id`
3. 用 ConfigMap/模板渲染为每个 Pod 注入不同 `node_id` / `worker_id`
4. 不要把当前仓库里的 control-plane 示例直接套 HPA 扩成多副本

补充：

- `worker-deployment.yaml` 中的 worker 节点不再依赖 HTTP `/healthz` 做探针
- worker 角色默认不启动北向 HTTP，探针应改为 `tcpSocket: 19090`，直接检查 `internal_grpc` 监听

## 5. public gRPC、internal gRPC、etcd 的边界

### 5.1 internal gRPC

`internal_grpc` 是 bootstrap 固定入口：

- 节点启动即初始化
- 不参与热更新
- 用于集群内部通信

### 5.2 public gRPC

`public gRPC` 是 runtime 管理能力：

- 可热启停
- 可热改监听地址
- 可通过 `public_grpc_registry_id` 绑定 etcd 注册配置

当 `public_grpc_registry_id > 0` 时：

- 服务启动后自动把当前实例接入点写入 etcd
- key 形如 `/{service_namespace}/transcode.v1.TranscodePublicService/{endpoint}`
- 配置发布导致 registry 变化时，会自动重建注册

### 5.3 gRPC 回调

回调支持两种 gRPC 目标：

- 固定地址：`rpc_endpoint + rpc_service_name`
- 动态发现：`registry_id + rpc_service_name`

因此 etcd 不只是“项目里有代码”，而是已经进入真实业务链路。

## 6. Nginx / SLB 推荐用法

对外统一入口建议：

- HTTP 后台与公共接口走 Nginx / SLB
- public gRPC 也走统一入口

这样有三个好处：

- 客户端不用记单机 IP
- 控制面节点切换更平滑
- 可以逐步替换后端节点而不改调用方

推荐分层：

- Nginx / LB 负责北向稳定入口
- etcd 负责服务注册 / 发现
- runtime config 负责业务暴露策略

补充：

- 仓库内 `deployments/nginx/hvc.conf` 是生产向入口模板
- 仓库内 `deployments/nginx/hvc.docker.conf` 是 Docker 单机演示模板

不要混成：

- 一部分客户端手写 IP
- 一部分客户端走 Nginx
- 一部分客户端走 etcd

生产里应该统一。

## 7. Redis 在集群模式下的职责

当前项目里 Redis 负责：

- runtime config 缓存
- 进度热路径
- 节点状态缓存
- lease 热路径

补充：

- 后台 `cluster overview / realtime / resource distribution` 这类高频轮询接口，当前已把任务状态计数、按节点活跃任务数、Worker 总数与在线数下推为数据库聚合查询
- 节点实时指标读取也已切到按节点集合批量执行 Redis `MGET`，控制面不会再对每台节点逐个发起 Redis 往返
- `overview` / `realtime` 的 GPU 汇总也已下推到数据库聚合，控制面轮询不再为纯汇总目的构造整批 GPU 明细对象
- 调度器 `scheduler.insight` 和 WebSocket 监控快照也已复用同一套批量 metrics 读取路径，避免后台巡检和在线监控重复放大 Redis 压力
- Worker 写入 `t_cluster_node` 的节点主档快照现已做低频持久化；高频实时指标只进 Redis 热路径，避免按秒级回写节点表
- Worker / control-plane 节点心跳也已改成 Redis 高频、MySQL 节流持久化；数据库只保留可审计的最近心跳时间，不再每轮循环都更新 `t_cluster_node`、`t_worker_instance`
- 转码执行链路里的任务进度与执行实例心跳也已做同类收敛：实时进度仍进 Redis，任务主表与 `t_transcode_job_execution` 只保留近实时数据库快照，避免 ffmpeg 高频事件直接放大为高频写库
- 当前任务执行失联判定的权威心跳来源是 `t_transcode_job_execution.last_heartbeat_at`；`t_transcode_job.last_worker_heartbeat_at` 仅作历史兼容字段保留，不再作为新的失联判断依据
- 运行日志也已改成异步批量入库：业务线程只负责入队，后台批量写 `t_system_runtime_log`，并带失败重试，避免日志持久化反向放大业务请求延迟
- HTTP 中间件对高频路径已做降噪：健康检查、清单接口、监控快照、进度轮询、内部 heartbeat/lease 等路径不再捕获完整 request/response body，降低日志体积与序列化开销
- 后台 `overview / realtime / topology / resource distribution / monitor snapshot` 这类高频 polling 接口现已补上 2~3 秒 Redis 摘要缓存，同一时间窗内的重复刷新会直接命中缓存，进一步降低控制面重复拼装与重复查询成本
- 调度循环也不再每轮全量扫描 queued 任务，而是按剩余全局容量拉取有限批次任务，降低队列堆积时的数据库压力
- 也就是说控制面高频轮询不再依赖“全表拉回后在进程内计数”的旧路径，数据库压力会更可控

推荐要求：

- 所有节点连接同一个 Redis 集群或同一组主从
- runtime config 发布后先删缓存再回填
- 其它节点通过 Redis 版本探测收敛到最新配置

## 8. MySQL / 分库分表

当前 bootstrap 已有：

- `mysql.sharding_enabled`
- `mysql.shard_count`

但这只是基础配置位，不代表当前所有 repository 都完成了真实分库分表路由。

所以现在的准确结论是：

- 项目已经为分库分表预留 bootstrap 位
- 但是否“完全支撑高并发分库分表生产态”，还不能只靠这两个字段判断

这一块如果你要我继续做，我下一轮会按 repository 实际实现把“已落地 / 未落地”逐项点出来并补齐。

## 9. 后台应该连哪台服务器

集群模式下，后台管理入口应该优先连：

- `cluster-control`
- 或 `cluster-allinone`

不建议直接把后台流量打到 `cluster-worker`。

原因：

- worker 角色不应该承担主要管理入口
- 后台接口需要完整的 cluster overview / scheduler / audit / config publish 能力
- 控制面更适合挂在稳定入口和权限体系后面

## 10. 当前项目此轮已经落地的关键点

- `node_mode` 已进入 bootstrap
- 节点角色不再依赖运行时开关猜测
- cluster overview / realtime / topology 已新增 `node_mode`
- `configs/config.yaml` 与 `deployments/` 样例已同步
- public gRPC etcd 注册已接入真实服务生命周期

## 11. 建议的生产落地顺序

1. 先按单机 `standalone` 跑通
2. 再拆出 `cluster-control`
3. 再增加一个或多个 `cluster-worker`
4. 再接入 Nginx / SLB
5. 再启用 public gRPC etcd 注册
6. 再启用 MQ 创建任务 / MQ 回调

这样更稳，不会一次把变量引爆。

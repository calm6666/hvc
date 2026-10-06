# hvc

视频转码与直播处理服务（Go）。

> A video transcoding and live-processing service in Go: one codebase serving standalone, cluster-control,
> cluster-worker and all-in-one modes, with MinIO object storage, RabbitMQ and Redis on the hot path.

---

## 项目定位

`hvc` 是面向视频平台的处理后端：接收转码任务 → 调度到具备相应硬件能力的节点 → 探测、计划、切片、
上传 → 发布清单 → 回调通知业务方；同时承载直播链路的推流、转码与回放。

一个项目、一个入口（`cmd/main.go`），**同一套代码**支持四种运行方式：

| 模式 | 说明 |
|---|---|
| 单机模式 | 控制面与执行面在同一进程内，适合开发与单机部署 |
| 集群控制节点 | 只做调度、租约、状态汇总，不跑转码 |
| 集群执行节点 | 只接收任务并执行，向控制节点心跳与上报 |
| 集群一体化 | 两种角色都承担，用于小规模集群 |

## 设计原则（摘自 `项目总原则.md`）

- 编码**必须硬编**；解码优先硬解，仅在源不支持硬解时允许软解；软解 CPU 总占用默认不超过 50%
- 水印、缩放、overlay 优先走硬件链路
- MySQL **只做持久化**；Redis 与低延迟通信承担热路径
- 限流、并发、热路径后端、RBAC、上传策略、故障策略都由后台动态配置控制
- 结构先定死再写代码；框架代码（go-zero / protoc 生成物）必须生成而非手写

## 代码结构

```
cmd/                唯一入口 main.go
api/                proto（public/internal/shared）、openapi（public/admin/internal）、mq
internal/           领域实现，20 个包：
                      auth audit config configcenter cluster callback
                      domain handler infra interfaces live manifest
                      model opslog scheduler server service usecase worker
pkg/                通用工具（errorsx / idgen / logx / netutil / retryx / timex）
configs/            各运行模式的配置
deployments/        docker / k8s / nginx / systemd
scripts/            build / release / migrate / dev
sql/                000_full_project_schema.sql（全量） + 编号增量迁移
test/               integration / e2e / benchmark
web/                内嵌管理台前端
```

规模（当前）：约 1000 个 Go 源文件 / 5.7 万行（不含测试），测试文件 60+；HTTP 路由注册点 150+，
gRPC 服务方法 70+，SQL 迁移 10+。

## 任务数据流

```
提交任务 → 调度（节点/GPU 能力筛选、评分、租约）→ 执行节点：探测 → 计划 → 切片 → 上传
        → 发布（清单物化 + 逐片校验：可 GET 且 sha256 通过）→ 回调（outbox：HTTP/gRPC/MQ 任一成功即算送达）
```

状态机（转码任务）：

```
CREATED → QUEUED → ASSIGNED → RUNNING → UPLOADING → COMPLETED → PUBLISHED → CALLBACK_SENT
                                                        ↓
                                                     FAILED / CANCELED
```

- `COMPLETED` 只表示"内容已产出"；分片由**全局待传队列**异步上传，别的节点也可能在传这个任务的分片
- `PUBLISHED` 表示"清单已物化、每个分片可 GET 且 sha256 通过"，下游此刻拉取必然完整
- `CALLBACK_SENT` 表示完成回调已由至少一个通道送达，是对外承诺兑现的最终终态

## 本次迭代新增的能力

### A1 回调状态机（完成）

- 状态新增 `PUBLISHED` / `CALLBACK_SENT`；`CanPublish()` 仅允许从 `COMPLETED` 进入，
  `CanSendCallback()` 仅允许从 `PUBLISHED` 进入 —— 保证下游收到回调时内容已完整可读
- 仓储 `MarkPublished` / `MarkCallbackSent` 都用 `status` 做条件更新：多节点竞争或乱序回写不会把状态拉回
- 转码结束时**不再立刻发回调**：先把完成载荷 `SavePendingCompletion` 落库暂存（因为分片还没传完），
  落库失败则退回旧行为，避免任务永远不回调
- `worker/publish_completion.go`：
  - `publishCompletion`：本任务待传分片数归零 → 逐片校验（已上传 + 对象回执 + 摘要齐备）
    → `TakePendingCompletion`（取+清的比较交换，天然只发一次）→ `MarkPublished` → 此刻才写 outbox 事件
  - `ReconcilePublish`：复用 Application 已有的 1 秒对账循环，兜住"最后一片由别的节点上传"
    与"本进程在两次上传之间重启"这两类没有本地唤醒的情况
- 分片上传批处理结束后就地触发发布（比等对账更及时）
- 回调投递成功（outbox `MarkDelivered`）后 `MarkCallbackSent` ⇒ 状态机闭合

### B2 断点续跑（完成；跳过范围见"现状与边界"）

- `sql/109_transcode_job_step_schema.sql`：`t_transcode_job_step`（job_id, step, state, input_hash,
  detail, attempt, started_at, finished_at，`UNIQUE(job_id, step)`）+ `t_transcode_job.input_hash`
- 仓储：`UpsertStep`（先查后写；`Running` 累加 attempt 并记开始时间，`Done`/`Failed` 记结束时间；
  空指纹不覆盖已记指纹）、`ListByJob`、`IsStepDone`、`MarkStepDone`，并在 `JobRepository` 上做薄封装
- 指纹（`worker/job_input_hash.go`）：源 URL + 切片时长/模板 + DASH/HLS + 输出存储/前缀 + 水印 +
  缩略图 + 清晰度集合（排序后拼接）。**刻意不含源文件大小/ETag**：那两项在网络抖动时可能取不到，
  放进去会把"这次没取到"误判成"输入变了"而白白全量重跑
- 执行入口比对指纹：空则记录；一致则允许续跑；不一致则清步骤行 + 重置指纹 ⇒ 整任务重跑
  （日志打出 old_hash/new_hash）。PROBE/PLAN/SEGMENT/UPLOAD 四步写 Running/Done，
  并输出 `worker.job.resume_decision` 日志（含各步骤完成状态）

### B3 清单物化（完成）

- 切片产出时计算内容摘要并随分片行落库（`worker/segment_digest.go`）
- 发布校验要求每片摘要非空 ⇒ "sha256 通过"进入发布判据
  （对象存储的 ETag/大小只能证明"传上去了、大小对"，证明不了内容）
- 回调载荷携带每片摘要：`CompletedSegmentDigest{ObjectKey, SHA256, SizeBytes}`，
  `CompletedRendition.Segments`（不含 init 段）；摘要取自分片行而非重算文件
  （输出目录在任务收尾会被清理）

### 交付门禁 `internal/gate`

把验收判据做成可 `go test` 跑的判定，每条一个函数 + 单测，一条命令跑全部：

```bash
go test ./internal/gate/...
```

已实现（13 条，44 个用例）：

| 判据 | 内容 | 判定方式 |
|---|---|---|
| A1 | 清单引用的每个分片可 GET 且 sha256 通过 | 注入 `ObjectReader`，逐片取回复算（含 init 段） |
| A2 | 实际使用硬件编码器（软编须有记录在案的例外理由） | 注入运行时观测（编码器名） |
| A3 | 软解路径 CPU 峰值 ≤ 50%（硬解路径标记为不适用） | 注入运行时观测（CPU 峰值与采样数） |
| A4 | 切片时长与约定一致（±1s 容差；尾片允许偏短） | 纯数据 |
| A5 | 分片序号连续无缺口（重复序号同样判失败） | 纯数据 |
| A6 | 每片带对象存储回执（状态 + Key + ETag + 大小） | 纯数据 |
| A7 | 回调载荷声明的每片摘要与库中一致 | 纯数据 |
| A8 | 清单可取回、容器可识别、覆盖每个清晰度 | 注入 `ManifestFetcher` |
| A9 | 同一任务只有一条完成回调事件 | 纯数据 |
| A10 | 步骤指纹与任务一致，且完成态是连续前缀 | 纯数据 |
| A11 | 失败收敛且不静默（失败必带错误信息，终态不得残留错误） | 纯数据 |
| A12 | 进度取值 ∈ [0,1000]，终态满进度 | 纯数据 |

判定原则：**只判定不修数据**；需要外部资源或运行时观测的判据一律注入，缺失时返回
`Skipped`（说明"该判却没判成"）；本次情形下本就不该判的返回 `NotApplicable`
（例如 A3 遇到硬解路径）—— 两者都不计入通过，且报告语义不同。

A2/A3 采用**运行时观测**口径：不看配置里写了什么、也不看调度时选了什么，只看执行期间实际观测到的
编码器名与 CPU 峰值（配置与决策都可能与真实执行不一致）。观测数据的采集与上报不在门禁内实现。

## 构建与测试

```bash
go build ./...        # 编译
go vet ./...          # 静态检查
go test ./...         # 全部测试（含 internal/gate 门禁）
go test ./internal/gate/... -v   # 只看门禁逐条结果
```

当前三者均通过。

## 文档

- `DESIGN.md`：总体设计
- `API_DOC.md`：HTTP / gRPC 接口
- `DEPLOYMENT_GUIDE.md`：部署指南（docker / k8s / systemd / nginx）
- `项目总原则.md`：设计原则与目录蓝图
- `HVC_API_POSTMAN.json`：Postman 集合

## 现状与边界（如实标注）

- **B2 能真正跳过的步骤目前只有 UPLOAD**（它的产出是分片行，可从库中判定）。
  PROBE / PLAN / SEGMENT 的产出还没有可回读的持久化形态（ffprobe 结果与计划需要写进步骤行的
  `detail`，分片清单需要一层到 `segmenter.Result` 的转换），因此续跑时这几步仍会真的重跑。
- **门禁 A1–A12 的判据原文未随仓库提供**（`sql/107` 引用的 `docs/TRANSCODE-SERVICE-DESIGN.md`
  不在仓库中）。现有 13 条是从设计与实现**推导**而来，每条标题都标注"（推导）"；
  拿到原文后按条替换即可（每条判定都是独立纯函数）。
- **A2/A3 的运行时观测数据由执行侧采集**（编码器名取 ffmpeg 实际使用的 `-c:v` 取值，
  CPU 取任务执行窗口内的采样峰值）：门禁只做判定，采集与上报不在本包内；
  没有观测数据时两条都会返回 `Skipped`（不算通过）。

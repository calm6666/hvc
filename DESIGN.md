# 视频点播/直播转码服务详细设计（中文完整版）

> 当前可直接联调的接口文档请优先看根目录：`API_OVERVIEW.md`、`API_HTTP.md`、`API_MODELS.md`、`API_RPC_MQ_CALLBACKS.md`、`API_GAP_NOTES.md`。
>
> 技术栈：**FFmpeg（命令行调用）+ Go（Gin）+ MySQL + Redis + gRPC + 消息队列 + S3 协议对象存储 + Vue3.5 + Element Plus+TypeScript**
>
> 设计目标：
> - 支持**点播转码**与**直播转码**两大模式。
> - 点播支持通过 **HTTP / gRPC / 消息队列** 触发转码。
> - 直播支持接入推流并转码后分发。
> - 全链路异步，不阻塞主线程。
> - 支持**单机模式**和**集群模式**。
> - 支持 **NVIDIA / Intel 核显 / AMD** 硬件加速。
> - 配置支持后台热更新，对新任务和排队中任务生效，对执行中任务不生效。
> - 追求极致性能、极致稳定性、极致可运维性。

---

## 1. 关键修正说明（基于你的最新要求）

### 1.1 分片输出方式修正

本系统的转码/打包输出调整为：

#### 1.1.1 点播（VOD）分片持久化

- **点播任务**：只关注并持久化 m4s 分段文件与 init 分段文件本身。
- 这些分段文件必须能够同时用于：
  - **DASH 协议**
  - **HLS 协议（fMP4 / CMAF 模式）**
- **不强依赖生成或持久化 mpd / m3u8 文件**。
- 点播系统重点是：
  - 把每个音频/视频分片、init 分片的**文件信息、协议能力、清晰度、顺序号、时长、对象存储位置**等全部入库持久化。
- 后续如果业务侧需要播放清单，应由**外部播放网关 / 分发服务 / 边缘服务**根据数据库中的分片元数据自行生成；本系统不提供生成或导出能力。

#### 1.1.2 直播（Live）分片不持久化（按你的要求新增）

- **直播转码分片不做 MySQL 持久化**（直播与点播不同，不需要长期保存分片明细）。
- Redis 不保存“完整直播 m3u8 文件”，只建议保存少量短期状态：
  - 当前频道状态
  - 最近窗口起始序号
  - 最近分片号
  - 当前码率 / 帧率 / 输入输出状态
- 直播相关的“短期可观测”仅做：
  - Redis 极小窗口状态缓存（短 TTL）用于后台实时展示
  - Prometheus 指标用于趋势与告警
- 直播分片文件本身由直播分发系统（Nginx/SRS/CDN/对象存储短期策略）负责，转码服务只关注：
  - 当前输出质量
  - 原画档与各直播清晰度档的生成状态
  - 当前推流/拉流状态
  - 失败重启/熔断
- 直播输出采用“**1 路输入 + 1 路原画档 + N 路主流清晰度档**”模型：
  - 原画档固定命名为 `source`
  - 主流档位如 `2160p/1440p/1080p/720p/540p/480p/360p`
  - 所有档位统一挂在直播 Profile 下，由频道绑定 Profile 后生效
  - 不允许上采样，最终实际输出档位由输入分辨率与 Profile 联合裁剪决定
- 用户侧不直接依赖“推流码”播放，主流实现是：
  - **主播推流**靠 `push_url + stream_key(推流码)` 接入
  - **业务后台/外部系统**通过 HTTP / gRPC 获取该频道当前可播放的清晰度列表
  - **播放器**再使用返回的 HLS / HTTP-FLV 等外部分发地址进行播放
  - 清晰度切换依赖业务接口返回的清晰度列表或外部分发层提供的变体流能力
- **直播的 HLS 必须提供 m3u8**（主流直播架构要求，播放器依赖 playlist 滚动更新）：
  - TS-HLS：m3u8 + .ts 分片
  - fMP4/CMAF-HLS：m3u8 + init.mp4（或 init.m4s）+ .m4s 分片
  - **m3u8 不会无限增长**：直播采用“滑动窗口（sliding window）”
    - playlist 只保留最近 `hls_list_size` 个分片
    - 通过 `#EXT-X-MEDIA-SEQUENCE` 标识当前窗口起始分片序号
    - 服务端持续覆盖写（rewrite）同一个 m3u8 文件，或在内存中生成后直接响应请求
    - 用户在开播很久后才开始播放时，只会拿到“最近窗口”的分片，符合你的期望
  - m3u8 可由直播分发层（推荐：SRS/Nginx）生成，本系统只维护转码状态与分片对象元数据，不负责清单生成
- FLV：主流做法是通过 **RTMP 或 HTTP-FLV** 直接分发，不涉及分片清单。
  - HTTP-FLV 是“长连接持续输出”的字节流（通常基于 HTTP chunked 传输），客户端边收边播
  - 服务端无需维护分片列表，只需保持推流/转推链路稳定
  - 适合低延迟（相对 HLS 更低），但移动端/生态兼容性通常不如 HLS


### 1.1.3 直播清晰度策略（新增）

直播转码必须支持：
- **原画**（保留输入原始分辨率与主编码策略，作为最高档）
- **主流直播清晰度档位**（按后台 Profile 配置）

推荐直播档位（可后台调整）：
- 原画（source）
- 2160p（4K）
- 1440p
- 1080p
- 720p
- 540p
- 480p
- 360p

约束：
- 若输入分辨率低于某档位，则该档位不生成
- 直播同样禁止上采样
- 原画档默认保留；后台可配置是否允许关闭原画输出
- 每个直播清晰度都可独立配置：
  - 分辨率
  - 码率
  - 编码格式（仅硬编支持）
  - GOP
  - B 帧
  - preset
- `source` 原画档支持三种策略：
  - 强制重编码
  - 优先旁路转封装/透传
  - 自动判定
- 自动判定时优先检查：输入编码、封装、GOP、音频编码、目标分发协议约束；满足则走旁路，否则切到硬件转码
- 外部系统获取清晰度列表的主入口应为 **HTTP/gRPC 元数据接口**，由业务层返回当前可播放清晰度、鉴权信息与播放地址。
- 业务端/APP 先调业务接口拿播放信息与鉴权 token，再选择 HLS / HTTP-FLV 等外部分发地址播放。


由于 HLS(fMP4) 与 DASH 都可基于 **CMAF 分片（init + m4s media segment）**，所以推荐采用：

- 一套编码结果
- 一套音视频分片
- 同时标记支持：
  - `support_dash = 1`
  - `support_hls = 1`

这样可以：
- 减少重复编码
- 减少重复存储
- 减少上传次数
- 提高整体吞吐

### 1.3 输入对象要求修正

输入对象要尽可能丰富、可选项尽可能多，但必须满足：
- 有合理默认值
- 允许后台白名单控制哪些字段可被外部覆盖
- 防止外部任意提交过高规格导致资源打爆

---

## 2. 总体目标与边界

### 2.1 对外能力边界

#### 2.1.1 对外触发转码接口

只允许三种触发方式：
- **HTTP：1 个创建任务接口**
- **gRPC：1 个创建任务方法**
- **消息队列：1 组创建任务消息消费通道**

#### 2.1.2 对外完成通知方式

当一个视频所有清晰度全部完成后，按后台动态配置对外通知其它应用，支持：
- HTTP 回调
- gRPC 回调
- MQ 发布（RabbitMQ 通过 amqp091-go 实现，支持 exchange + routing key）

补充规则：
- MQ 回调发布使用 broker confirm 作为成功判定
- 当消息因无匹配队列而被 broker `basic.return` 返回时，视为失败而不是成功
- 回调配置改为后台可维护的多条记录，而不是单值字段
- 每条记录都支持 `enabled` 与 `priority`
- 分发时按同类型配置的优先级顺序尝试
- 只要至少一个已启用回调目标成功，即视为该 outbox 事件投递成功
- 仅当所有已启用目标都失败，或完全没有可用配置时，才标记失败
- 旧的 runtime 单值字段仅保留为兼容回退路径，在对应类型没有动态配置时才使用

#### 2.1.3 其它接口归属

其它所有接口全部是：
- 后台管理接口
- 集群内部接口
- 节点上报接口
- 配置接口
- 实时指标接口

---

## 3. 整体架构设计

## 3.1 架构分层

### 3.1.1 接入层

- 公共 HTTP API（创建点播转码任务）
- 公共 gRPC API（创建点播转码任务）
- MQ Consumer（消费转码任务创建消息，RabbitMQ 后端统一使用 amqp091-go）
- 直播接入协同（接收 Nginx-RTMP / SRS Hook 或主动拉流）
- 直播接入网关（RTMP / WHIP / SRT 可扩展，当前主推 RTMP）
- 直播鉴权适配器（对接外部 HTTP / gRPC 认证系统）
- 直播播放发现接口（对外返回可播放清晰度列表与播放地址）

### 3.1.2 控制层

- 任务校验模块
- 任务入库模块
- 配置快照模块
- 调度器 Scheduler
- 失败恢复控制器
- 配置发布控制器
- 审计记录模块

### 3.1.3 执行层

- Worker 任务执行器
- 源视频下载器（支持 HTTP/HTTPS Range）
- 媒体探测器（ffprobe）
- FFmpeg 命令构建器
- GPU 资源管理器
- 分片上传器
- 进度上报器
- Live Session Manager（直播会话状态机）
- Live Ingest Validator（推流鉴权与推流码校验）
- Live Distribution Adapter（HLS / HTTP-FLV / RTMP 分发对接）

### 3.1.4 存储层

- MySQL：关系数据持久化、配置快照、审计、任务结果归档
- Redis：配置缓存、状态缓存、分布式锁、限流、实时指标缓存、热路径协调
- S3：音视频分片对象存储
- 节点间低延迟通信总线：用于心跳、执行上报、任务派发、租约续约、上传结果回传；不得依赖 MySQL 轮询

### 3.1.5 热路径边界与低延迟原则

- MySQL 只负责持久化与冷数据查询，不承担高频心跳、任务派发、执行进度、节点间通信。
- Redis 负责高频状态与短期协调，例如：
  - 调度候选队列
  - Worker 心跳与在线状态
  - 任务执行租约缓存
  - 任务实时进度
  - 分片上传队列与短期重试状态
- 集群节点之间的实时通信应优先采用低延迟通道：
  - gRPC 双向流 / streaming
  - Redis Streams / PubSub（按场景选型）
  - 必要时独立轻量消息总线
- 禁止通过 MySQL 轮询实现节点间高频协调，避免锁竞争、写放大与高延迟扩散。
- 任一节点内部的通信模块必须与转码执行线程隔离，通信阻塞、重连、短暂雪崩不得拖死正在执行的转码线程。

### 3.1.6 动态配置与部署兼容原则

- 除启动基础配置外，其它运行参数都应支持后台动态配置并热更新。启动基础配置只负责进程拉起，例如 MySQL、Redis、节点身份、集群注册入口；运行期业务配置统一通过后台发布版本生效，包括：
  - 集群通信方式与地址
  - 上传并发与重试策略
  - 限流阈值
  - 调度参数
  - 在线转码低延迟参数
  - RBAC 权限点与角色绑定策略
- Redis 需同时兼容：
  - 单机部署
  - Redis Cluster
- MySQL 需同时兼容：
  - 单机部署
  - 分库分表后的逻辑路由访问
- Bootstrap 配置源、RBAC、调度策略、回调策略都要设计成边界清晰：启动基础配置用于拉起进程，业务运行配置通过后台动态维护，不把关键行为硬编码在进程内。

### 3.1.7 并发执行与上传模型

- 单个 Worker 必须支持多任务并发执行，但并发度受以下资源共同限制：
  - 节点总转码会话数
  - 每张 GPU 最大会话数
  - 上传并发数
  - 源站下载带宽
- 单个转码任务内部应拆分为独立协程或线程池：
  - 源视频读取/拉流
  - FFmpeg 转码进程托管
  - 分片发现
  - 分片上传
  - 失败重试
  - 进度汇报
- 分片上传必须支持多线程并发，且上传失败重试不得阻塞转码主线程。
- 多线程并发数量必须严格受后台配置限制，并允许动态算法根据实时资源状态自动收缩。
- 重试控制必须区分：
  - 可立即重试的瞬时错误
  - 需要退避的网络错误
  - 不可恢复的权限/参数错误
- 上传失败、回调失败、节点通信失败都必须在独立工作队列中处理，避免拖垮其它转码任务。

### 3.1.8 在线转码低延迟与网络波动控制

- 在线视频转码场景下，必须同时处理低延迟与网络波动：
  - 源站下载支持 Range、断点续拉、超时重连
  - 直播/在线输入支持短时抖动缓冲与恢复窗口
  - 关键状态采用内存 + Redis 双层缓存，避免抖动期间频繁打 MySQL
  - 分片上传支持指数退避、并发限速、失败隔离
  - 单个清晰度失败时优先局部降级，不影响其他清晰度继续运行
  - 当 CPU、内存、显存、上传队列接近阈值时，必须主动降并发，而不是继续榨干机器
- 对于长时间在线播放或大文件点播，需提供：
  - 片段级重试
  - 任务级租约续约
  - 节点级故障切换
- 设计目标是：网络抖动、瞬时上传失败、单卡故障、单节点通信异常都不应拖垮整个集群或其它正在运行的任务。

### 3.1.9 注释规范

- 后续代码实现中，所有核心结构体字段、持久化字段、公开方法、调度与执行关键流程都必须补充详细中文注释。
- 注释语义必须贴合本项目上下文，重点说明：
  - 为什么存在该字段或方法
  - 在集群、GPU、分片上传、重试、故障切换中的职责
  - 与 Redis、MySQL、对象存储之间的边界
- 避免写空泛注释，要求注释能直接帮助后续维护者理解运行时语义与故障处理意图。

### 3.1.10 展示层

- 后台管理端（Vue3.5 + Element Plus）
- HTTP 监控快照接口（首屏加载 / 断线重拉）
- WebSocket 实时推送（当前实现目标：`/api/admin/transcode/monitor/ws`）
- 可选 Nuxt4 SSR 前端壳层

---

## 4. 编解码能力与硬件加速设计（新增）

### 4.1 原始视频多格式、多编码支持

系统必须支持常见输入容器与编码格式，并在任务探测阶段识别：

#### 4.1.1 输入容器格式
- MP4
- MOV
- MKV
- FLV
- TS
- WebM
- AVI（兼容性支持）
- MPEG-PS / MPEG-TS（兼容性支持）

#### 4.1.2 输入视频编码格式
- H.264 / AVC
- H.265 / HEVC
- VP8
- VP9
- AV1
- MPEG-2 Video
- MPEG-4 Part 2（兼容性输入）

#### 4.1.3 输入音频编码格式
- AAC
- MP3
- Opus
- AC3
- EAC3
- PCM（兼容性输入）

### 4.2 解码能力检测与自动回退

#### 4.2.1 节点启动时探测内容

每个 Worker 节点启动时必须探测并缓存：
- 支持的硬件解码器列表
- 支持的硬件编码器列表
- 每种硬编编码器的最大并发会话数
- 各 GPU 型号与驱动版本
- 硬件滤镜能力（包括缩放、水印、overlay、colorspace 等）

#### 4.2.2 解码策略

任务开始时：
1. 通过 `ffprobe` 获取源视频编码格式。
2. 先检查当前节点是否支持该编码的**硬件解码**。
3. 若支持，则必须使用硬件解码。
4. 若**不支持硬件解码，才允许回退到软件解码**。
5. 一旦进入软件解码，必须同步启用 CPU 占用保护，不得因单任务软解拖垮整机。

#### 4.2.3 回退原则

- **编码必须全部使用硬件编码**，不允许回退软件编码。
- **解码仅在源视频确实不支持硬解时，才允许回退软件解码**。
- **水印、缩放、overlay 等图像处理优先走硬件滤镜链路**，避免把主要负载压回 CPU。
- 若任务需要水印但当前节点无法提供“硬解 + 硬件滤镜 + 硬编”的完整链路，则应：
  - 优先换节点
  - 若集群无可执行节点，则直接拒绝创建或标记调度失败
- 若进入软件解码：
  - 单任务 CPU 占用必须受限
  - 节点总 CPU 使用率目标不得超过后台配置上限
  - 默认要求软解相关总 CPU 占用不得超过 **50%**，防止系统卡死

### 4.2.4 软解保护与过载控制

- 软解只作为“源视频不支持硬解”时的例外路径，不是常规能力。
- 调度器与 Worker 必须同时检查以下约束：
  - 当前节点 CPU 使用率
  - 当前软解任务数
  - 单任务 CPU 限额
  - 全局 CPU 保护阈值
- 当任一指标接近阈值时，必须自动执行以下策略之一：
  - 暂停接收新软解任务
  - 降低同节点并发
  - 降低分片上传并发
  - 延迟低优先级任务派发
- 设计目标：即使多个软解任务同时出现，也不能让服务器资源打满导致系统卡死。

### 4.3 输出编码设计（只能是硬件编码支持的格式）

#### 4.3.1 输出编码来源

输出编码格式支持两种来源：
- 后台管理端 Profile 为每个清晰度配置
- 外部触发对象为每个清晰度单独指定（前提是后台允许覆盖）

#### 4.3.2 输出编码限制

当前设计中，为了追求极致速度：
- 输出视频编码格式只能选择当前系统支持的**硬件编码格式**
- 例如：
  - NVIDIA：`h264` / `hevc` / `av1(视硬件而定)`
  - Intel QSV：`h264` / `hevc` / `av1(视硬件而定)`
  - AMD：`h264` / `hevc` / `av1(视驱动和硬件而定)`

### 4.3.4 输出编码白名单与节点能力校验规则（可直接开发）

#### 4.3.4.1 统一 codec 枚举定义

数据库和接口层统一使用如下视频编码枚举：
- `1 = h264`
- `2 = hevc`
- `3 = av1`

音频编码当前统一建议：
- `1 = aac`
- `2 = opus（预留）`

#### 4.3.4.2 Profile 层白名单校验

后台保存 `t_profile_rendition` 时，必须校验：
- `video_codec` 只能是 `1/2/3`
- `preset` 必须属于后台允许的 preset 白名单
- 不能保存软件编码专用 codec 配置

#### 4.3.4.3 请求对象层白名单校验

当外部请求通过 `renditions[].video_codec` 覆盖时，必须同时满足：
- `t_config_transcode_runtime.allow_request_override_rendition_codec = 1`
- 请求中的 `video_codec` 在枚举范围内
- 请求清晰度不能上采样
- 请求 codec 必须能被当前集群至少一个节点的硬件编码能力支持

否则直接拒绝创建任务或进入失败队列（取决于设计点，推荐创建前拒绝）。

#### 4.3.4.4 节点能力白名单映射

节点硬件编码支持关系通过 `t_worker_codec_capability` 表维护：
- `cap_type = 2` 表示硬件编码能力
- `codec_name in ('h264','hevc','av1')`

调度器按以下规则判断节点是否可执行：
- 当前任务所有清晰度要求的 `video_codec` 集合
- 该节点 `cap_type=2` 的 `codec_name` 集合
- 若任务 codec 集合是节点 codec 集合的子集，则节点可执行
- 否则节点不可执行

#### 4.3.4.5 推荐硬件与编码支持表（设计约束）

- NVIDIA：
  - 常见支持：h264、hevc
  - 部分新卡支持：av1
- Intel QSV：
  - 常见支持：h264、hevc
  - 新平台可支持：av1
- AMD AMF / VAAPI：
  - 常见支持：h264、hevc
  - 部分平台支持：av1

所以系统实现必须遵循：
- **不写死“某硬件一定支持某 codec”**
- 一律以启动探测 + `t_worker_codec_capability` 实际上报结果为准

### 4.4.2 水印与滤镜链路约束
- 水印必须优先走硬件滤镜链路，例如 CUDA/NPP、QSV VPP、VAAPI/AMF 对应硬件处理路径。
- 不允许把水印处理默认落回 CPU 再回送 GPU，这会显著抬高 CPU 占用并拉低吞吐。
- 若节点无法提供满足当前任务要求的硬件水印能力，应在调度阶段判定为不可执行，而不是执行中隐式退回 CPU 滤镜。

### 4.4.3 节点资源保护与动态调控
- 后台必须可配置以下资源上限：
  - 节点最大并发转码任务数
  - 单 GPU 最大并发会话数
  - 分片上传并发数
  - 单任务最大上传并发数
  - 软解 CPU 使用率上限
  - 节点总 CPU 使用率保护阈值
  - 节点总内存使用率保护阈值
  - 节点总显存使用率保护阈值
- 上述上限既支持后台静态配置，也支持运行时动态算法调控。
- 动态算法至少可基于以下实时指标收敛并发：
  - CPU 使用率
  - 内存占用
  - GPU 使用率
  - GPU 显存占用
  - 上传积压深度
  - 源站下载抖动与失败率
- 任何情况下都必须以“服务器资源不过满、不把系统拖死”为最高约束，高优先级大于吞吐。
  - 硬编 preset

#### 4.4.2 请求对象配置能力
- 外部触发对象可在 `renditions[]` 中指定每个清晰度的：
  - `video_codec`
  - `video_bitrate_kbps`
  - `video_maxrate_kbps`
  - `video_bufsize_kbps`
  - `preset`

但前提是：
- 后台允许该字段覆盖
- 指定 codec 必须在目标节点上存在硬件编码支持

---

## 5. 点播转码业务流程

### 4.1 创建任务流程

1. 外部通过 HTTP / gRPC / MQ 提交转码请求对象。
2. 三种触发方式统一映射到同一个内部对象 `CreateTranscodeJobRequest`。
3. API / gRPC Server / MQ Consumer 对以下内容做一致性校验：
   - 水印参数
   - 缩略图雪碧图参数
   - 缩略图 `.bin` 参数
   - 对象存储前缀参数
   - 非 16:9 保持比例参数
4. 根据 `request_id` 做幂等判断。
5. 读取当前有效配置版本。
6. 合并后台默认配置与外部覆盖项，生成任务最终快照。
7. 持久化任务主表、任务覆盖参数表、雪碧图任务配置、缩略图 bin 配置、任务快照表。
8. 投递到待调度队列。
9. 立即返回受理成功。

### 4.2 调度执行流程

1. Scheduler 从待调度队列拉取任务。
2. 按节点过滤规则和加权打分选出最优节点。
3. 写入租约并下发到目标 Worker。
4. Worker 执行：
   - 下载源视频
   - 探测媒体信息
   - 生成清晰度集合（只能向下）
   - 计算每个输出清晰度的保持比例缩放方案，保证 4:3、电影宽银幕等输入在输出到 16:9 档位时不变形
   - 执行 FFmpeg 命令行转码、scale、pad、watermark overlay
   - 同时抽帧生成缩略图
   - 每 100 张缩略图拼成 10x10 雪碧图，视频太长则生成多张雪碧图
   - 按顺序将缩略图 base64 编码写入 `.bin` 文件，单文件超过上限则自动切分多个 `.bin`
   - 上传 init 分片、m4s 分片、雪碧图、`.bin` 文件到对象存储
   - 分片、雪碧图、`.bin` 文件、缩略图索引全部入库持久化
   - 更新进度
5. 所有清晰度及附属产物完成后：
   - 标记任务完成
   - 生成完成事件到 Outbox
   - 异步通知外部系统

### 4.3 失败处理流程

- MQ 消费失败：重试、死信、审计
- 转码失败：进入失败队列
- 上传失败：分片级重试、任务级失败升级
- 回调失败：Outbox 重试，不影响主任务完成态

---

## 6. 输入对象设计（尽可能丰富、可自定义）

## 5.1 公共创建任务对象 `CreateTranscodeJobRequest`

```json
{
  "request_id": "20260328-abcdef-001",
  "biz_key": "video_10001",
  "source_url": "https://cdn.example.com/a.mp4",
  "source_headers": {
    "User-Agent": "custom-agent",
    "Referer": "https://example.com"
  },
  "source_connect_timeout_ms": 5000,
  "source_read_timeout_ms": 30000,
  "source_range_chunk_bytes": 4194304,
  "source_retry_count": 3,
  "profile_id": 1,
  "priority": 100,
  "callback_biz_type": "video_publish",
  "enable_watermark": true,
  "watermark": {
    "image_url": "https://cdn.example.com/wm.png",
    "anchor": 1,
    "x_ratio": 0.05,
    "y_ratio": 0.05,
    "width_ratio": 0.12,
    "opacity": 1.0,
    "safe_margin_ratio": 0.02
  },
  "video_options": {
    "force_downscale_only": true,
    "output_aspect_keep": true,
    "aspect_fill_mode": "pad_black",
    "deinterlace": false,
    "denoise": false,
    "sharpen": false,
    "rotate_degree": 0,
    "target_fps": 0,
    "target_pix_fmt": "yuv420p",
    "gop_size": 48,
    "bframes": 2,
    "sc_threshold": 40,
    "video_codec": "h264",
    "audio_codec": "aac",
    "audio_channels": 2,
    "audio_sample_rate": 48000,
    "audio_bitrate_kbps": 128
  },
  "segment_options": {
    "enable_cmaf": true,
    "segment_duration_sec": 4,
    "independent_segments": true,
    "save_init_segment": true,
    "support_dash": true,
    "support_hls": true,
    "naming_template_id": 1
  },
  "thumbnail_options": {
    "enable_sprite": true,
    "sprite_rows": 10,
    "sprite_cols": 10,
    "thumb_interval_sec": 10,
    "thumb_width": 214,
    "thumb_height": 120,
    "sprite_image_format": "jpeg",
    "sprite_storage_prefix": "vod/thumbs/2026/03/28",
    "enable_binary_index": true,
    "binary_storage_prefix": "vod/thumbbin/2026/03/28",
    "binary_max_size_bytes": 10485760
  },
  "storage_options": {
    "storage_id": 1,
    "bucket_prefix": "vod/custom/2026/03/28",
    "segment_prefix": "vod/custom/2026/03/28/segments",
    "object_acl": "private",
    "multipart_threshold_bytes": 8388608,
    "multipart_part_size_bytes": 8388608
  },
  "schedule_options": {
    "preferred_node_tags": ["gpu", "bj"],
    "preferred_hwaccel": "nvidia",
    "allow_software_decode_fallback": true,
    "max_wait_seconds": 3600,
    "require_hw_encode": true
  },
  "callback_options": {
    "notify_on_success": true,
    "notify_on_failure": true,
    "notify_payload_mode": "full"
  },
  "renditions": [
    {
      "name": "2160p",
      "width": 3840,
      "height": 2160,
      "video_codec": "hevc",
      "video_bitrate_kbps": 16000,
      "video_maxrate_kbps": 19000,
      "video_bufsize_kbps": 24000,
      "preset": "p4"
    },
    {
      "name": "1080p",
      "width": 1920,
      "height": 1080,
      "video_codec": "h264",
      "video_bitrate_kbps": 5000,
      "video_maxrate_kbps": 5500,
      "video_bufsize_kbps": 7500,
      "preset": "p4"
    },
    {
      "name": "720p",
      "width": 1280,
      "height": 720,
      "video_codec": "hevc",
      "video_bitrate_kbps": 2800,
      "video_maxrate_kbps": 3200,
      "video_bufsize_kbps": 4200,
      "preset": "p4"
    }
  ],
  "ext": {
    "operator": "system"
  }
}
```

## 5.2 字段设计原则

### 5.2.1 必填字段
- `request_id`
- `source_url`

### 5.2.2 强烈建议字段
- `profile_id`
- `priority`
- `segment_options`
- `storage_options`

### 5.2.3 白名单覆盖机制

后台需要配置允许外部覆盖的字段，例如：
- 允许覆盖：水印位置、分片时长、输出前缀、preferred_hwaccel、缩略图间隔、雪碧图前缀、`.bin` 前缀、`.bin` 单文件大小上限
- 不允许覆盖：最大并发、系统级上传线程数、租约 TTL

### 5.2.4 三种触发方式字段一致性要求（新增）

HTTP / gRPC / MQ 三种触发方式必须支持同一套核心字段：
- watermark
- video_options.output_aspect_keep
- video_options.aspect_fill_mode
- thumbnail_options
- storage_options.bucket_prefix / segment_prefix

禁止出现：
- HTTP 支持但 gRPC 不支持
- gRPC 支持但 MQ 不支持
- 三种入口字段语义不一致

系统内部必须先把 HTTP JSON / gRPC Proto / MQ Message 统一转换为同一内部 DTO，再进入校验和落库流程。

---

## 7. 分片设计（核心修正版）

## 6.1 分片类型

每个清晰度下，至少包含：
- 视频 init 分片
- 视频 media 分片（m4s）
- 音频 init 分片
- 音频 media 分片（m4s）

## 6.2 为什么不强依赖 mpd / m3u8

因为你的重点是：
- **数据库里要持久化每个分片**
- 播放组织文件不属于本系统职责范围

所以系统设计改为：
- 转码系统负责生产**可被 HLS/DASH 共同消费的标准 CMAF 分片集合**
- 不把 manifest 作为核心产物
- 以“分片元数据中心”的思路建模

## 6.3 分片表必须具备的关键字段

每个分片必须记录：
- 属于哪个任务
- 属于哪个清晰度
- 属于音频还是视频
- 是否 init 分片
- 序号
- 时长
- 字节大小
- S3 对象 key
- ETag
- SHA256（可选）
- 是否支持 HLS
- 是否支持 DASH
- 编码格式
- 时间戳范围（可选）

---

## 8. FFmpeg 转码/打包方案设计（按你的要求修正）

## 7.1 目标

- 输出 **CMAF 分片（init + m4s）**
- 一份分片可同时支持 DASH / HLS(fMP4)
- 音视频分离
- 分片全部持久化到数据库
- manifest 文件不是核心要求，本系统不生成也不持久化
- **FFmpeg 采用命令行调用方式（通过 Go 的 `os/exec` 包启动 ffmpeg/ffprobe 子进程）**
- **通过解析 FFmpeg stderr 输出实时获取转码进度**
- **Worker 进程通过 goroutine 管理 FFmpeg 子进程的生命周期，支持优雅终止与强制杀死**

## 7.2 命令行调用方案设计

### 7.2.1 为什么采用 FFmpeg 命令行调用方式

采用 FFmpeg 命令行调用方式的优势：
- **部署简单**：无需编译链接 FFmpeg 动态库/静态库，只需系统安装 ffmpeg 可执行文件
- **隔离性好**：FFmpeg 子进程崩溃不影响 Worker 主进程，Worker 可检测退出码并做失败重试
- **调试方便**：可直接复制命令行参数在终端执行，快速定位参数问题
- **升级灵活**：升级 FFmpeg 版本只需替换可执行文件，无需重新编译 Worker
- **Go 生态契合**：Go 的 `os/exec` 包提供了完善的子进程管理能力，结合 `context.Context` 可实现超时控制与优雅取消
- **进度可观测**：FFmpeg 在 stderr 中输出实时进度信息（`-progress` / `-stats`），可被 Go 程序实时解析

### 7.2.2 命令行调用总体架构

Worker 内部 goroutine 模型：
- 调度 goroutine：接收任务、状态流转
- 下载 goroutine 池：负责源数据拉取
- 探测 goroutine：调用 `ffprobe` 命令获取媒体信息
- 转码 goroutine：调用 `ffmpeg` 命令执行转码，实时解析 stderr 获取进度
- 上传 goroutine 池：负责 S3 上传
- 进度上报 goroutine：定期将解析到的进度写入 Redis 并上报 Scheduler

这样可以实现：
- FFmpeg 子进程与 Worker 主进程隔离，互不影响
- 通过管道实时读取 FFmpeg 进度输出
- 通过 `context.Context` 控制子进程生命周期
- 通过 `syscall.Kill` 实现强制终止

### 7.2.3 FFmpeg 命令行调用核心实现

#### 7.2.3.1 ffprobe 探测实现

```go
type ProbeResult struct {
    VideoCodec       string  `json:"video_codec"`
    AudioCodec       string  `json:"audio_codec"`
    ContainerFmt     string  `json:"container_format"`
    Width            int     `json:"width"`
    Height           int     `json:"height"`
    FPS              float64 `json:"fps"`
    AvgBitrateKbps   int     `json:"avg_bitrate_kbps"`
    VideoBitrateKbps int     `json:"video_bitrate_kbps"`
    AudioBitrateKbps int     `json:"audio_bitrate_kbps"`
    GOPSize          int     `json:"gop_size"`
    KeyintSec        float64 `json:"keyint_sec"`
    HasBFrame        bool    `json:"has_bframe"`
    PixFmt           string  `json:"pix_fmt"`
    ColorRange       string  `json:"color_range"`
    AudioChannels    int     `json:"audio_channels"`
    AudioSampleRate  int     `json:"audio_sample_rate"`
    DurationMs       int64   `json:"duration_ms"`
    PTSMonotonic     bool    `json:"pts_monotonic"`
    DTSMonotonic     bool    `json:"dts_monotonic"`
}

func ProbeMedia(ctx context.Context, inputURL string, ffprobePath string) (*ProbeResult, error) {
    args := []string{
        "-v", "quiet",
        "-print_format", "json",
        "-show_format",
        "-show_streams",
        "-show_entries",
        "stream=codec_name,codec_type,width,height,r_frame_rate,pix_fmt," +
            "channels,sample_rate,bits_per_raw_sample,has_b_frames:" +
            "format=duration,bit_rate,size,format_name",
        inputURL,
    }
    cmd := exec.CommandContext(ctx, ffprobePath, args...)
    output, err := cmd.Output()
    if err != nil {
        return nil, fmt.Errorf("ffprobe failed: %w", err)
    }
    return parseProbeOutput(output)
}
```

#### 7.2.3.2 ffmpeg 转码命令执行与进度解析

```go
type TranscodeProgress struct {
    JobID         int64   `json:"job_id"`
    RenditionName string  `json:"rendition_name"`
    Frame         int64   `json:"frame"`
    FPS           float64 `json:"fps"`
    BitrateKbps   float64 `json:"bitrate_kbps"`
    TotalSize     int64   `json:"total_size"`
    TimeMs        int64   `json:"time_ms"`
    Speed         float64 `json:"speed"`
    Percent       float64 `json:"percent"`
}

type FFmpegRunner struct {
    ffmpegPath      string
    progressCh      chan<- TranscodeProgress
    cmd             *exec.Cmd
    cancelFn        context.CancelFunc
    totalDurationMs int64
}

func (r *FFmpegRunner) Run(ctx context.Context, args []string) error {
    childCtx, cancel := context.WithCancel(ctx)
    r.cancelFn = cancel

    cmd := exec.CommandContext(childCtx, r.ffmpegPath, args...)
    r.cmd = cmd

    stderr, err := cmd.StderrPipe()
    if err != nil {
        return fmt.Errorf("failed to create stderr pipe: %w", err)
    }

    if err := cmd.Start(); err != nil {
        return fmt.Errorf("failed to start ffmpeg: %w", err)
    }

    go r.parseProgress(stderr)

    if err := cmd.Wait(); err != nil {
        return fmt.Errorf("ffmpeg exited with error: %w", err)
    }
    return nil
}

func (r *FFmpegRunner) parseProgress(reader io.Reader) {
    scanner := bufio.NewScanner(reader)
    re := regexp.MustCompile(
        `frame=\s*(\d+)\s+fps=\s*([\d.]+)\s+` +
        `.*size=\s*(\d+)\w*\s+time=(\d+:\d+:\d+\.\d+)\s+` +
        `bitrate=\s*([\d.]+)\w*/s\s+speed=\s*([\d.]+)x`)

    for scanner.Scan() {
        line := scanner.Text()
        matches := re.FindStringSubmatch(line)
        if matches == nil {
            continue
        }
        frame, _ := strconv.ParseInt(matches[1], 10, 64)
        fps, _ := strconv.ParseFloat(matches[2], 64)
        totalSize, _ := strconv.ParseInt(matches[3], 10, 64)
        timeMs := parseTimeToMs(matches[4])
        bitrate, _ := strconv.ParseFloat(matches[5], 64)
        speed, _ := strconv.ParseFloat(matches[6], 64)

        percent := 0.0
        if r.totalDurationMs > 0 {
            percent = float64(timeMs) / float64(r.totalDurationMs) * 100.0
            if percent > 100.0 {
                percent = 100.0
            }
        }

        progress := TranscodeProgress{
            Frame:       frame,
            FPS:         fps,
            TotalSize:   totalSize,
            TimeMs:      timeMs,
            BitrateKbps: bitrate,
            Speed:       speed,
            Percent:     percent,
        }
        select {
        case r.progressCh <- progress:
        default:
        }
    }
}

func (r *FFmpegRunner) Stop(force bool) {
    if r.cancelFn != nil {
        r.cancelFn()
    }
    if force && r.cmd != nil && r.cmd.Process != nil {
        r.cmd.Process.Signal(syscall.SIGKILL)
    }
}
```

### 7.2.4 阶段一：命令行转码产出标准 CMAF 分片

通过 FFmpeg 命令行输出：
- 视频 init 分片
- 视频 media m4s
- 音频 init 分片
- 音频 media m4s

### 7.2.5 阶段二：分片采集与异步上传

- FFmpeg 命令行将分片输出到本地临时目录
- 分片监听器（基于 `fsnotify` 或定时目录扫描）检测新分片文件
- 某个分片一旦 finalize：
  - 读取分片文件元数据（大小、时长等）
  - 上传 S3
  - 落库到 `t_transcode_segment`

### 7.2.6 点播与直播的分片持久化差异

#### 7.2.6.1 点播（VOD）
- 点播 init 分片与 m4s 分片全部上传 S3 并落 MySQL `t_transcode_segment`
- 播放侧若需要清单文件，应由外部分发系统基于这些元数据自行生成

#### 7.2.6.2 直播（Live）
- 直播分片**不落 MySQL，不做长期持久化**
- 直播分片状态只做：
  - Redis 短 TTL 缓存
  - 指标系统实时采集
- Redis 仅保存最近窗口信息，例如：
  - 最近 N 个分片序号
  - 最近分片时长
  - 当前推流码率
  - 当前频道状态
- 直播转码服务主要落库：
  - 频道配置
  - 频道运行状态
  - 故障/重启记录
  - 审计信息
- 直播输出编排规则：
  - 每个频道绑定 1 个直播 Profile
  - 每个 Profile 下包含 1 个 `source` 原画档和 0~N 个转码档
  - `source` 档优先表示“保留原始分辨率档”而不是“数据库里另起一套特殊流程”
  - 当输入编码、封装、GOP、码率策略满足分发要求时，可选择 `source` 走旁路转封装/透传（不缩放、不重编码）
  - 当输入不满足分发规范，或后台要求统一编码格式时，`source` 仍可走硬件重编码
  - 非 `source` 档统一走降采样转码，不允许放大
  - 最终向 HLS/HTTP-FLV/RTMP 分发层暴露 `source + 多档清晰度` 播放地址

#### 7.2.7 分片元数据边界

- 点播任务只生成并持久化 init/media 分片对象及其元数据。
- 本系统不生成、不缓存、不导出、不持久化 MPD 或 M3U8。
- 若播放侧需要清单文件，应由外部播放网关、CDN 或分发系统基于自身规则生成，不属于本系统职责范围。
- 默认建议：直播与点播都只在本系统内维护转码状态、对象路径、时长、PTS 与上传结果，不维护 playlist 文件生命周期。

### 7.2.9 主播推流鉴权、播放鉴权与错误恢复设计

#### 7.2.9.1 主播推流鉴权时机

推荐接入层（Nginx-RTMP / SRS / 自研 Ingest）在以下时机触发校验：
- `on_connect`：做基础连接级限流、黑白名单、签名时效校验
- `on_publish`：做最终主播身份、频道状态、推流码、业务权限校验
- `on_unpublish`：通知控制面结束或挂起当前推流会话

#### 7.2.9.2 主播推流鉴权请求内容

请求外部鉴权系统时建议传：
- channel_key
- stream_key
- client_ip
- app_name
- tc_url
- request_ts
- nonce
- device_id（有则传）
- publisher_uid（有则传）

#### 7.2.9.3 主播推流鉴权决策

外部鉴权系统返回：
- `allow=true/false`
- `decision_code`
- `decision_message`
- `publisher_uid`
- `profile_id_override`（可选）
- `expire_at`（可选）

处理规则：
- allow=true：放行推流并创建/更新 `t_live_publish_session`
- allow=false：立即断开推流连接，状态记为 `REJECTED`
- HTTP/gRPC 超时：按频道配置决定失败关闭或回退到备用鉴权方式
- 本地校验通过但外部校验失败：以外部校验结果为准

#### 7.2.9.4 观众播放鉴权

主流做法：
- 观众不直接拿裸播放 URL
- 先调业务“获取播放信息”接口
- 业务侧校验登录态、会员权限、地域、黑名单、付费状态
- 返回带时效签名的 HLS / HTTP-FLV URL
- CDN / 边缘层可再次校验 token、expire、sign

#### 7.2.9.5 推流中断与自动恢复状态机

Live Session Manager 状态机建议：
- `CONNECTING`
- `PUBLISHING`
- `INTERRUPTED_WAIT_RESUME`
- `RESUMED`
- `STOPPED`
- `REJECTED`

恢复规则：
1. `last_media_at` 超过短阈值（如 3~5 秒）未更新，标记为 `INTERRUPTED_WAIT_RESUME`
2. 在 `resume_timeout_sec` 内重新收到同一频道/同一主播/同一推流码的新 publish，可直接恢复到 `RESUMED`
3. 恢复成功后：
   - 尽量复用原频道配置与播放地址
   - HLS 滑动窗口继续前进
   - HTTP-FLV 新连接自动切到新 session 数据流
4. 若超过 `resume_timeout_sec` 仍未恢复：
   - 关闭当前 session
   - 释放转码与分发资源
   - 频道状态转为 `STOPPED` 或 `ERROR`
5. 若恢复期间连续失败达到阈值：
   - 进入短时熔断
   - 一段 sleep 后允许再次接入

#### 7.2.9.6 推流网络抖动错误处理

必须内建以下容错：
- 音视频短时无数据：不要立刻销毁会话，先进入等待恢复窗口
- 单路清晰度转码失败：优先降级该清晰度，不影响其它清晰度继续输出
- source 透传失败：自动回切到硬件重编码（若允许）
- 上传/分发层瞬时失败：做指数退避重试，不立即终止直播
- 外部鉴权系统短时不可用：按配置走备用 HTTP/gRPC 或短时缓存决策
- 节点故障：调度器将频道漂移到健康节点重建 session

#### 7.2.9.7 推流码管理建议

推荐推流码生成规则：
- 由业务系统签发 `stream_key`
- 包含：channel_key、publisher_uid、expire_ts、nonce、sign
- 服务端只存摘要，不存明文长期值
- 支持立即失效、定时过期、轮换重发

### 7.2.10 `source` 自动判定规则表（可直接编码）

| 规则ID | 条件项 | 判定条件 | 动作 | 说明 |
|---|---|---|---|---|
| S001 | 后台策略 | `source_process_mode=1` | 强制重编码 | 后台明确指定 |
| S002 | 后台策略 | `source_process_mode=2` | 优先透传 | 后台明确指定，若后续协议不兼容再降级 |
| S003 | 后台策略 | `source_process_mode=3` | 进入自动判定 | 默认推荐 |
| S010 | 输入视频编码 | 输入 codec 不在分发白名单（如 H264/H265） | 重编码 | 避免播放器/CDN 兼容问题 |
| S011 | 输入音频编码 | 输入音频 codec 不在白名单（如 AAC） | 重编码/转音频 | HLS/FLV 常见要求 AAC |
| S012 | 输入封装 | 输入容器不能直接转目标封装 | 转封装或重编码 | 先尝试转封装，不满足再重编码 |
| S013 | 目标协议 | 目标需要 HLS + HTTP-FLV 双输出且当前输入关键参数不兼容 | 重编码 | 统一 GOP/音频/时间基 |
| S014 | GOP | 输入关键帧间隔大于 Profile 要求上限 | 重编码 | 保证切片与低延迟稳定 |
| S015 | 时间戳 | 检测到 DTS/PTS 回退、跳变严重 | 重编码 | 提升播放稳定性 |
| S016 | 分辨率 | source 仅保留原分辨率，不缩放 | 继续判定 | source 不因分辨率触发重编码 |
| S017 | 水印 | 频道启用水印 | 重编码 | 透传无法叠加水印 |
| S018 | DRM/加密 | 需要输出侧加密或插入额外 metadata | 重编码 | 便于统一处理 |
| S019 | 节点能力 | 当前节点无目标 codec 的硬件编码能力且必须转码 | 调度失败/换节点 | 不回退软编 |
| S020 | 码率策略 | 输入码率超出 source 上限且后台要求收敛 | 重编码 | 控制带宽成本 |
| S021 | 外部覆盖 | 外部请求强制 source 重编码 | 重编码 | 高优先级业务覆盖 |
| S022 | 外部覆盖 | 外部请求允许 source 透传且全部兼容 | 透传 | 仅在安全白名单内生效 |
| S099 | 默认收敛 | 上述规则均未命中且协议兼容 | 透传 | 最终默认走旁路 |

编码实现建议：
- 按规则 ID 顺序短路执行
- 每次命中都记录 `decision_rule_id`
- 将最终结果写入 Redis 与运行日志：
  - `source_runtime_mode=passthrough/transcode`
  - `source_decision_rule_id=S0xx`
  - 当网络抖动或输入流不稳定时，需同步记录抖动窗口、恢复次数、最近一次降级原因
- 在后台详情页展示“原画档当前模式 + 触发规则 + 原因说明”

### 7.2.11 `source` 自动判定所需探测字段清单（开发必备）

为保证 S0xx 规则可直接落地，进入判定函数前必须准备以下结构化字段：

#### 7.2.11.1 输入流探测字段 `InputProbe`
- `video_codec`：h264/hevc/av1/vp9...
- `audio_codec`：aac/mp3/opus/ac3...
- `container_format`：flv/ts/mp4/mkv...
- `width`
- `height`
- `fps`
- `avg_bitrate_kbps`
- `video_bitrate_kbps`
- `audio_bitrate_kbps`
- `gop_size`
- `keyint_sec`
- `has_bframe`
- `pix_fmt`
- `color_range`
- `audio_channels`
- `audio_sample_rate`
- `timebase_num`
- `timebase_den`
- `pts_monotonic`：PTS 是否单调
- `dts_monotonic`：DTS 是否单调
- `pts_jump_count`
- `dts_jump_count`
- `has_corrupt_packet`

#### 7.2.11.2 频道配置字段 `ChannelConfig`
- `enable_source_rendition`
- `source_passthrough_mode`
- `enable_watermark`
- `viewer_auth_mode`
- `play_protocols`：如 `[hls, http_flv]`
- `resume_timeout_sec`
- `force_normalize_gop`
- `require_aac_audio`
- `require_cmaf_compatible`
- `require_flv_compatible`
- `enable_output_encryption`
- `max_source_bitrate_kbps`
- `source_bitrate_cap_enabled`

#### 7.2.11.3 原画档配置字段 `SourceRenditionConfig`
- `video_codec`
- `audio_codec`
- `gop_size`
- `bframes`
- `source_process_mode`
- `allow_passthrough`
- `allow_transmux_only`
- `force_target_video_codec`
- `force_target_audio_codec`

#### 7.2.11.4 节点能力字段 `NodeCapabilities`
- `supported_hw_decode_codecs[]`
- `supported_hw_encode_codecs[]`
- `supported_passthrough_video_codecs[]`
- `supported_passthrough_audio_codecs[]`
- `supported_ingest_protocols[]`
- `supported_output_protocols[]`
- `available_hw_sessions`
- `has_required_filter_graph_capacity`

#### 7.2.11.5 外部覆盖字段 `ExternalOverride`
- `force_source_transcode`
- `prefer_source_passthrough`
- `target_video_codec`
- `target_audio_codec`
- `target_protocols[]`
- `biz_force_low_bitrate`

#### 7.2.11.6 运行态检测字段 `RuntimeHealth`
- `recent_publish_disconnect_count`
- `recent_pts_jump_count`
- `recent_mux_error_count`
- `recent_output_error_count`
- `source_passthrough_fail_count`

### 7.2.12 `source` 自动判定伪实现（if/else，可直接照写）

```go
type SourceDecisionResult struct {
    Mode            string // passthrough / transmux / transcode / reject
    RuleID          string // S001 ... S099
    Reason          string
    NeedVideoEncode bool
    NeedAudioEncode bool
    NeedRemux       bool
}

func DecideSourceMode(
    in InputProbe,
    channel ChannelConfig,
    sourceCfg SourceRenditionConfig,
    nodeCaps NodeCapabilities,
    ext ExternalOverride,
    health RuntimeHealth,
) SourceDecisionResult {

    if ext.ForceSourceTranscode {
        return SourceDecisionResult{"transcode", "S021", "外部请求强制 source 重编码", true, true, true}
    }

    if sourceCfg.SourceProcessMode == 1 || channel.SourcePassthroughMode == 1 {
        return SourceDecisionResult{"transcode", "S001", "后台配置强制 source 重编码", true, true, true}
    }

    if ext.PreferSourcePassthrough {
        if IsProtocolCompatible(in, channel, sourceCfg) &&
            IsCodecCompatibleForPassthrough(in, channel, nodeCaps) &&
            !channel.EnableWatermark &&
            !channel.EnableOutputEncryption {
            return SourceDecisionResult{"passthrough", "S022", "外部请求允许且协议兼容，source 旁路输出", false, false, true}
        }
    }

    if channel.EnableWatermark {
        return SourceDecisionResult{"transcode", "S017", "启用水印，透传无法叠加滤镜", true, true, true}
    }

    if channel.EnableOutputEncryption {
        return SourceDecisionResult{"transcode", "S018", "输出加密要求统一处理，需重编码", true, true, true}
    }

    if !IsVideoCodecWhitelisted(in.VideoCodec, channel.PlayProtocols) {
        return SourceDecisionResult{"transcode", "S010", "输入视频编码不在分发白名单", true, false, true}
    }

    if !IsAudioCodecWhitelisted(in.AudioCodec, channel.PlayProtocols, channel.RequireAacAudio) {
        return SourceDecisionResult{"transcode", "S011", "输入音频编码不兼容，需转音频", false, true, true}
    }

    if !IsContainerCompatible(in.ContainerFormat, channel.PlayProtocols) {
        if CanTransmux(in, channel, nodeCaps) {
            return SourceDecisionResult{"transmux", "S012", "输入封装不兼容，走旁路转封装", false, false, true}
        }
        return SourceDecisionResult{"transcode", "S012", "输入封装不兼容且无法安全转封装", true, true, true}
    }

    if !IsProtocolCompatible(in, channel, sourceCfg) {
        return SourceDecisionResult{"transcode", "S013", "同时输出 HLS/HTTP-FLV 时关键参数不兼容", true, true, true}
    }

    if channel.ForceNormalizeGop && in.KeyintSec > MaxAllowedKeyintSec(sourceCfg) {
        return SourceDecisionResult{"transcode", "S014", "输入 GOP 过大，不利于切片与低延迟", true, false, true}
    }

    if !in.PTSMonotonic || !in.DTSMonotonic || in.PTSJumpCount > 0 || in.DTSJumpCount > 0 || in.HasCorruptPacket {
        return SourceDecisionResult{"transcode", "S015", "时间戳跳变或坏包，需重编码稳定输出", true, true, true}
    }

    if channel.SourceBitrateCapEnabled && in.AvgBitrateKbps > channel.MaxSourceBitrateKbps {
        return SourceDecisionResult{"transcode", "S020", "source 输入码率超上限", true, false, true}
    }

    if health.SourcePassthroughFailCount >= 3 || health.RecentMuxErrorCount >= 3 {
        return SourceDecisionResult{"transcode", "S015", "透传/转封装近期持续失败，切重编码", true, true, true}
    }

    if NeedEncodeByAnyPolicy(in, channel, sourceCfg) && !HasRequiredHwEncoder(nodeCaps, sourceCfg.VideoCodec) {
        return SourceDecisionResult{"reject", "S019", "当前节点无所需硬编能力，应换节点调度", false, false, false}
    }

    if sourceCfg.SourceProcessMode == 2 || channel.SourcePassthroughMode == 2 {
        return SourceDecisionResult{"passthrough", "S002", "后台配置优先透传", false, false, true}
    }

    return SourceDecisionResult{"passthrough", "S099", "所有兼容性检查通过，默认走旁路", false, false, true}
}
```

#### 7.2.12.1 关键辅助函数建议
- `IsVideoCodecWhitelisted(codec, protocols)`
- `IsAudioCodecWhitelisted(codec, protocols, requireAac)`
- `IsContainerCompatible(container, protocols)`
- `CanTransmux(input, channel, nodeCaps)`
- `IsProtocolCompatible(input, channel, sourceCfg)`
- `MaxAllowedKeyintSec(sourceCfg)`
- `HasRequiredHwEncoder(nodeCaps, targetCodec)`
- `NeedEncodeByAnyPolicy(...)`

#### 7.2.12.2 判定结果落库/缓存建议
- Redis：
  - `lts:prod:live:{channel_id}:source_mode`
  - `lts:prod:live:{channel_id}:source_rule_id`
  - `lts:prod:live:{channel_id}:source_reason`
- WebSocket：实时推送 `source_runtime_mode`、`source_decision_rule_id`
- 审计/日志：记录输入探测摘要和命中规则

### 7.2.13 单元测试用例矩阵（开发直接照测）

| 用例ID | 输入视频/音频/封装 | 频道配置 | 期望结果 | 规则ID |
|---|---|---|---|---|
| TC001 | h264 + aac + flv，无码率超限，无水印 | auto + hls/flv | passthrough | S099 |
| TC002 | h264 + aac + flv | 强制重编码 | transcode | S001 |
| TC003 | h264 + aac + flv | 优先透传 | passthrough | S002 |
| TC004 | av1 + aac + flv | auto + hls/flv | transcode | S010 |
| TC005 | h264 + mp3 + flv | require_aac_audio=1 | transcode(audio) | S011 |
| TC006 | h264 + aac + mkv | auto + hls/flv | transmux 或 transcode | S012 |
| TC007 | h264 + aac + flv，GOP=10s | force_normalize_gop=1 | transcode | S014 |
| TC008 | h264 + aac + flv，PTS 跳变 | auto | transcode | S015 |
| TC009 | h264 + aac + flv | enable_watermark=1 | transcode | S017 |
| TC010 | h264 + aac + flv | enable_output_encryption=1 | transcode | S018 |
| TC011 | h264 + aac + flv，avg_bitrate=9000 | bitrate_cap=5000 | transcode | S020 |
| TC012 | h264 + aac + flv | 外部强制重编码 | transcode | S021 |
| TC013 | h264 + aac + flv | 外部优先透传 | passthrough | S022 |
| TC014 | h264 + aac + flv，透传近期失败>=3次 | auto | transcode | S015 |
| TC015 | h264 + aac + flv，需要重编码但节点无 h264 硬编 | auto | reject / reschedule | S019 |
| TC016 | h265 + aac + ts，协议兼容且无水印 | auto + 仅hls | passthrough/transmux | S099 或 S012 |
| TC017 | h264 + opus + flv | hls+flv 双输出 | transcode(audio) | S011 |
| TC018 | h264 + aac + mp4，外部优先透传但封装不兼容 | auto | transmux/transcode | S012 |
| TC019 | h264 + aac + flv，外部优先透传但启用水印 | auto | transcode | S017 |
| TC020 | h264 + aac + flv，外部优先透传但需 DRM | auto | transcode | S018 |

#### 7.2.13.1 测试断言要求
- 必须断言：
  - `mode`
  - `ruleId`
  - `needVideoEncode`
  - `needAudioEncode`
  - `needRemux`
- 必须覆盖：
  - HTTP-FLV only
  - HLS only
  - HLS + HTTP-FLV 双输出
  - 水印开/关
  - 外部覆盖开/关
  - 节点硬编能力存在/不存在
  - 时间戳异常/正常
- 必须增加回归测试：
  - source 从 passthrough 回切 transcode
  - 中断恢复后重新判定 source 规则不漂移

### 7.2.8 Go 接口设计建议

建议在 Go 层定义以下接口：

```go
type FFmpegEngine interface {
    Probe(ctx context.Context, inputURL string) (*ProbeResult, error)
    BuildArgs(jobConfig *TranscodeJobConfig) []string
    Run(ctx context.Context, args []string) error
    Stop(force bool)
    ProgressCh() <-chan TranscodeProgress
}

type HardwareCodecSelector interface {
    SelectDecoder(inputCodec string, nodeCaps NodeCapabilities) (string, error)
    SelectEncoder(outputCodec string, nodeCaps NodeCapabilities) (string, error)
}

type SegmentSink interface {
    OnInitSegment(ctx context.Context, seg SegmentMeta) error
    OnMediaSegment(ctx context.Context, seg SegmentMeta) error
}
```

业务层只调这些接口，不直接依赖 FFmpeg 命令行细节。

### 7.2.9 命令行方案的 ffprobe 兜底用途

- `ffprobe` 命令模式同时作为生产探测和调试兜底工具
- 极端故障时可手动执行 ffprobe 命令排查问题
- 生产默认通过 `FFmpegEngine.Probe()` 接口调用

## 7.3 命令行转码管线设计

### 7.3.1 输入阶段
- 调用 `ffprobe` 命令获取源视频信息
- 解析 JSON 输出，建立输入流上下文
- 判断源编码与容器
- 优先启用硬解；仅当源视频确实不支持硬解时，才允许进入受限软解路径

### 7.3.2 滤镜阶段
- 使用 FFmpeg `-filter_complex` 参数构建 filter graph：
  - split
  - scale
  - overlay（水印）
  - format / fps / setsar
- 水印、scale、overlay 必须优先走硬件滤镜链路，避免把主要图像处理负载压回 CPU

### 7.3.3 编码阶段
- 每个清晰度单独指定编码参数
- 编码器只能选择当前节点支持的硬件编码器
- 若目标编码没有可用硬编，任务失败或不进入该节点

### 7.3.4 打包阶段
- 使用 FFmpeg `-f dash` 输出 CMAF 风格 init + m4s 分片
- 支持同一份分片元数据标记 `support_dash=1, support_hls=1`

## 7.4 FFmpeg 命令模板（生产主方案）

> 以下命令模板为生产环境主方案，通过 Go 的 `os/exec` 调用执行。

```bash
ffmpeg -hide_banner -y \
  {HWACCEL_ARGS} \
  -i "{INPUT_FILE}" \
  -i "{WATERMARK_FILE}" \
  -filter_complex "\
    {HARDWARE_FILTER_GRAPH}\
  " \
  {MAP_AND_ENCODE_ARGS} \
  -c:a aac \
  -b:a {AUDIO_BITRATE} \
  -ac {AUDIO_CHANNELS} \
  -ar {AUDIO_SAMPLE_RATE} \
  -f dash \
  -streaming 1 \
  -seg_duration {SEGMENT_DURATION} \
  -use_template 1 \
  -use_timeline 1 \
  -single_file 0 \
  -init_seg_name "{INIT_NAME_TEMPLATE}" \
  -media_seg_name "{MEDIA_NAME_TEMPLATE}" \
  -adaptation_sets "id=0,streams=v id=1,streams=a" \
  "{OUTPUT_DIR}/stream.mpd"
```

## 7.5 说明

### 7.5.1 命令模板说明

- 命令模板为生产主方案，通过 Go `os/exec` 调用
- 可直接复制到终端调试，快速定位 FFmpeg 参数问题
- 通过解析 stderr 输出实时获取转码进度

### 7.5.2 最终数据库化重点

真正持久化的不是 MPD/M3U8，而是：
- init 分片
- m4s 分片
- 这些分片的全部元数据
- 运行时的硬解/硬编/硬件滤镜执行决策结果与失败原因

## 7.6 水印滤镜设计（四角锚点 + 偏移比例）

### 7.6.1 水印锚点定义

- `1=左上`：偏移含义为「到左边距离比例」「到上边距离比例」
- `2=右上`：偏移含义为「到右边距离比例」「到上边距离比例」
- `3=左下`：偏移含义为「到左边距离比例」「到下边距离比例」
- `4=右下`：偏移含义为「到右边距离比例」「到下边距离比例」

> 说明：DESIGN.md 里此前存在 `5=居中` 的扩展锚点，本次需求明确只需要四角锚点，因此对外接口与落库只保证 1~4；若未来需要居中可再扩展。

### 7.6.2 保持比例缩放与不变形规则

目标：
- 非标准 16:9 输入在输出到 1920x1080 / 1280x720 / 854x480 等档位时不允许拉伸变形
- 4:3、电影宽银幕（如 2.35:1）都必须保持原画面纵横比
- 允许上下黑边（letterbox）或左右黑边（pillarbox）

统一规则：
- 先根据目标清晰度计算缩放后的 `video_w/video_h`
- 缩放原则：`scale = min(target_w / input_w, target_h / input_h)`
- 得到：
  - `video_w = floor(input_w * scale)`
  - `video_h = floor(input_h * scale)`
- 再 pad 到目标画布：
  - `pad_x = floor((target_w - video_w) / 2)`
  - `pad_y = floor((target_h - video_h) / 2)`

示例：
- 输入 1440x1080（4:3）输出到 1920x1080：
  - scale 后 1440x1080
  - 左右补边到 1920x1080
- 输入 1920x800（电影宽屏）输出到 1920x1080：
  - scale 后 1920x800
  - 上下补边到 1920x1080

### 7.6.3 统一坐标换算

对每个输出清晰度：
- 先按「保持比例不变形」规则对主视频做 scale + pad
- 再按 `width_ratio` 对水印图做 scale（保持水印原始比例）
- 再按锚点与偏移比例计算 overlay 位置

记：
- `main_w, main_h`：输出画布大小（例如 1920x1080）
- `video_w, video_h`：scale 后的视频有效画面大小（可能小于 main_w/main_h）
- `pad_x, pad_y`：视频画面在画布内的偏移（letterbox/pillarbox 时非 0）
- `wm_w, wm_h`：水印缩放后的宽高
- `x_ratio, y_ratio`：偏移比例（0~1），含义随 anchor 不同

计算：

- anchor=左上：
  - `x = pad_x + (video_w - wm_w) * x_ratio`
  - `y = pad_y + (video_h - wm_h) * y_ratio`

- anchor=右上：
  - `x = pad_x + (video_w - wm_w) * (1 - x_ratio)`
  - `y = pad_y + (video_h - wm_h) * y_ratio`

- anchor=左下：
  - `x = pad_x + (video_w - wm_w) * x_ratio`
  - `y = pad_y + (video_h - wm_h) * (1 - y_ratio)`

- anchor=右下：
  - `x = pad_x + (video_w - wm_w) * (1 - x_ratio)`
  - `y = pad_y + (video_h - wm_h) * (1 - y_ratio)`

### 7.6.4 safe margin

为了避免水印贴边：
- `safe_margin_ratio` 表示相对于 `video_w/video_h` 的安全边距比例
- 计算出的 x/y 需要 clamp 在：
  - `x ∈ [pad_x + safe_margin_x, pad_x + video_w - wm_w - safe_margin_x]`
  - `y ∈ [pad_y + safe_margin_y, pad_y + video_h - wm_h - safe_margin_y]`

其中：
- `safe_margin_x = video_w * safe_margin_ratio`
- `safe_margin_y = video_h * safe_margin_ratio`

## 7.7 缩略图雪碧图与 `.bin` 设计（新增）

### 7.7.1 目标

转码时需要同时生成预览缩略图体系，包括：
- 雪碧图（sprite sheet）
- 缩略图索引元数据
- `.bin` 二进制文件（内部按顺序写入 base64 编码后的缩略图片段）
- 以上对象全部上传对象存储并持久化入库

### 7.7.2 雪碧图规则

- 每张雪碧图固定包含 `10 x 10 = 100` 个缩略图单元
- 若视频总缩略图数大于 100，则自动生成多张雪碧图
- 雪碧图命名建议：
  - `{output_base_prefix}/thumbnails/sprite_0001.jpg`
  - `{output_base_prefix}/thumbnails/sprite_0002.jpg`
- 缩略图时间点默认按固定间隔抽帧
- 抽帧间隔支持后台配置，也允许请求对象覆盖（若后台允许）

### 7.7.3 `.bin` 文件规则

- 每个缩略图单元在生成后，需要把该缩略图图片内容进行 base64 编码
- 按时间顺序写入 `.bin` 文件
- `.bin` 文件内部建议采用定长头 + 变长内容的顺序记录结构：
  - `item_index`
  - `time_ms`
  - `sprite_no`
  - `row_index`
  - `col_index`
  - `payload_length`
  - `base64_payload`
- 当单个 `.bin` 文件超过配置的 `thumbnail_bin_max_bytes` 时，自动切分为多个文件
- 命名建议：
  - `{output_base_prefix}/thumbnails/preview_0001.bin`
  - `{output_base_prefix}/thumbnails/preview_0002.bin`

### 7.7.4 对象存储规则

对象存储改为后台动态多配置模型，统一规则如下：
- 后台可维护多条对象存储配置记录，每条记录支持 `enabled` 与 `priority`
- 默认选择 `enabled=1` 且优先级最高的对象存储配置
- 选择顺序按 `priority DESC, updated_at DESC, id ASC`
- 任务创建时即冻结 `output_storage_id` 与 `output_base_prefix`
- 后续即使管理员调整优先级，已创建任务仍继续使用其冻结的对象存储配置
- 若数据库中还没有已发布 runtime config 或尚未补齐对象存储配置，Worker 可使用程序内置默认值完成首启初始化；生产环境应以后台动态配置为准
- 启动 YAML 不再承载对象存储动态业务配置；对象存储配置必须通过后台 runtime config / 存储配置接口维护

对象存储路径前缀规则统一如下：
- 优先使用任务冻结的 `output_base_prefix`
- 若任务未冻结前缀，则回退使用运行时默认对象前缀
- 若后台默认 prefix 为空，则默认使用对象存储根目录

该规则适用于：
- media 分片
- 雪碧图
- `.bin` 文件
- master / rendition 播放列表
- 未来其它转码附属对象

### 7.7.5 持久化原则

以下信息必须持久化：
- 雪碧图文件记录
- `.bin` 文件记录
- 每个缩略图时间点与其所在雪碧图坐标
- 每个缩略图对应 `.bin` 文件中的偏移信息
- 对象存储 key / size / etag / hash / upload_status
- 任务冻结后的 `output_storage_id` 与 `output_base_prefix`

### 7.7.6 动态 gRPC receiver 与 etcd 配置

gRPC 入站接收端与 gRPC 出站回调目标必须拆开：
- 出站 gRPC 回调目标属于 callback config
- 入站 gRPC receiver 监听参数属于 runtime config

receiver 侧规则：
- 使用 `rpc_callback_receiver_enabled` 控制是否启用本地 gRPC receiver
- 使用 `rpc_callback_receiver_host` / `rpc_callback_receiver_port` 指定监听地址
- 使用 `rpc_callback_receiver_registry_id` 绑定后台维护的 etcd registry 配置
- registry etcd 配置与 storage / callback 一样，先落 MySQL，再缓存到 Redis，运行期优先从 Redis 读取
- receiver 内部会像 S3 / MQ 一样复用已建立的客户端状态；当 `registry_id`、`endpoints`、`service_namespace`、`lease_ttl_sec`、`dial_timeout_ms`、`listen_address` 未变化时，不重复重建 etcd 连接/租约
- 若 registry 未配置或不可用，本地 receiver 仍可直接启动，不影响 create-job 入站能力
- MQ 侧同样采用运行时配置驱动；当 `mq_host`、`mq_port`、`mq_username`、`mq_password`、`mq_vhost`、`mq_queue_name`、`mq_prefetch_count` 未变化时，复用已建立的 amqp091-go 连接与 channel
- MQ 主实现不再依赖平台专属适配器；通过 amqp091-go 走同一套连接、拉取、ack、confirm 逻辑
- 当前 amqp091-go 登录握手不额外启用 heartbeat 特殊分支，避免不同平台下因 heartbeat 参数差异造成行为漂移；请求级等待统一由 gRPC timeout / confirm timeout 控制

outbox 持久化规则：
- `t_event_outbox` 保存事件当前投递状态、重试次数、下次重试时间
- 失败时写入 `t_delivery_failure_queue`，持久化 `failure_stage`、`failure_code`、`failure_message`
- 同步持久化命中的 `callback_config_id` 与具体 `callback_target`，便于后台排查失败落点
- 管理后台重试成功后会将该事件未解决失败记录标记为 `resolved=1`

## 7.8 GPU 模板片段（用于命令行参数映射）
### 7.6.1 NVIDIA
- 解码：`-hwaccel cuda -hwaccel_output_format cuda`
- 编码：`-c:v h264_nvenc` 或 `-c:v hevc_nvenc`

### 7.6.2 Intel QSV
- `-hwaccel qsv`
- `-c:v h264_qsv` 或 `-c:v hevc_qsv`

### 7.6.3 AMD
- Windows：`h264_amf` / `hevc_amf`
- Linux：`vaapi` 路线

---

## 8. 转码任务进度查看功能设计

### 8.1 进度数据来源

转码任务进度数据来自两个层面：

#### 8.1.1 FFmpeg 进度输出

FFmpeg 在 stderr 中输出实时进度信息，格式如下：
```
frame=  120 fps= 60 q=28.0 size=    1024kB time=00:00:04.00 bitrate=2097.1kbits/s speed=2.00x
```

Worker 通过 Go 的 `os/exec` 启动 FFmpeg 子进程后，使用 goroutine 实时读取 stderr 并解析以下关键字段：
- `frame`：已编码帧数
- `fps`：当前编码帧率
- `size`：已输出数据大小
- `time`：已编码时长（HH:MM:SS.ms）
- `bitrate`：当前输出码率
- `speed`：编码速度倍率

#### 8.1.2 业务进度计算

基于 FFmpeg 输出的 `time` 和源视频总时长，计算进度百分比：

```go
percent = float64(currentTimeMs) / float64(totalDurationMs) * 100.0
```

整体任务进度按各清晰度加权计算：
```go
jobProgress = sum(renditionProgress * renditionWeight) / totalWeight
```

### 8.2 进度数据流转

```
FFmpeg stderr → Worker parseProgress() → Redis 缓存 → Scheduler/API 读取 → 前端展示
                                              ↓
                                         MySQL 定期落库
```

#### 8.2.1 Worker 侧进度上报流程

1. FFmpegRunner 启动 goroutine 实时解析 stderr
2. 每解析到一行进度信息，发送到 `progressCh` channel
3. 进度上报 goroutine 从 channel 读取，做以下处理：
   - 写入 Redis Hash：`lts:prod:job:{job_id}:progress`
   - 每 2 秒或进度变化超过 1% 时，通过低延迟集群通道（优先 gRPC streaming 或 Redis Streams）上报 Scheduler
   - 上报内容包括：job_id、rendition_id、当前帧数、fps、已编码时长、码率、速度、进度百分比
4. MySQL 只做节流后的异步落库，不参与高频进度广播与节点协同

```go
type ProgressReporter struct {
    redisClient *redis.Client
    httpClient  *http.Client
    schedulerURL string
    reportInterval time.Duration
}

func (r *ProgressReporter) Report(ctx context.Context, progress TranscodeProgress) error {
    fields := map[string]interface{}{
        "frame":          progress.Frame,
        "fps":            progress.FPS,
        "time_ms":        progress.TimeMs,
        "bitrate_kbps":   progress.BitrateKbps,
        "speed":          progress.Speed,
        "percent":        progress.Percent,
        "rendition_name": progress.RenditionName,
        "updated_at":     time.Now().UnixMilli(),
    }
    key := fmt.Sprintf("lts:prod:job:%d:progress", progress.JobID)
    if err := r.redisClient.HSet(ctx, key, fields).Err(); err != nil {
        return err
    }
    r.redisClient.Expire(ctx, key, 24*time.Hour)
    return nil
}
```

#### 8.2.2 Redis 进度数据结构

```
lts:prod:job:{job_id}:progress (Hash)
├── frame            → 已编码帧数
├── fps              → 当前帧率
├── time_ms          → 已编码时长(ms)
├── bitrate_kbps     → 当前码率
├── speed            → 编码速度倍率
├── percent          → 进度百分比(0~100)
├── stage            → 当前阶段(PROBING/TRANSCODING/UPLOADING)
├── updated_at       → 最后更新时间戳
└── renditions       → JSON: 各清晰度独立进度

lts:prod:job:{job_id}:rendition:{rendition_id}:progress (Hash)
├── frame
├── fps
├── time_ms
├── percent
├── segment_count_video   → 已生成视频分片数
├── segment_count_audio   → 已生成音频分片数
├── uploaded_count        → 已上传分片数
└── updated_at
```

### 8.3 进度查询接口

#### 8.3.1 查询任务进度（HTTP）

**GET** `/api/transcode/job/progress?job_id={job_id}`

响应：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "job_id": 10001,
    "status": 4,
    "stage": "TRANSCODING",
    "progress_permille": 650,
    "current_fps": 60.5,
    "current_bitrate_kbps": 5000.2,
    "current_speed": 2.0,
    "elapsed_ms": 120000,
    "estimated_remaining_ms": 60000,
    "renditions": [
      {
        "rendition_id": 20001,
        "rendition_name": "1080p",
        "status": 4,
        "progress_permille": 700,
        "frame": 4200,
        "fps": 60.5,
        "time_ms": 70000,
        "bitrate_kbps": 5000,
        "speed": 2.0,
        "segment_count_video": 17,
        "segment_count_audio": 17,
        "uploaded_count": 15
      },
      {
        "rendition_id": 20002,
        "rendition_name": "720p",
        "status": 4,
        "progress_permille": 600,
        "frame": 3600,
        "fps": 58.2,
        "time_ms": 60000,
        "bitrate_kbps": 2800,
        "speed": 1.95,
        "segment_count_video": 15,
        "segment_count_audio": 15,
        "uploaded_count": 13
      }
    ]
  }
}
```

#### 8.3.2 查询任务进度（gRPC）

```proto
message GetJobProgressRequest {
  uint64 job_id = 1;
}

message RenditionProgress {
  uint64 rendition_id = 1;
  string rendition_name = 2;
  int32 status = 3;
  int32 progress_permille = 4;
  int64 frame = 5;
  double fps = 6;
  int64 time_ms = 7;
  double bitrate_kbps = 8;
  double speed = 9;
  int32 segment_count_video = 10;
  int32 segment_count_audio = 11;
  int32 uploaded_count = 12;
}

message GetJobProgressResponse {
  Error error = 1;
  uint64 job_id = 2;
  int32 status = 3;
  string stage = 4;
  int32 progress_permille = 5;
  double current_fps = 6;
  double current_bitrate_kbps = 7;
  double current_speed = 8;
  int64 elapsed_ms = 9;
  int64 estimated_remaining_ms = 10;
  repeated RenditionProgress renditions = 11;
}

service TranscodePublicService {
  rpc CreateJob(CreateJobRequest) returns (CreateJobResponse);
  rpc GetJobProgress(GetJobProgressRequest) returns (GetJobProgressResponse);
}
```

#### 8.3.3 后台管理进度查询

**GET** `/admin/transcode/job/progress?job_id={job_id}`

与公共接口相同，但额外返回：
- Worker 节点信息
- FFmpeg 命令行参数
- 实时 GPU 使用率
- 上传队列深度

### 8.4 WebSocket 实时进度推送

在现有 WebSocket 监控通道 `/api/admin/transcode/monitor/ws` 的 `snapshot` 消息中，增加运行中任务的实时进度字段：

```json
{
  "type": "snapshot",
  "timestamp": "1777777777777",
  "data": {
    "overview": { ... },
    "jobs": [
      {
        "job_id": 10001,
        "status": 4,
        "progress_permille": 650,
        "current_fps": 60.5,
        "current_speed": 2.0,
        "stage": "TRANSCODING",
        "renditions_progress": [
          {
            "rendition_name": "1080p",
            "progress_permille": 700,
            "fps": 60.5,
            "speed": 2.0
          }
        ]
      }
    ]
  }
}
```

### 8.5 进度阶段定义

| 阶段 | 名称 | 说明 | 进度范围 |
|---|---|---|---|
| 1 | DOWNLOADING | 下载源视频 | 0~5% |
| 2 | PROBING | 探测媒体信息 | 5~10% |
| 3 | TRANSCODING | FFmpeg 转码中 | 10~85% |
| 4 | UPLOADING | 上传分片到 S3 | 85~95% |
| 5 | FINALIZING | 更新数据库、生成事件 | 95~100% |

### 8.6 预估剩余时间计算

```go
type ETACalculator struct {
    totalDurationMs int64
    startTime       time.Time
}

func (c *ETACalculator) Calculate(currentTimeMs int64, speed float64) int64 {
    if speed <= 0 || currentTimeMs <= 0 {
        return -1
    }
    remainingMs := float64(c.totalDurationMs-currentTimeMs) / speed
    return int64(remainingMs)
}
```

### 8.7 进度数据生命周期

1. **任务创建**：Redis 初始化进度为 0%
2. **任务执行中**：Worker 实时更新 Redis，达到节流条件后异步同步到 MySQL `t_transcode_job.progress_permille` 和 `t_transcode_rendition.progress_permille`
3. **任务完成**：Redis 保留 24 小时后自动过期，MySQL 记录最终进度 1000（千分比）
4. **任务失败**：Redis 保留最后进度状态 2 小时，MySQL 记录失败时进度

---

## 9. 调度与负载均衡算法（补充细化版）

## 8.1 节点选择总流程

1. 拉取可用节点列表。
2. 过滤不符合硬约束的节点。
3. 计算候选节点分数。
4. Top-K 随机选优。
5. 写入租约。
6. 派发执行。

## 9.2 硬约束过滤

### 9.2.1 节点必须满足
- `enabled = 1`
- `quarantined = 0`
- 心跳新鲜
- 空闲会话数 > 0
- 剩余磁盘空间 > 任务最低阈值
- 上传积压未爆
- GPU 类型匹配（若外部指定 preferred_hwaccel）
- 目标任务所有清晰度要求的 `video_codec`，必须都能在该节点的 `t_worker_codec_capability(cap_type=2)` 中找到

### 9.2.2 调度器过滤伪代码（可直接开发）

```text
input:
  job
  renditions = job.required_renditions
  required_codecs = set(r.video_codec for r in renditions)
  preferred_hwaccel = job.preferred_hwaccel

candidates = []

for node in all_nodes:
  if node.enabled != 1:
    continue
  if node.quarantined == 1:
    continue
  if now - node.last_heartbeat_at > worker_heartbeat_timeout_sec:
    continue
  if node.free_session_count <= 0:
    continue
  if node.disk_free_gb < job.required_min_disk_gb:
    continue
  if node.uploader_queue_depth > uploader_queue_limit:
    continue
  if preferred_hwaccel is not empty and node does not support preferred_hwaccel:
    continue

  node_encode_caps = read latest capability snapshot from Redis cache
                     fallback to MySQL t_worker_codec_capability
                     where node_id = node.node_id
                       and cap_type = 2
                       and enabled = 1
                       and is_latest = 1

  node_codec_set = set(cap.codec_name for cap in node_encode_caps)

  if not required_codecs subset_of node_codec_set:
    continue

  if any(codec session limit exhausted on target gpu):
    continue

  candidates.append(node)

if candidates is empty:
  fail job with code = NO_COMPATIBLE_HW_ENCODER_NODE
else:
  score candidates
  choose top-k random best one
```

## 8.3 分数模型

```text
score =
  0.30 * session_util
+ 0.20 * gpu_util
+ 0.15 * cpu_util
+ 0.10 * mem_pressure
+ 0.10 * disk_pressure
+ 0.10 * upload_backlog
+ 0.05 * recent_fail_penalty
```

## 8.4 recent_fail_penalty 设计

过去 5 分钟内：
- 转码失败率高
- 上传失败率高
- 进程崩溃多

则提高惩罚系数，防止调度持续打到坏节点。

## 8.5 故障自动屏蔽与恢复

### 8.5.1 自动屏蔽触发条件
- 心跳超时
- GPU 驱动异常
- 磁盘剩余低于阈值
- 上传失败率连续超阈值
- Worker 崩溃频繁

### 8.5.2 自动恢复条件
- 连续 N 次心跳正常
- 资源恢复到安全水位
- 近 M 分钟错误率回落

---

## 10. 数据库详细设计（完整补充版）

> 说明：以下继续沿用之前核心表，并在此基础上完整补全其余所有表与外键关系说明。

## 9.1 核心任务表

### 9.1.1 `t_transcode_job`（转码任务主表）

```sql
CREATE TABLE `t_transcode_job` (
  `job_id` BIGINT UNSIGNED NOT NULL COMMENT '转码任务ID（内部主键）',
  `request_id` VARCHAR(64) NOT NULL COMMENT '外部传入唯一标识符，全局唯一',
  `biz_key` VARCHAR(128) DEFAULT NULL COMMENT '业务侧资源标识，例如视频ID',
  `mode` TINYINT NOT NULL COMMENT '任务模式：1=点播转码；2=直播转码',
  `status` TINYINT NOT NULL COMMENT '任务状态：1=创建；2=排队；3=已分配；4=执行中；5=上传中；6=完成；7=失败；8=取消',
  `priority` INT NOT NULL DEFAULT 0 COMMENT '优先级，数值越大优先级越高',
  `source_url` VARCHAR(2048) NOT NULL COMMENT '源视频地址，支持http/https',
  `source_protocol` TINYINT NOT NULL COMMENT '源协议：1=http；2=https；3=其它预留',
  `source_etag` VARCHAR(128) DEFAULT NULL COMMENT '源文件ETag',
  `source_content_length` BIGINT UNSIGNED DEFAULT NULL COMMENT '源文件大小（字节）',
  `source_probe_duration_ms` BIGINT UNSIGNED DEFAULT NULL COMMENT '探测到的视频时长（毫秒）',
  `source_probe_width` INT DEFAULT NULL COMMENT '探测到的视频宽度',
  `source_probe_height` INT DEFAULT NULL COMMENT '探测到的视频高度',
  `source_probe_fps_x1000` INT DEFAULT NULL COMMENT '探测到的视频帧率*1000',
  `source_video_codec_name` VARCHAR(32) DEFAULT NULL COMMENT '源视频编码名称，如h264/hevc/vp9/av1',
  `source_audio_codec_name` VARCHAR(32) DEFAULT NULL COMMENT '源音频编码名称，如aac/mp3/opus',
  `source_decode_mode` TINYINT NOT NULL DEFAULT 2 COMMENT '源解码模式：1=软件解码（仅源视频不支持硬解时例外）；2=硬件解码',
  `source_decode_hw_type` TINYINT NOT NULL DEFAULT 0 COMMENT '源解码硬件类型：0=无；1=NVDEC；2=QSV；3=VAAPI',
  `source_decode_fallback_reason` VARCHAR(256) DEFAULT NULL COMMENT '仅当源视频不支持硬解时记录软件解码回退原因',
  `profile_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '转码模板Profile ID',
  `job_config_version` BIGINT UNSIGNED NOT NULL COMMENT '任务锁定的配置版本号',
  `enable_watermark` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用水印：0=否；1=是',
  `watermark_image_url` VARCHAR(2048) DEFAULT NULL COMMENT '水印图片URL',
  `watermark_anchor` TINYINT NOT NULL DEFAULT 1 COMMENT '水印锚点：1=左上；2=右上；3=左下；4=右下',
  `watermark_x_ratio` DECIMAL(6,5) NOT NULL DEFAULT 0.05000 COMMENT '水印X相对位置比例',
  `watermark_y_ratio` DECIMAL(6,5) NOT NULL DEFAULT 0.05000 COMMENT '水印Y相对位置比例',
  `watermark_width_ratio` DECIMAL(6,5) NOT NULL DEFAULT 0.12000 COMMENT '水印宽度相对视频宽度比例',
  `watermark_opacity` DECIMAL(6,5) NOT NULL DEFAULT 1.00000 COMMENT '水印透明度',
  `thumb_interval_sec` INT NOT NULL DEFAULT 10 COMMENT '缩略图抽帧间隔（秒）',
  `thumb_width` INT NOT NULL DEFAULT 214 COMMENT '缩略图宽度',
  `thumb_height` INT NOT NULL DEFAULT 120 COMMENT '缩略图高度',
  `sprite_rows` INT NOT NULL DEFAULT 10 COMMENT '雪碧图行数',
  `sprite_cols` INT NOT NULL DEFAULT 10 COMMENT '雪碧图列数',
  `enable_thumbnail_sprite` TINYINT NOT NULL DEFAULT 1 COMMENT '是否生成雪碧图：0=否；1=是',
  `enable_thumbnail_bin` TINYINT NOT NULL DEFAULT 1 COMMENT '是否生成缩略图bin：0=否；1=是',
  `thumbnail_bin_max_bytes` BIGINT UNSIGNED NOT NULL DEFAULT 10485760 COMMENT '单个缩略图bin最大字节数',
  `aspect_keep_mode` TINYINT NOT NULL DEFAULT 1 COMMENT '画面比例保持模式：1=保持比例并补黑边',
  `segment_duration_sec` INT NOT NULL COMMENT '分片时长（秒）',
  `support_dash` TINYINT NOT NULL DEFAULT 1 COMMENT '生成分片是否支持DASH：0=否；1=是',
  `support_hls` TINYINT NOT NULL DEFAULT 1 COMMENT '生成分片是否支持HLS：0=否；1=是',
  `output_storage_id` BIGINT UNSIGNED NOT NULL COMMENT '输出对象存储配置ID',
  `output_base_prefix` VARCHAR(256) NOT NULL COMMENT '输出对象存储前缀',
  `segment_output_prefix` VARCHAR(256) DEFAULT NULL COMMENT '分片对象存储前缀',
  `thumbnail_sprite_prefix` VARCHAR(256) DEFAULT NULL COMMENT '雪碧图对象存储前缀',
  `thumbnail_bin_prefix` VARCHAR(256) DEFAULT NULL COMMENT '缩略图bin对象存储前缀',
  `callback_config_id` BIGINT UNSIGNED NOT NULL COMMENT '完成通知配置ID',
  `assigned_node_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '最近一次调度命中的节点ID',
  `assigned_worker_id` VARCHAR(64) DEFAULT NULL COMMENT '最近一次执行的Worker实例ID',
  `executor_worker_instance_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '当前或最近一次持有执行权的Worker实例记录ID',
  `selected_execution_hwaccel` VARCHAR(32) DEFAULT NULL COMMENT '调度阶段最终选中的执行模式',
  `selected_gpu_index` INT NOT NULL DEFAULT 0 COMMENT '调度阶段选中的GPU设备索引，仅用于诊断展示',
  `selected_gpu_device_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '调度阶段选中的稳定GPU设备记录ID',
  `lease_owner` VARCHAR(64) DEFAULT NULL COMMENT '租约持有者',
  `lease_generation` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '任务租约代次',
  `attempt_no` INT NOT NULL DEFAULT 0 COMMENT '任务执行尝试次数',
  `lease_expire_at` DATETIME DEFAULT NULL COMMENT '租约到期时间',
  `progress_permille` INT NOT NULL DEFAULT 0 COMMENT '总体进度千分比',
  `progress_stage` VARCHAR(32) DEFAULT NULL COMMENT '当前进度阶段：DOWNLOADING/PROBING/TRANSCODING/UPLOADING/FINALIZING',
  `current_fps` DOUBLE DEFAULT NULL COMMENT '当前编码帧率',
  `current_speed` DOUBLE DEFAULT NULL COMMENT '当前编码速度倍率',
  `current_bitrate_kbps` DOUBLE DEFAULT NULL COMMENT '当前输出码率',
  `elapsed_ms` BIGINT DEFAULT NULL COMMENT '已耗时（毫秒）',
  `estimated_remaining_ms` BIGINT DEFAULT NULL COMMENT '预估剩余时间（毫秒）',
  `error_code` VARCHAR(64) DEFAULT NULL COMMENT '失败错误码',
  `error_message` VARCHAR(1024) DEFAULT NULL COMMENT '失败错误信息',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`job_id`),
  UNIQUE KEY `uk_request_id` (`request_id`),
  KEY `idx_status_created` (`status`, `created_at`),
  KEY `idx_node_status` (`assigned_node_id`, `status`),
  KEY `idx_lease_expire` (`lease_expire_at`),
  KEY `idx_profile_id` (`profile_id`),
  KEY `idx_callback_config_id` (`callback_config_id`),
  KEY `idx_output_storage_id` (`output_storage_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='转码任务主表';
```

### 9.1.2 `t_transcode_job_request_override`（任务请求覆盖参数表）

```sql
CREATE TABLE `t_transcode_job_request_override` (
  `id` BIGINT UNSIGNED NOT NULL COMMENT '记录ID',
  `job_id` BIGINT UNSIGNED NOT NULL COMMENT '任务ID',
  `override_profile_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '覆盖后的Profile ID',
  `override_segment_duration_sec` INT DEFAULT NULL COMMENT '覆盖后的分片时长（秒）',
  `override_preferred_hwaccel` VARCHAR(32) DEFAULT NULL COMMENT '覆盖后的硬件加速偏好',
  `override_bucket_prefix` VARCHAR(256) DEFAULT NULL COMMENT '覆盖后的对象存储前缀',
  `override_segment_prefix` VARCHAR(256) DEFAULT NULL COMMENT '覆盖后的分片对象存储前缀',
  `override_thumbnail_sprite_prefix` VARCHAR(256) DEFAULT NULL COMMENT '覆盖后的雪碧图对象存储前缀',
  `override_thumbnail_bin_prefix` VARCHAR(256) DEFAULT NULL COMMENT '覆盖后的缩略图bin对象存储前缀',
  `override_thumb_interval_sec` INT DEFAULT NULL COMMENT '覆盖后的缩略图抽帧间隔秒数',
  `override_thumbnail_bin_max_bytes` BIGINT UNSIGNED DEFAULT NULL COMMENT '覆盖后的单个缩略图bin最大字节数',
  `override_enable_watermark` TINYINT DEFAULT NULL COMMENT '覆盖后的水印开关',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_job_id` (`job_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='单任务覆盖参数表';
```

### 9.1.3 `t_transcode_rendition`（任务清晰度子任务表）

```sql
CREATE TABLE `t_transcode_rendition` (
  `rendition_id` BIGINT UNSIGNED NOT NULL COMMENT '清晰度子任务ID',
  `job_id` BIGINT UNSIGNED NOT NULL COMMENT '所属任务ID',
  `rendition_name` VARCHAR(64) NOT NULL COMMENT '清晰度名称，例如1080p/720p',
  `status` TINYINT NOT NULL COMMENT '状态：1=待执行；2=执行中；3=上传中；4=完成；5=失败',
  `out_width` INT NOT NULL COMMENT '输出宽度',
  `out_height` INT NOT NULL COMMENT '输出高度',
  `out_fps_x1000` INT DEFAULT NULL COMMENT '输出帧率*1000',
  `video_codec` TINYINT NOT NULL COMMENT '视频编码：1=H264；2=H265；3=AV1预留',
  `video_bitrate_kbps` INT NOT NULL COMMENT '目标视频码率（kbps）',
  `video_maxrate_kbps` INT NOT NULL COMMENT '视频最大码率（kbps）',
  `video_bufsize_kbps` INT NOT NULL COMMENT '视频缓冲大小（kbps）',
  `gop_size` INT NOT NULL COMMENT 'GOP 大小',
  `bframes` INT NOT NULL COMMENT 'B帧数量',
  `preset` VARCHAR(32) NOT NULL COMMENT '编码预设',
  `encode_mode` TINYINT NOT NULL DEFAULT 2 COMMENT '编码模式：2=硬件编码（强制）',
  `encode_hw_type` TINYINT NOT NULL COMMENT '硬件编码类型：1=NVENC；2=QSV；3=AMF；4=VAAPI',
  `decode_mode` TINYINT NOT NULL DEFAULT 2 COMMENT '解码模式：2=硬件解码；1=软件解码仅在源视频不支持硬解时例外',
  `decode_hw_type` TINYINT NOT NULL DEFAULT 0 COMMENT '硬件解码类型：0=无；1=NVDEC；2=QSV；3=VAAPI',
  `decode_fallback_reason` VARCHAR(256) DEFAULT NULL COMMENT '仅当源视频不支持硬解时记录软件解码回退原因',
  `audio_codec` TINYINT NOT NULL COMMENT '音频编码：1=AAC；2=OPUS预留',
  `audio_bitrate_kbps` INT NOT NULL COMMENT '音频码率（kbps）',
  `audio_channels` INT NOT NULL COMMENT '音频声道数',
  `audio_sample_rate` INT NOT NULL COMMENT '音频采样率（Hz）',
  `segment_count_video` INT NOT NULL DEFAULT 0 COMMENT '视频分片数',
  `segment_count_audio` INT NOT NULL DEFAULT 0 COMMENT '音频分片数',
  `duration_ms` BIGINT UNSIGNED DEFAULT NULL COMMENT '输出时长（毫秒）',
  `progress_permille` INT NOT NULL DEFAULT 0 COMMENT '该清晰度进度千分比',
  `current_fps` DOUBLE DEFAULT NULL COMMENT '当前编码帧率',
  `current_speed` DOUBLE DEFAULT NULL COMMENT '当前编码速度倍率',
  `current_bitrate_kbps` DOUBLE DEFAULT NULL COMMENT '当前输出码率',
  `current_time_ms` BIGINT DEFAULT NULL COMMENT '当前已编码时长（毫秒）',
  `uploaded_count` INT NOT NULL DEFAULT 0 COMMENT '已上传分片数',
  `error_code` VARCHAR(64) DEFAULT NULL COMMENT '失败错误码',
  `error_message` VARCHAR(1024) DEFAULT NULL COMMENT '失败错误信息',
  `started_at` DATETIME DEFAULT NULL COMMENT '开始时间',
  `finished_at` DATETIME DEFAULT NULL COMMENT '结束时间',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`rendition_id`),
  KEY `idx_job_id` (`job_id`),
  KEY `idx_job_status` (`job_id`, `status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='任务清晰度子任务表';
```

### 9.1.4 `t_transcode_segment`（分片明细表，修正版）

```sql
CREATE TABLE `t_transcode_segment` (
  `segment_id` BIGINT UNSIGNED NOT NULL COMMENT '分片记录ID',
  `job_id` BIGINT UNSIGNED NOT NULL COMMENT '任务ID',
  `rendition_id` BIGINT UNSIGNED NOT NULL COMMENT '清晰度子任务ID',
  `media_type` TINYINT NOT NULL COMMENT '媒体类型：1=视频；2=音频',
  `is_init_segment` TINYINT NOT NULL DEFAULT 0 COMMENT '是否为初始化分片：0=否；1=是',
  `sequence_no` INT NOT NULL COMMENT '分片序号，init分片可固定为0',
  `duration_ms` INT NOT NULL COMMENT '分片时长（毫秒）',
  `support_dash` TINYINT NOT NULL DEFAULT 1 COMMENT '该分片是否支持DASH：0=否；1=是',
  `support_hls` TINYINT NOT NULL DEFAULT 1 COMMENT '该分片是否支持HLS：0=否；1=是',
  `codec_name` VARCHAR(32) DEFAULT NULL COMMENT '该分片对应编码名称，例如h264/aac',
  `object_key` VARCHAR(1024) NOT NULL COMMENT '对象存储Key',
  `object_size_bytes` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '对象大小（字节）',
  `object_etag` VARCHAR(128) DEFAULT NULL COMMENT '对象ETag',
  `sha256` CHAR(64) DEFAULT NULL COMMENT '分片SHA256校验值',
  `start_pts_ms` BIGINT DEFAULT NULL COMMENT '分片起始PTS（毫秒）',
  `end_pts_ms` BIGINT DEFAULT NULL COMMENT '分片结束PTS（毫秒）',
  `upload_status` TINYINT NOT NULL COMMENT '上传状态：1=待上传；2=上传中；3=上传成功；4=上传失败',
  `upload_retry_count` INT NOT NULL DEFAULT 0 COMMENT '上传重试次数',
  `upload_error_message` VARCHAR(1024) DEFAULT NULL COMMENT '上传失败原因',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`segment_id`),
  UNIQUE KEY `uk_rendition_media_seq_init` (`rendition_id`, `media_type`, `sequence_no`, `is_init_segment`),
  KEY `idx_job_id` (`job_id`),
  KEY `idx_upload_status` (`upload_status`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='分片明细表（init+m4s，支持HLS/DASH双协议）';
```

### 9.1.5 `t_transcode_thumbnail_sprite`（雪碧图文件表，新增）

```sql
CREATE TABLE `t_transcode_thumbnail_sprite` (
  `sprite_id` BIGINT UNSIGNED NOT NULL COMMENT '雪碧图记录ID',
  `job_id` BIGINT UNSIGNED NOT NULL COMMENT '任务ID',
  `sprite_no` INT NOT NULL COMMENT 'job级雪碧图序号，从1开始',
  `rows_count` INT NOT NULL DEFAULT 10 COMMENT '行数',
  `cols_count` INT NOT NULL DEFAULT 10 COMMENT '列数',
  `thumb_count` INT NOT NULL DEFAULT 0 COMMENT '当前雪碧图包含缩略图数量',
  `thumb_width` INT NOT NULL COMMENT '单个缩略图宽度',
  `thumb_height` INT NOT NULL COMMENT '单个缩略图高度',
  `image_format` VARCHAR(16) NOT NULL COMMENT '图片格式，如jpeg/png',
  `object_key` VARCHAR(1024) NOT NULL COMMENT '对象存储Key',
  `object_size_bytes` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '对象大小（字节）',
  `object_etag` VARCHAR(128) DEFAULT NULL COMMENT '对象ETag',
  `sha256` CHAR(64) DEFAULT NULL COMMENT 'SHA256校验值',
  `upload_status` TINYINT NOT NULL COMMENT '上传状态：1=待上传；2=上传中；3=上传成功；4=上传失败',
  `upload_retry_count` INT NOT NULL DEFAULT 0 COMMENT '上传重试次数',
  `upload_error_message` VARCHAR(1024) DEFAULT NULL COMMENT '上传失败原因',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`sprite_id`),
  UNIQUE KEY `uk_job_sprite_no` (`job_id`, `sprite_no`),
  KEY `idx_job_id` (`job_id`),
  KEY `idx_upload_status` (`upload_status`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='job级缩略图雪碧图文件表';
```

### 9.1.6 `t_transcode_thumbnail_bin`（缩略图bin文件表，新增）

```sql
CREATE TABLE `t_transcode_thumbnail_bin` (
  `bin_id` BIGINT UNSIGNED NOT NULL COMMENT 'bin文件记录ID',
  `job_id` BIGINT UNSIGNED NOT NULL COMMENT '任务ID',
  `bin_no` INT NOT NULL COMMENT 'job级bin文件序号，从1开始',
  `item_count` INT NOT NULL DEFAULT 0 COMMENT '包含条目数',
  `max_size_bytes` BIGINT UNSIGNED NOT NULL COMMENT '该文件配置上限字节数',
  `actual_size_bytes` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '实际大小（字节）',
  `object_key` VARCHAR(1024) NOT NULL COMMENT '对象存储Key',
  `object_etag` VARCHAR(128) DEFAULT NULL COMMENT '对象ETag',
  `sha256` CHAR(64) DEFAULT NULL COMMENT 'SHA256校验值',
  `upload_status` TINYINT NOT NULL COMMENT '上传状态：1=待上传；2=上传中；3=上传成功；4=上传失败',
  `upload_retry_count` INT NOT NULL DEFAULT 0 COMMENT '上传重试次数',
  `upload_error_message` VARCHAR(1024) DEFAULT NULL COMMENT '上传失败原因',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`bin_id`),
  UNIQUE KEY `uk_job_bin_no` (`job_id`, `bin_no`),
  KEY `idx_job_id` (`job_id`),
  KEY `idx_upload_status` (`upload_status`, `updated_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='job级缩略图bin文件表';
```

### 9.1.7 `t_transcode_thumbnail_item`（缩略图索引明细表，新增）

```sql
CREATE TABLE `t_transcode_thumbnail_item` (
  `thumb_item_id` BIGINT UNSIGNED NOT NULL COMMENT '缩略图条目ID',
  `job_id` BIGINT UNSIGNED NOT NULL COMMENT '任务ID',
  `item_index` INT NOT NULL COMMENT '整个job缩略图时间轴上的顺序号，从0开始',
  `capture_time_ms` BIGINT UNSIGNED NOT NULL COMMENT '该缩略图对应时间点（毫秒）',
  `sprite_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '所属雪碧图ID',
  `sprite_no` INT DEFAULT NULL COMMENT '雪碧图序号',
  `sprite_row_index` INT DEFAULT NULL COMMENT '在雪碧图中的行号，从0开始',
  `sprite_col_index` INT DEFAULT NULL COMMENT '在雪碧图中的列号，从0开始',
  `bin_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '所属bin文件ID',
  `bin_no` INT DEFAULT NULL COMMENT 'bin文件序号',
  `bin_item_index` INT DEFAULT NULL COMMENT '在对应bin文件中的条目序号，从0开始',
  `base64_length` INT DEFAULT NULL COMMENT '该条目base64文本长度',
  `thumb_sha256` CHAR(64) DEFAULT NULL COMMENT '单张缩略图SHA256',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`thumb_item_id`),
  UNIQUE KEY `uk_job_item_index` (`job_id`, `item_index`),
  KEY `idx_job_time` (`job_id`, `capture_time_ms`),
  KEY `idx_sprite_id` (`sprite_id`),
  KEY `idx_bin_id` (`bin_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='job级缩略图条目索引表';
```

### 9.1.8 `t_transcode_failure_queue`（失败队列表）

```sql
CREATE TABLE `t_transcode_failure_queue` (
  `failure_id` BIGINT UNSIGNED NOT NULL COMMENT '失败记录ID',
  `job_id` BIGINT UNSIGNED NOT NULL COMMENT '任务ID',
  `request_id` VARCHAR(64) NOT NULL COMMENT '外部唯一标识',
  `failure_stage` TINYINT NOT NULL COMMENT '失败阶段：1=拉取；2=探测；3=转码；4=上传；5=回调；6=调度；7=MQ消费',
  `error_code` VARCHAR(64) NOT NULL COMMENT '错误码',
  `error_message` VARCHAR(1024) NOT NULL COMMENT '错误信息',
  `retry_count` INT NOT NULL DEFAULT 0 COMMENT '已重试次数',
  `max_retry_count` INT NOT NULL COMMENT '最大重试次数',
  `next_retry_at` DATETIME DEFAULT NULL COMMENT '下次重试时间',
  `manual_action_required` TINYINT NOT NULL DEFAULT 0 COMMENT '是否需要人工介入：0=否；1=是',
  `status` TINYINT NOT NULL COMMENT '状态：1=待处理；2=重试中；3=已恢复；4=放弃',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`failure_id`),
  KEY `idx_job_id` (`job_id`),
  KEY `idx_status_next_retry` (`status`, `next_retry_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='失败队列表';
```

## 9.2 配置与模板表

### 9.2.1 `t_config_version`

```sql
CREATE TABLE `t_config_version` (
  `config_version` BIGINT UNSIGNED NOT NULL COMMENT '配置版本号',
  `changed_by_admin_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '变更管理员ID',
  `change_summary` VARCHAR(256) NOT NULL COMMENT '变更摘要',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`config_version`),
  KEY `idx_admin_id` (`changed_by_admin_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='配置版本表';
```

### 9.2.2 `t_profile`

> 历史章节保留作概念说明。实际落地请以仓库中的 `t_transcode_profile` / `t_transcode_profile_rendition` 与 `sql/000_full_project_schema.sql` 为准。

```sql
CREATE TABLE `t_profile` (
  `profile_id` BIGINT UNSIGNED NOT NULL COMMENT 'Profile主键',
  `profile_name` VARCHAR(64) NOT NULL COMMENT '模板名称',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用：0=否；1=是',
  `downscale_only` TINYINT NOT NULL DEFAULT 1 COMMENT '是否仅向下转码：0=否；1=是',
  `default_enable_watermark` TINYINT NOT NULL DEFAULT 1 COMMENT '默认是否启用水印',
  `default_segment_duration_sec` INT NOT NULL COMMENT '默认分片时长（秒）',
  `default_hwaccel_preference` TINYINT NOT NULL COMMENT '默认硬件偏好：1=AUTO；2=NVIDIA；3=INTEL_QSV；4=AMD_AMF；5=VAAPI',
  `allow_software_decode_fallback` TINYINT NOT NULL DEFAULT 1 COMMENT '是否允许在源视频不支持硬解时回退软解：0=否；1=是',
  `require_hardware_encode` TINYINT NOT NULL DEFAULT 1 COMMENT '是否强制硬件编码：0=否；1=是',
  `require_hardware_watermark` TINYINT NOT NULL DEFAULT 1 COMMENT '是否强制水印优先走硬件滤镜链路：0=否；1=是',
  `soft_decode_cpu_limit_percent` INT NOT NULL DEFAULT 50 COMMENT '软解路径CPU占用上限百分比',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`profile_id`),
  UNIQUE KEY `uk_profile_name` (`profile_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='转码模板主表';
```

### 9.2.3 `t_profile_rendition`

```sql
CREATE TABLE `t_profile_rendition` (
  `profile_rendition_id` BIGINT UNSIGNED NOT NULL COMMENT '模板清晰度配置ID',
  `profile_id` BIGINT UNSIGNED NOT NULL COMMENT '模板ID',
  `rendition_name` VARCHAR(64) NOT NULL COMMENT '清晰度名称',
  `out_width` INT NOT NULL COMMENT '输出宽度',
  `out_height` INT NOT NULL COMMENT '输出高度',
  `video_codec` TINYINT NOT NULL COMMENT '视频编码：1=H264；2=H265；3=AV1（仅硬编支持时可用）',
  `video_bitrate_kbps` INT NOT NULL COMMENT '视频码率',
  `video_maxrate_kbps` INT NOT NULL COMMENT '视频最大码率',
  `video_bufsize_kbps` INT NOT NULL COMMENT '视频缓冲',
  `gop_size` INT NOT NULL COMMENT 'GOP大小',
  `bframes` INT NOT NULL COMMENT 'B帧数',
  `preset` VARCHAR(32) NOT NULL COMMENT '编码preset',
  `audio_codec` TINYINT NOT NULL COMMENT '音频编码',
  `audio_bitrate_kbps` INT NOT NULL COMMENT '音频码率',
  `audio_channels` INT NOT NULL COMMENT '音频声道数',
  `audio_sample_rate` INT NOT NULL COMMENT '音频采样率',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用',
  `sort_order` INT NOT NULL DEFAULT 0 COMMENT '排序序号',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`profile_rendition_id`),
  KEY `idx_profile_id` (`profile_id`),
  UNIQUE KEY `uk_profile_rendition_name` (`profile_id`, `rendition_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='模板清晰度配置表';
```

### 9.2.4 `t_storage_s3_config`

```sql
CREATE TABLE `t_storage_s3_config` (
  `storage_id` BIGINT UNSIGNED NOT NULL COMMENT '对象存储配置ID',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用',
  `provider_name` VARCHAR(32) NOT NULL COMMENT '提供方名称',
  `endpoint` VARCHAR(256) NOT NULL COMMENT 'S3 endpoint',
  `region` VARCHAR(64) NOT NULL COMMENT '区域',
  `bucket` VARCHAR(128) NOT NULL COMMENT '桶名',
  `access_key_id` VARCHAR(128) NOT NULL COMMENT '访问Key',
  `secret_access_key` VARCHAR(128) NOT NULL COMMENT '密钥',
  `use_https` TINYINT NOT NULL DEFAULT 1 COMMENT '是否使用HTTPS',
  `path_style` TINYINT NOT NULL DEFAULT 0 COMMENT '是否path style',
  `connect_timeout_ms` INT NOT NULL COMMENT '连接超时',
  `request_timeout_ms` INT NOT NULL COMMENT '请求超时',
  `max_connections` INT NOT NULL COMMENT '最大连接数',
  `multipart_threshold_bytes` BIGINT UNSIGNED NOT NULL COMMENT '超过该大小触发分片上传',
  `multipart_part_size_bytes` BIGINT UNSIGNED NOT NULL COMMENT '分片上传每片大小',
  `uploader_concurrency` INT NOT NULL COMMENT '上传并发数',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`storage_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='S3对象存储配置表';
```

### 9.2.5 `t_callback_config`

```sql
CREATE TABLE `t_callback_config` (
  `callback_config_id` BIGINT UNSIGNED NOT NULL COMMENT '回调配置ID',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用',
  `mode` TINYINT NOT NULL COMMENT '通知模式：1=HTTP；2=gRPC；3=MQ',
  `http_url` VARCHAR(2048) DEFAULT NULL COMMENT 'HTTP通知地址',
  `http_method` VARCHAR(8) DEFAULT NULL COMMENT 'HTTP方法',
  `http_timeout_ms` INT DEFAULT NULL COMMENT 'HTTP超时',
  `rpc_service_name` VARCHAR(128) DEFAULT NULL COMMENT 'gRPC服务名',
  `rpc_method_name` VARCHAR(128) DEFAULT NULL COMMENT 'gRPC方法名',
  `rpc_timeout_ms` INT DEFAULT NULL COMMENT 'gRPC超时',
  `mq_type` TINYINT DEFAULT NULL COMMENT 'MQ类型：1=Kafka；2=RabbitMQ；3=RedisStreams',
  `mq_topic` VARCHAR(128) DEFAULT NULL COMMENT 'MQ主题或队列名',
  `mq_key` VARCHAR(128) DEFAULT NULL COMMENT 'MQ消息键',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`callback_config_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='完成通知配置表';
```

## 9.3 MQ / gRPC / 服务发现 / 管理员 / 权限 / 审计 / 直播 / 节点故障 / 配置拆表（全部补全 SQL DDL）

> 说明：以下为完整 SQL DDL。为高并发考虑，默认不强制加数据库层 FOREIGN KEY（但 10 章给出逻辑外键关系）。如你希望强制外键，可在上线前按业务选择性加上。

### 9.3.1 管理员与权限、审计表

#### 9.3.1.1 `t_admin_user`（管理员用户表）

```sql
CREATE TABLE `t_admin_user` (
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '管理员用户ID',
  `username` VARCHAR(64) NOT NULL COMMENT '登录用户名（唯一）',
  `password_hash` VARCHAR(128) NOT NULL COMMENT '密码哈希（bcrypt/argon2）',
  `password_salt` VARCHAR(64) DEFAULT NULL COMMENT '密码盐（如算法需要）',
  `display_name` VARCHAR(64) NOT NULL COMMENT '显示名称',
  `email` VARCHAR(128) DEFAULT NULL COMMENT '邮箱',
  `phone` VARCHAR(32) DEFAULT NULL COMMENT '手机号',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '账号状态：1=启用；2=禁用；3=锁定',
  `require_otp` TINYINT NOT NULL DEFAULT 0 COMMENT '是否要求OTP：0=否；1=是',
  `failed_login_count` INT NOT NULL DEFAULT 0 COMMENT '连续登录失败次数',
  `lock_until` DATETIME DEFAULT NULL COMMENT '锁定截止时间',
  `last_login_at` DATETIME DEFAULT NULL COMMENT '最近登录时间',
  `last_login_ip` VARCHAR(64) DEFAULT NULL COMMENT '最近登录IP',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`user_id`),
  UNIQUE KEY `uk_username` (`username`),
  KEY `idx_status` (`status`),
  KEY `idx_last_login_at` (`last_login_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='后台管理员用户表';
```

#### 9.3.1.2 `t_admin_role`（角色表）

```sql
CREATE TABLE `t_admin_role` (
  `role_id` BIGINT UNSIGNED NOT NULL COMMENT '角色ID',
  `role_key` VARCHAR(64) NOT NULL COMMENT '角色标识（唯一），如super_admin/ops/read_only',
  `role_name` VARCHAR(64) NOT NULL COMMENT '角色名称（展示用）',
  `role_desc` VARCHAR(256) DEFAULT NULL COMMENT '角色描述',
  `status` TINYINT NOT NULL DEFAULT 1 COMMENT '状态：1=启用；2=禁用',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`role_id`),
  UNIQUE KEY `uk_role_key` (`role_key`),
  KEY `idx_status` (`status`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='后台角色表';
```

#### 9.3.1.3 `t_admin_permission`（权限点表）

```sql
CREATE TABLE `t_admin_permission` (
  `perm_id` BIGINT UNSIGNED NOT NULL COMMENT '权限点ID',
  `perm_key` VARCHAR(128) NOT NULL COMMENT '权限键（唯一），如job.read/config.runtime.update',
  `perm_name` VARCHAR(64) NOT NULL COMMENT '权限名称',
  `perm_desc` VARCHAR(256) DEFAULT NULL COMMENT '权限描述',
  `module` VARCHAR(64) NOT NULL COMMENT '所属模块，如job/config/cluster/system',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`perm_id`),
  UNIQUE KEY `uk_perm_key` (`perm_key`),
  KEY `idx_module` (`module`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='权限点表（细粒度到按钮）';
```

#### 9.3.1.4 `t_admin_user_role`（用户-角色关联表）

```sql
CREATE TABLE `t_admin_user_role` (
  `id` BIGINT UNSIGNED NOT NULL COMMENT '记录ID',
  `user_id` BIGINT UNSIGNED NOT NULL COMMENT '用户ID',
  `role_id` BIGINT UNSIGNED NOT NULL COMMENT '角色ID',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_user_role` (`user_id`, `role_id`),
  KEY `idx_role_id` (`role_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='管理员用户与角色关联表';
```

#### 9.3.1.5 `t_admin_role_permission`（角色-权限关联表）

```sql
CREATE TABLE `t_admin_role_permission` (
  `id` BIGINT UNSIGNED NOT NULL COMMENT '记录ID',
  `role_id` BIGINT UNSIGNED NOT NULL COMMENT '角色ID',
  `perm_id` BIGINT UNSIGNED NOT NULL COMMENT '权限点ID',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_role_perm` (`role_id`, `perm_id`),
  KEY `idx_perm_id` (`perm_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='角色与权限点关联表';
```

#### 9.3.1.6 `t_audit_log`（审计日志表）

```sql
CREATE TABLE `t_audit_log` (
  `audit_id` BIGINT UNSIGNED NOT NULL COMMENT '审计日志ID',
  `actor_user_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '操作者用户ID（为空表示系统行为）',
  `actor_username` VARCHAR(64) DEFAULT NULL COMMENT '操作者用户名（冗余）',
  `action` VARCHAR(128) NOT NULL COMMENT '动作，如config.update/job.retry/node.quarantine',
  `resource_type` VARCHAR(64) NOT NULL COMMENT '资源类型，如config/job/node/live_channel',
  `resource_id` VARCHAR(64) DEFAULT NULL COMMENT '资源ID（字符串，兼容多种主键）',
  `request_id` VARCHAR(64) DEFAULT NULL COMMENT '业务请求ID，用于串联审计与幂等链路',
  `request_ip` VARCHAR(64) DEFAULT NULL COMMENT '请求IP',
  `user_agent` VARCHAR(256) DEFAULT NULL COMMENT 'User-Agent',
  `result` TINYINT NOT NULL COMMENT '结果：1=成功；2=失败',
  `error_code` VARCHAR(64) DEFAULT NULL COMMENT '失败错误码',
  `error_message` VARCHAR(512) DEFAULT NULL COMMENT '失败错误信息',
  `extra_json` TEXT DEFAULT NULL COMMENT '扩展信息JSON（可选）',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`audit_id`),
  KEY `idx_action_time` (`action`, `created_at`),
  KEY `idx_actor_time` (`actor_user_id`, `created_at`),
  KEY `idx_resource` (`resource_type`, `resource_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='后台审计日志表';
```

### 9.3.2 MQ 配置表

#### 9.3.2.1 `t_mq_kafka_config`（Kafka 配置表）

```sql
CREATE TABLE `t_mq_kafka_config` (
  `mq_id` BIGINT UNSIGNED NOT NULL COMMENT 'Kafka配置ID',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用：0=否；1=是',
  `name` VARCHAR(64) NOT NULL COMMENT '配置名称（唯一）',
  `brokers` VARCHAR(512) NOT NULL COMMENT 'Broker列表（逗号分隔）',
  `client_id` VARCHAR(128) NOT NULL COMMENT '客户端ID',
  `security_protocol` VARCHAR(32) NOT NULL DEFAULT 'PLAINTEXT' COMMENT '协议：PLAINTEXT/SASL_PLAINTEXT/SASL_SSL/SSL',
  `sasl_enabled` TINYINT NOT NULL DEFAULT 0 COMMENT '是否启用SASL：0=否；1=是',
  `sasl_mechanism` VARCHAR(32) DEFAULT NULL COMMENT 'SASL机制：PLAIN/SCRAM-SHA-256/SCRAM-SHA-512',
  `sasl_username` VARCHAR(128) DEFAULT NULL COMMENT 'SASL用户名',
  `sasl_password` VARCHAR(128) DEFAULT NULL COMMENT 'SASL密码',
  `tls_ca_path` VARCHAR(256) DEFAULT NULL COMMENT 'CA证书路径',
  `tls_cert_path` VARCHAR(256) DEFAULT NULL COMMENT '客户端证书路径',
  `tls_key_path` VARCHAR(256) DEFAULT NULL COMMENT '客户端私钥路径',
  `connect_timeout_ms` INT NOT NULL COMMENT '连接超时(ms)',
  `request_timeout_ms` INT NOT NULL COMMENT '请求超时(ms)',
  `producer_acks` VARCHAR(8) NOT NULL DEFAULT 'all' COMMENT '生产者acks策略：0/1/all',
  `producer_retries` INT NOT NULL DEFAULT 3 COMMENT '生产者重试次数',
  `producer_linger_ms` INT NOT NULL DEFAULT 5 COMMENT 'producer linger(ms)',
  `producer_batch_size` INT NOT NULL DEFAULT 16384 COMMENT 'producer batch size(字节)',
  `consumer_group` VARCHAR(128) NOT NULL COMMENT '消费者组名',
  `consumer_auto_offset_reset` VARCHAR(16) NOT NULL DEFAULT 'latest' COMMENT 'offset策略：latest/earliest',
  `topic_job_create` VARCHAR(128) NOT NULL COMMENT '创建任务topic',
  `topic_job_completed` VARCHAR(128) NOT NULL COMMENT '完成事件topic',
  `topic_job_failed` VARCHAR(128) NOT NULL COMMENT '失败事件topic',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`mq_id`),
  UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Kafka消息队列配置表';
```

#### 9.3.2.2 `t_mq_rabbitmq_config`（RabbitMQ 配置表）

```sql
CREATE TABLE `t_mq_rabbitmq_config` (
  `mq_id` BIGINT UNSIGNED NOT NULL COMMENT 'RabbitMQ配置ID',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用：0=否；1=是',
  `name` VARCHAR(64) NOT NULL COMMENT '配置名称（唯一）',
  `host` VARCHAR(128) NOT NULL COMMENT '主机',
  `port` INT NOT NULL COMMENT '端口',
  `vhost` VARCHAR(64) NOT NULL COMMENT 'vhost',
  `username` VARCHAR(128) NOT NULL COMMENT '用户名',
  `password` VARCHAR(128) NOT NULL COMMENT '密码',
  `tls_enabled` TINYINT NOT NULL DEFAULT 0 COMMENT '是否启用TLS：0=否；1=是',
  `connect_timeout_ms` INT NOT NULL COMMENT '连接超时(ms)',
  `prefetch_count` INT NOT NULL DEFAULT 100 COMMENT '消费者prefetch数量',
  `exchange_name` VARCHAR(128) NOT NULL COMMENT '交换机名称',
  `exchange_type` VARCHAR(16) NOT NULL DEFAULT 'topic' COMMENT '交换机类型：direct/topic/fanout',
  `queue_job_create` VARCHAR(128) NOT NULL COMMENT '创建任务队列名',
  `queue_job_completed` VARCHAR(128) NOT NULL COMMENT '完成事件队列名',
  `queue_job_failed` VARCHAR(128) NOT NULL COMMENT '失败事件队列名',
  `routing_key_job_create` VARCHAR(128) NOT NULL COMMENT '创建任务routing key',
  `routing_key_job_completed` VARCHAR(128) NOT NULL COMMENT '完成事件routing key',
  `routing_key_job_failed` VARCHAR(128) NOT NULL COMMENT '失败事件routing key',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`mq_id`),
  UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='RabbitMQ消息队列配置表';
```

#### 9.3.2.3 `t_mq_redis_streams_config`（Redis Streams 配置表）

```sql
CREATE TABLE `t_mq_redis_streams_config` (
  `mq_id` BIGINT UNSIGNED NOT NULL COMMENT 'RedisStreams配置ID',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用：0=否；1=是',
  `name` VARCHAR(64) NOT NULL COMMENT '配置名称（唯一）',
  `redis_host` VARCHAR(128) NOT NULL COMMENT 'Redis主机',
  `redis_port` INT NOT NULL COMMENT 'Redis端口',
  `redis_db` INT NOT NULL COMMENT 'Redis DB编号',
  `redis_password` VARCHAR(128) DEFAULT NULL COMMENT 'Redis密码',
  `connect_timeout_ms` INT NOT NULL COMMENT '连接超时(ms)',
  `stream_job_create` VARCHAR(128) NOT NULL COMMENT '创建任务Stream key',
  `stream_job_completed` VARCHAR(128) NOT NULL COMMENT '完成事件Stream key',
  `stream_job_failed` VARCHAR(128) NOT NULL COMMENT '失败事件Stream key',
  `consumer_group` VARCHAR(128) NOT NULL COMMENT '消费者组',
  `consumer_name_prefix` VARCHAR(64) NOT NULL COMMENT '消费者名称前缀',
  `read_count` INT NOT NULL DEFAULT 50 COMMENT '每次读取条数',
  `block_ms` INT NOT NULL DEFAULT 2000 COMMENT '阻塞读取时间(ms)',
  `ack_on_success` TINYINT NOT NULL DEFAULT 1 COMMENT '成功是否ACK：0=否；1=是',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`mq_id`),
  UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Redis Streams消息队列配置表';
```

### 9.3.3 gRPC 与注册中心配置表

#### 9.3.3.1 `t_rpc_server_config`（gRPC 服务端配置表）

```sql
CREATE TABLE `t_rpc_server_config` (
  `rpc_server_id` BIGINT UNSIGNED NOT NULL COMMENT 'gRPC服务端配置ID',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用：0=否；1=是',
  `name` VARCHAR(64) NOT NULL COMMENT '配置名称（唯一）',
  `bind_host` VARCHAR(128) NOT NULL COMMENT '监听地址',
  `bind_port` INT NOT NULL COMMENT '监听端口',
  `tls_enabled` TINYINT NOT NULL DEFAULT 0 COMMENT '是否启用TLS：0=否；1=是',
  `tls_cert_path` VARCHAR(256) DEFAULT NULL COMMENT '证书路径',
  `tls_key_path` VARCHAR(256) DEFAULT NULL COMMENT '私钥路径',
  `max_concurrent_calls` INT NOT NULL DEFAULT 1000 COMMENT '最大并发调用数（限流）',
  `max_msg_bytes` INT NOT NULL DEFAULT 4194304 COMMENT '单条消息最大字节数',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`rpc_server_id`),
  UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='gRPC服务端配置表';
```

#### 9.3.3.2 `t_rpc_client_route`（gRPC 客户端路由配置表）

```sql
CREATE TABLE `t_rpc_client_route` (
  `route_id` BIGINT UNSIGNED NOT NULL COMMENT '路由ID',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用：0=否；1=是',
  `service_name` VARCHAR(128) NOT NULL COMMENT 'gRPC服务名',
  `method_name` VARCHAR(128) NOT NULL COMMENT 'gRPC方法名',
  `timeout_ms` INT NOT NULL DEFAULT 3000 COMMENT '超时(ms)',
  `retry_count` INT NOT NULL DEFAULT 0 COMMENT '重试次数',
  `lb_policy` TINYINT NOT NULL DEFAULT 1 COMMENT '负载均衡策略：1=RR；2=随机；3=最小连接；4=一致性哈希',
  `circuit_breaker_enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用熔断：0=否；1=是',
  `circuit_breaker_fail_threshold` INT NOT NULL DEFAULT 20 COMMENT '熔断失败阈值（窗口内）',
  `circuit_breaker_window_sec` INT NOT NULL DEFAULT 10 COMMENT '熔断窗口秒数',
  `circuit_breaker_sleep_sec` INT NOT NULL DEFAULT 5 COMMENT '熔断后休眠秒数',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`route_id`),
  UNIQUE KEY `uk_service_method` (`service_name`, `method_name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='gRPC客户端路由配置表（用于回调/内部调用）';
```

#### 9.3.3.3 `t_registry_etcd_config`（etcd 服务发现配置表）

```sql
CREATE TABLE `t_registry_etcd_config` (
  `registry_id` BIGINT UNSIGNED NOT NULL COMMENT 'etcd服务发现配置ID',
  `enabled` TINYINT NOT NULL DEFAULT 0 COMMENT '是否启用：0=否；1=是',
  `name` VARCHAR(64) NOT NULL COMMENT '配置名称（唯一）',
  `endpoints` VARCHAR(512) NOT NULL COMMENT 'etcd endpoints（逗号分隔）',
  `namespace` VARCHAR(128) NOT NULL COMMENT '命名空间前缀',
  `ttl_sec` INT NOT NULL DEFAULT 10 COMMENT '注册TTL秒数',
  `auth_enabled` TINYINT NOT NULL DEFAULT 0 COMMENT '是否启用鉴权：0=否；1=是',
  `username` VARCHAR(128) DEFAULT NULL COMMENT '用户名',
  `password` VARCHAR(128) DEFAULT NULL COMMENT '密码',
  `tls_enabled` TINYINT NOT NULL DEFAULT 0 COMMENT '是否启用TLS：0=否；1=是',
  `tls_ca_path` VARCHAR(256) DEFAULT NULL COMMENT 'CA路径',
  `tls_cert_path` VARCHAR(256) DEFAULT NULL COMMENT '证书路径',
  `tls_key_path` VARCHAR(256) DEFAULT NULL COMMENT '私钥路径',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`registry_id`),
  UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='etcd服务发现配置表';
```

#### 9.3.3.4 `t_registry_consul_config`（Consul 服务发现配置表）

```sql
CREATE TABLE `t_registry_consul_config` (
  `registry_id` BIGINT UNSIGNED NOT NULL COMMENT 'Consul服务发现配置ID',
  `enabled` TINYINT NOT NULL DEFAULT 0 COMMENT '是否启用：0=否；1=是',
  `name` VARCHAR(64) NOT NULL COMMENT '配置名称（唯一）',
  `address` VARCHAR(256) NOT NULL COMMENT 'Consul地址，如http://127.0.0.1:8500',
  `datacenter` VARCHAR(64) NOT NULL COMMENT '数据中心',
  `namespace` VARCHAR(128) NOT NULL COMMENT '命名空间前缀',
  `ttl_sec` INT NOT NULL DEFAULT 10 COMMENT '注册TTL秒数',
  `token` VARCHAR(256) DEFAULT NULL COMMENT 'ACL Token（可选）',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`registry_id`),
  UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Consul服务发现配置表';
```

#### 9.3.3.5 `t_registry_nacos_config`（Nacos 服务发现配置表）

```sql
CREATE TABLE `t_registry_nacos_config` (
  `registry_id` BIGINT UNSIGNED NOT NULL COMMENT 'Nacos服务发现配置ID',
  `enabled` TINYINT NOT NULL DEFAULT 0 COMMENT '是否启用：0=否；1=是',
  `name` VARCHAR(64) NOT NULL COMMENT '配置名称（唯一）',
  `server_addr` VARCHAR(256) NOT NULL COMMENT 'Nacos服务地址',
  `namespace_id` VARCHAR(128) NOT NULL COMMENT '命名空间ID',
  `group_name` VARCHAR(128) NOT NULL COMMENT '分组名称',
  `username` VARCHAR(128) DEFAULT NULL COMMENT '用户名',
  `password` VARCHAR(128) DEFAULT NULL COMMENT '密码',
  `ttl_sec` INT NOT NULL DEFAULT 10 COMMENT '注册TTL秒数',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`registry_id`),
  UNIQUE KEY `uk_name` (`name`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Nacos服务发现配置表';
```

### 9.3.4 运行配置拆表（按版本固化）

#### 9.3.4.1 `t_config_transcode_runtime`（全局运行配置）

> 历史拆表章节保留作概念说明。实际运行配置请以仓库中的 `t_runtime_config` 为准；其中已包含硬编强制、软解CPU保护、动态并发调控与硬件水印约束。

```sql
CREATE TABLE `t_config_transcode_runtime` (
  `id` BIGINT UNSIGNED NOT NULL COMMENT '记录ID',
  `config_version` BIGINT UNSIGNED NOT NULL COMMENT '配置版本号',
  `default_profile_id` BIGINT UNSIGNED NOT NULL COMMENT '默认Profile ID',
  `max_global_transcode_sessions` INT NOT NULL COMMENT '全局最大并发转码会话数（集群维度）',
  `max_job_enqueue_per_sec` INT NOT NULL COMMENT '每秒最大入队任务数（保护系统）',
  `job_lease_ttl_sec` INT NOT NULL COMMENT '任务租约TTL秒数',
  `worker_heartbeat_timeout_sec` INT NOT NULL COMMENT 'worker心跳超时秒数',
  `scheduler_tick_ms` INT NOT NULL DEFAULT 200 COMMENT '调度循环tick毫秒数',
  `allow_request_override_profile` TINYINT NOT NULL COMMENT '是否允许请求覆盖profile：0=否；1=是',
  `allow_request_override_segment_duration` TINYINT NOT NULL COMMENT '是否允许请求覆盖分片时长：0=否；1=是',
  `allow_request_override_hwaccel` TINYINT NOT NULL DEFAULT 1 COMMENT '是否允许请求指定硬件偏好：0=否；1=是',
  `allow_request_override_rendition_codec` TINYINT NOT NULL DEFAULT 0 COMMENT '是否允许请求覆盖每个清晰度的视频编码：0=否；1=是（仅允许硬编支持的编码）',
  `require_hardware_encode` TINYINT NOT NULL DEFAULT 1 COMMENT '是否强制硬件编码',
  `allow_software_decode_fallback` TINYINT NOT NULL DEFAULT 1 COMMENT '是否允许在源视频不支持硬解时回退软解',
  `soft_decode_cpu_limit_percent` INT NOT NULL DEFAULT 50 COMMENT '软解路径CPU占用上限百分比',
  `require_hardware_watermark` TINYINT NOT NULL DEFAULT 1 COMMENT '是否强制水印优先走硬件滤镜链路',
  `dynamic_concurrency_control_enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用动态并发调控',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_version` (`config_version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='全局转码运行配置（拆字段，按版本固化）';
```

#### 9.3.4.2 `t_config_segmenting_vod`（点播分片配置）

```sql
CREATE TABLE `t_config_segmenting_vod` (
  `id` BIGINT UNSIGNED NOT NULL COMMENT '记录ID',
  `config_version` BIGINT UNSIGNED NOT NULL COMMENT '配置版本号',
  `segment_duration_sec` INT NOT NULL COMMENT '分片时长(秒)',
  `independent_segments` TINYINT NOT NULL DEFAULT 1 COMMENT '是否独立分片：0=否；1=是',
  `save_init_segment` TINYINT NOT NULL DEFAULT 1 COMMENT '是否保存init分片：0=否；1=是',
  `support_dash` TINYINT NOT NULL DEFAULT 1 COMMENT '分片支持DASH：0=否；1=是',
  `support_hls` TINYINT NOT NULL DEFAULT 1 COMMENT '分片支持HLS：0=否；1=是',
  `cmaf_enable` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用CMAF：0=否；1=是',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_version` (`config_version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='点播分片配置（按版本固化）';
```

#### 9.3.4.3 `t_config_segmenting_live`（直播分片配置）

```sql
CREATE TABLE `t_config_segmenting_live` (
  `id` BIGINT UNSIGNED NOT NULL COMMENT '记录ID',
  `config_version` BIGINT UNSIGNED NOT NULL COMMENT '配置版本号',
  `hls_segment_duration_sec` INT NOT NULL COMMENT 'HLS分片时长(秒)',
  `hls_list_size` INT NOT NULL COMMENT 'HLS列表长度',
  `hls_delete_segments` TINYINT NOT NULL DEFAULT 1 COMMENT '是否删除过期分片：0=否；1=是',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_version` (`config_version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='直播分片配置（按版本固化）';
```

#### 9.3.4.4 `t_config_naming_template`（命名模板配置）

```sql
CREATE TABLE `t_config_naming_template` (
  `id` BIGINT UNSIGNED NOT NULL COMMENT '记录ID',
  `config_version` BIGINT UNSIGNED NOT NULL COMMENT '配置版本号',
  `output_base_prefix_tpl` VARCHAR(256) NOT NULL COMMENT '输出基础prefix模板',
  `init_seg_name_tpl` VARCHAR(256) NOT NULL COMMENT 'init分片命名模板',
  `media_seg_name_tpl` VARCHAR(256) NOT NULL COMMENT 'media分片命名模板',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_version` (`config_version`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='输出命名模板配置（按版本固化）';
```

### 9.3.5 节点与故障、实时指标表

#### 9.3.5.0 `t_worker_codec_capability`（节点编解码能力表）

说明：用于把“硬解/硬编支持哪些 codec”持久化，调度热路径优先读取 Redis 中的最新能力快照，MySQL 只作为持久化与回源校验。

```sql
CREATE TABLE `t_worker_codec_capability` (
  `id` BIGINT UNSIGNED NOT NULL COMMENT '记录ID',
  `node_id` BIGINT UNSIGNED NOT NULL COMMENT '节点ID',
  `worker_instance_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '关联的 Worker 实例记录ID',
  `gpu_device_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '关联的稳定 GPU 设备记录ID',
  `gpu_index` INT NOT NULL DEFAULT 0 COMMENT 'GPU索引（无GPU可固定为0）',
  `gpu_uuid` VARCHAR(128) DEFAULT NULL COMMENT 'GPU稳定硬件标识',
  `startup_instance_id` VARCHAR(128) DEFAULT NULL COMMENT '本次启动实例标识',
  `probe_generation` BIGINT UNSIGNED NOT NULL DEFAULT 1 COMMENT '同一启动周期内的探测代次',
  `machine_fingerprint` VARCHAR(512) DEFAULT NULL COMMENT '机器指纹',
  `codec_name` VARCHAR(32) NOT NULL COMMENT '编码名称，如h264/hevc/av1/vp9',
  `cap_type` TINYINT NOT NULL COMMENT '能力类型：1=硬件解码；2=硬件编码',
  `hw_type` TINYINT NOT NULL COMMENT '硬件类型：1=NVENC/NVDEC；2=QSV；3=AMF；4=VAAPI；5=VideoToolbox',
  `max_sessions` INT NOT NULL DEFAULT 0 COMMENT '该能力最大会话数（如未知可为0）',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用：0=否；1=是',
  `is_latest` TINYINT NOT NULL DEFAULT 1 COMMENT '是否为当前最新快照，0=否；1=是',
  `capability_payload_json` JSON DEFAULT NULL COMMENT '原始能力探测载荷摘要',
  `collected_at` DATETIME NOT NULL COMMENT '采集时间',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_node_startup_probe_codec` (`node_id`, `startup_instance_id`, `gpu_index`, `codec_name`, `cap_type`, `hw_type`, `probe_generation`),
  KEY `idx_node_type` (`node_id`, `cap_type`),
  KEY `idx_codec_type` (`codec_name`, `cap_type`),
  KEY `idx_worker_instance_latest` (`worker_instance_id`, `is_latest`, `collected_at`),
  KEY `idx_startup_instance_latest` (`startup_instance_id`, `is_latest`, `collected_at`),
  KEY `idx_gpu_uuid_latest` (`gpu_uuid`, `is_latest`, `collected_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='节点硬件编解码能力快照表';
```


#### 9.3.5.1 `t_cluster_node`（节点表）

```sql
CREATE TABLE `t_cluster_node` (
  `node_id` BIGINT UNSIGNED NOT NULL COMMENT '节点ID',
  `node_name` VARCHAR(64) NOT NULL COMMENT '节点名称（唯一）',
  `host_ip` VARCHAR(64) NOT NULL COMMENT '节点IP',
  `grpc_host` VARCHAR(128) DEFAULT NULL COMMENT '节点gRPC地址（含端口）',
  `http_host` VARCHAR(128) DEFAULT NULL COMMENT '节点HTTP地址（含端口）',
  `enabled` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用：0=否；1=是',
  `quarantined` TINYINT NOT NULL DEFAULT 0 COMMENT '是否隔离：0=否；1=是',
  `quarantine_reason` VARCHAR(256) DEFAULT NULL COMMENT '隔离原因',
  `last_state_change_at` DATETIME DEFAULT NULL COMMENT '最近一次节点状态变化时间',
  `capacity_generation` BIGINT UNSIGNED NOT NULL DEFAULT 0 COMMENT '节点能力或容量变更代次，用于调度缓存失效',
  `support_nvenc` TINYINT NOT NULL DEFAULT 0 COMMENT '是否支持NVIDIA NVENC',
  `support_qsv` TINYINT NOT NULL DEFAULT 0 COMMENT '是否支持Intel QSV',
  `support_amf` TINYINT NOT NULL DEFAULT 0 COMMENT '是否支持AMD AMF',
  `support_vaapi` TINYINT NOT NULL DEFAULT 0 COMMENT '是否支持VAAPI',
  `support_videotoolbox` TINYINT NOT NULL DEFAULT 0 COMMENT '是否支持Apple VideoToolbox',
  `cpu_cores` INT NOT NULL COMMENT 'CPU核心数',
  `memory_total_mb` INT NOT NULL COMMENT '内存总量(MB)',
  `disk_total_gb` INT NOT NULL COMMENT '磁盘总量(GB)',
  `net_up_mbps` INT NOT NULL COMMENT '上行带宽(Mbps)',
  `net_down_mbps` INT NOT NULL COMMENT '下行带宽(Mbps)',
  `max_transcode_sessions` INT NOT NULL COMMENT '该节点最大同时转码会话数',
  `max_upload_concurrency` INT NOT NULL COMMENT '该节点最大上传并发',
  `node_tags` VARCHAR(256) DEFAULT NULL COMMENT '节点标签（逗号分隔），如gpu,bj',
  `last_heartbeat_at` DATETIME DEFAULT NULL COMMENT '最后心跳时间',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`node_id`),
  UNIQUE KEY `uk_node_name` (`node_name`),
  KEY `idx_enabled_heartbeat` (`enabled`, `last_heartbeat_at`),
  KEY `idx_quarantined` (`quarantined`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='集群节点表';
```

#### 9.3.5.2 `t_worker_instance`（Worker 实例表）

```sql
CREATE TABLE `t_worker_instance` (
  `id` BIGINT UNSIGNED NOT NULL COMMENT '记录ID',
  `node_id` BIGINT UNSIGNED NOT NULL COMMENT '节点ID',
  `worker_id` VARCHAR(64) NOT NULL COMMENT 'Worker实例ID（唯一）',
  `logical_worker_id` VARCHAR(128) DEFAULT NULL COMMENT '逻辑 Worker 标识',
  `physical_worker_id` VARCHAR(128) DEFAULT NULL COMMENT '物理 Worker 标识（本次实例级）',
  `machine_fingerprint` VARCHAR(512) DEFAULT NULL COMMENT '机器指纹',
  `startup_instance_id` VARCHAR(128) DEFAULT NULL COMMENT '本次启动实例标识',
  `boot_id` VARCHAR(128) DEFAULT NULL COMMENT '系统启动或进程启动批次标识',
  `pid` INT DEFAULT NULL COMMENT '进程PID（可选）',
  `version` VARCHAR(64) DEFAULT NULL COMMENT 'Worker版本号',
  `status` TINYINT NOT NULL COMMENT '状态：1=在线；2=离线；3=隔离；4=已退出',
  `start_at` DATETIME DEFAULT NULL COMMENT '启动时间',
  `exited_at` DATETIME DEFAULT NULL COMMENT '退出时间',
  `exit_reason` VARCHAR(512) DEFAULT NULL COMMENT '退出原因',
  `last_heartbeat_at` DATETIME DEFAULT NULL COMMENT '最后心跳时间',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`id`),
  UNIQUE KEY `uk_worker` (`worker_id`),
  KEY `idx_node_status` (`node_id`, `status`),
  KEY `idx_startup_instance_id` (`startup_instance_id`),
  KEY `idx_heartbeat` (`last_heartbeat_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='Worker实例表';
```

#### 9.3.5.3 `t_node_fault_event`（节点故障事件表）

```sql
CREATE TABLE `t_node_fault_event` (
  `fault_id` BIGINT UNSIGNED NOT NULL COMMENT '故障事件ID',
  `node_id` BIGINT UNSIGNED NOT NULL COMMENT '节点ID',
  `worker_id` VARCHAR(64) DEFAULT NULL COMMENT '关联Worker实例ID（可选）',
  `fault_type` TINYINT NOT NULL COMMENT '故障类型：1=心跳超时；2=磁盘不足；3=上传拥塞；4=GPU不可用；5=进程崩溃；6=依赖不可用',
  `fault_level` TINYINT NOT NULL DEFAULT 2 COMMENT '故障级别：1=INFO；2=WARN；3=ERROR；4=FATAL',
  `fault_message` VARCHAR(512) NOT NULL COMMENT '故障描述',
  `quarantined` TINYINT NOT NULL COMMENT '是否触发隔离：0=否；1=是',
  `occurred_at` DATETIME NOT NULL COMMENT '发生时间',
  PRIMARY KEY (`fault_id`),
  KEY `idx_node_time` (`node_id`, `occurred_at`),
  KEY `idx_fault_type_time` (`fault_type`, `occurred_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='节点故障事件记录';
```

#### 9.3.5.4 `t_node_metrics_realtime`（节点实时指标快照表）

```sql
CREATE TABLE `t_node_metrics_realtime` (
  `id` BIGINT UNSIGNED NOT NULL COMMENT '记录ID',
  `node_id` BIGINT UNSIGNED NOT NULL COMMENT '节点ID',
  `cpu_usage_permille` INT NOT NULL COMMENT 'CPU使用率千分比0~1000',
  `mem_used_mb` INT NOT NULL COMMENT '内存已用(MB)',
  `disk_free_gb` INT NOT NULL COMMENT '磁盘剩余(GB)',
  `net_tx_mbps` INT NOT NULL COMMENT '当前上行Mbps',
  `net_rx_mbps` INT NOT NULL COMMENT '当前下行Mbps',
  `gpu_metrics_json` JSON DEFAULT NULL COMMENT '各 GPU 的实时使用率、显存占用、会话数等聚合指标',
  `active_transcode_sessions` INT NOT NULL COMMENT '当前转码会话数',
  `uploader_queue_depth` INT NOT NULL DEFAULT 0 COMMENT '上传队列深度',
  `local_queue_depth` INT NOT NULL DEFAULT 0 COMMENT '本地待执行队列深度',
  `collected_at` DATETIME NOT NULL COMMENT '采集时间',
  PRIMARY KEY (`id`),
  KEY `idx_node_time` (`node_id`, `collected_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='节点实时指标快照表';
```

### 9.3.6 直播表（只存频道与运行状态，不存直播分片明细）

#### 9.3.6.1 `t_live_channel`（直播频道表）

```sql
CREATE TABLE `t_live_channel` (
  `channel_id` BIGINT UNSIGNED NOT NULL COMMENT '直播频道ID',
  `channel_key` VARCHAR(64) NOT NULL COMMENT '频道唯一标识（对接推流系统）',
  `channel_name` VARCHAR(64) NOT NULL COMMENT '频道名称',
  `status` TINYINT NOT NULL COMMENT '状态：1=已创建；2=运行中；3=已停止；4=异常',
  `ingest_type` TINYINT NOT NULL COMMENT '接入类型：1=拉流(pull)；2=推流(hook通知)',
  `ingest_url` VARCHAR(2048) DEFAULT NULL COMMENT '拉流地址（ingest_type=1时使用）',
  `profile_id` BIGINT UNSIGNED NOT NULL COMMENT '使用的直播转码Profile ID（该 Profile 对应 t_live_profile_rendition）',
  `enable_source_rendition` TINYINT NOT NULL DEFAULT 1 COMMENT '是否启用原画档输出：0=否；1=是',
  `source_passthrough_mode` TINYINT NOT NULL DEFAULT 2 COMMENT '原画档策略：1=强制重编码；2=优先旁路转封装/透传；3=自动判定',
  `enable_watermark` TINYINT NOT NULL DEFAULT 0 COMMENT '直播是否启用水印：0=否；1=是',
  `watermark_image_url` VARCHAR(2048) DEFAULT NULL COMMENT '直播水印图片URL',
  `play_domain` VARCHAR(255) DEFAULT NULL COMMENT '播放域名或分发域名',
  `hls_path_prefix` VARCHAR(255) DEFAULT NULL COMMENT 'HLS 播放路径前缀',
  `flv_path_prefix` VARCHAR(255) DEFAULT NULL COMMENT 'HTTP-FLV 播放路径前缀',
  `push_domain` VARCHAR(255) DEFAULT NULL COMMENT '推流域名',
  `push_app_name` VARCHAR(64) DEFAULT NULL COMMENT '推流应用名，如 live',
  `stream_key_salt` VARCHAR(128) DEFAULT NULL COMMENT '推流码签名盐或校验辅助字段',
  `publisher_auth_mode` TINYINT NOT NULL DEFAULT 3 COMMENT '主播推流鉴权模式：1=本地推流码；2=HTTP；3=gRPC；4=HTTP优先失败回退gRPC；5=gRPC优先失败回退HTTP',
  `viewer_auth_mode` TINYINT NOT NULL DEFAULT 2 COMMENT '观众播放鉴权模式：1=无鉴权；2=HTTP；3=gRPC；4=JWT本地验签',
  `auth_callback_config_id` BIGINT UNSIGNED DEFAULT NULL COMMENT 'HTTP 鉴权配置ID（主播推流/播放均可复用）',
  `auth_rpc_route_id` BIGINT UNSIGNED DEFAULT NULL COMMENT 'gRPC 鉴权路由ID',
  `resume_timeout_sec` INT NOT NULL DEFAULT 30 COMMENT '推流中断后允许恢复的最长秒数',
  `session_idle_timeout_sec` INT NOT NULL DEFAULT 90 COMMENT '直播会话空闲超时秒数',
  `restart_on_failure` TINYINT NOT NULL DEFAULT 1 COMMENT '失败是否自动重启：0=否；1=是',
  `max_restart_times` INT NOT NULL DEFAULT 10 COMMENT '最大重启次数（窗口内）',
  `restart_window_sec` INT NOT NULL DEFAULT 60 COMMENT '重启统计窗口秒数',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`channel_id`),
  UNIQUE KEY `uk_channel_key` (`channel_key`),
  KEY `idx_status` (`status`),
  KEY `idx_profile_id` (`profile_id`),
  KEY `idx_auth_callback` (`auth_callback_config_id`),
  KEY `idx_auth_rpc` (`auth_rpc_route_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='直播频道表';
```

#### 9.3.6.3 `t_live_publish_session`（直播推流会话表）

```sql
CREATE TABLE `t_live_publish_session` (
  `session_id` BIGINT UNSIGNED NOT NULL COMMENT '推流会话ID',
  `channel_id` BIGINT UNSIGNED NOT NULL COMMENT '直播频道ID',
  `session_uuid` VARCHAR(64) NOT NULL COMMENT '会话唯一UUID',
  `publisher_uid` VARCHAR(64) DEFAULT NULL COMMENT '主播业务用户ID',
  `stream_key_digest` VARCHAR(128) DEFAULT NULL COMMENT '推流码摘要（不可逆存储）',
  `client_ip` VARCHAR(64) DEFAULT NULL COMMENT '推流端IP',
  `ingest_protocol` TINYINT NOT NULL COMMENT '接入协议：1=RTMP；2=WHIP预留；3=SRT预留',
  `auth_status` TINYINT NOT NULL COMMENT '鉴权结果：1=待校验；2=通过；3=拒绝',
  `auth_request_id` VARCHAR(64) DEFAULT NULL COMMENT '外部鉴权请求ID',
  `auth_reason` VARCHAR(256) DEFAULT NULL COMMENT '鉴权结果说明',
  `status` TINYINT NOT NULL COMMENT '会话状态：1=连接中；2=推流中；3=中断待恢复；4=已恢复；5=已结束；6=已拒绝',
  `resume_count` INT NOT NULL DEFAULT 0 COMMENT '恢复次数',
  `last_media_at` DATETIME DEFAULT NULL COMMENT '最近收到音视频数据时间',
  `started_at` DATETIME DEFAULT NULL COMMENT '开始推流时间',
  `ended_at` DATETIME DEFAULT NULL COMMENT '结束时间',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`session_id`),
  UNIQUE KEY `uk_session_uuid` (`session_uuid`),
  KEY `idx_channel_status` (`channel_id`, `status`),
  KEY `idx_last_media_at` (`last_media_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='直播推流会话表';
```

#### 9.3.6.4 `t_live_auth_attempt`（直播鉴权尝试明细表）

```sql
CREATE TABLE `t_live_auth_attempt` (
  `attempt_id` BIGINT UNSIGNED NOT NULL COMMENT '鉴权尝试ID',
  `channel_id` BIGINT UNSIGNED NOT NULL COMMENT '直播频道ID',
  `session_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '推流会话ID（播放鉴权可为空）',
  `scene` TINYINT NOT NULL COMMENT '场景：1=主播推流；2=观众播放',
  `auth_mode` TINYINT NOT NULL COMMENT '鉴权方式：1=本地；2=HTTP；3=gRPC',
  `request_target` VARCHAR(512) DEFAULT NULL COMMENT '请求目标URL或gRPC服务',
  `request_payload` TEXT DEFAULT NULL COMMENT '请求负载JSON',
  `result` TINYINT NOT NULL COMMENT '结果：1=成功；2=失败；3=超时',
  `decision_code` VARCHAR(64) DEFAULT NULL COMMENT '外部系统返回的业务决策码',
  `decision_message` VARCHAR(256) DEFAULT NULL COMMENT '外部系统返回的说明',
  `cost_ms` INT NOT NULL DEFAULT 0 COMMENT '耗时(ms)',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`attempt_id`),
  KEY `idx_channel_scene_time` (`channel_id`, `scene`, `created_at`),
  KEY `idx_session_id` (`session_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='直播鉴权尝试明细表';
```


#### 9.3.7.1 `t_event_outbox`（事件发件箱表）

```sql
CREATE TABLE `t_event_outbox` (
  `event_id` BIGINT UNSIGNED NOT NULL COMMENT '事件ID',
  `event_type` VARCHAR(64) NOT NULL COMMENT '事件类型，如transcode.completed/transcode.failed',
  `job_id` BIGINT UNSIGNED DEFAULT NULL COMMENT '关联任务ID',
  `request_id` VARCHAR(64) DEFAULT NULL COMMENT '外部request_id（冗余）',
  `payload_json` TEXT NOT NULL COMMENT '事件负载JSON',
  `status` TINYINT NOT NULL COMMENT '状态：1=待发送；2=发送中；3=已发送；4=失败',
  `retry_count` INT NOT NULL DEFAULT 0 COMMENT '已重试次数',
  `max_retry_count` INT NOT NULL DEFAULT 20 COMMENT '最大重试次数',
  `next_retry_at` DATETIME DEFAULT NULL COMMENT '下次重试时间',
  `last_error_message` VARCHAR(512) DEFAULT NULL COMMENT '最后失败原因',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  `updated_at` DATETIME NOT NULL COMMENT '更新时间',
  PRIMARY KEY (`event_id`),
  KEY `idx_status_next_retry` (`status`, `next_retry_at`),
  KEY `idx_job_id` (`job_id`),
  KEY `idx_request_id` (`request_id`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='事件Outbox表（保证可靠通知）';
```

#### 9.3.7.2 `t_event_delivery_attempt`（事件投递尝试明细表）

```sql
CREATE TABLE `t_event_delivery_attempt` (
  `attempt_id` BIGINT UNSIGNED NOT NULL COMMENT '投递尝试ID',
  `event_id` BIGINT UNSIGNED NOT NULL COMMENT '事件ID',
  `mode` TINYINT NOT NULL COMMENT '投递方式：1=HTTP；2=gRPC；3=MQ',
  `target` VARCHAR(512) NOT NULL COMMENT '投递目标（URL/Service/Topic等）',
  `result` TINYINT NOT NULL COMMENT '结果：1=成功；2=失败',
  `http_status` INT DEFAULT NULL COMMENT 'HTTP状态码（HTTP时）',
  `error_message` VARCHAR(512) DEFAULT NULL COMMENT '失败原因',
  `cost_ms` INT NOT NULL DEFAULT 0 COMMENT '耗时(ms)',
  `created_at` DATETIME NOT NULL COMMENT '创建时间',
  PRIMARY KEY (`attempt_id`),
  KEY `idx_event_id` (`event_id`),
  KEY `idx_time` (`created_at`)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COMMENT='事件投递尝试明细表';
```

---

## 11. 所有表的完整外键关系说明

> 说明：以下给出**逻辑外键关系**。高并发场景下，部分项目可选择“不在数据库层强制 FOREIGN KEY”，而在应用层保证一致性，以减少锁竞争与跨表约束开销。这里先按“逻辑上必须关联”说明。

## 10.1 主业务关系

### 10.1.1 任务主表与子表
- `t_transcode_job.profile_id` → `t_profile.profile_id`
- `t_transcode_job.output_storage_id` → `t_storage_s3_config.storage_id`
- `t_transcode_job.callback_config_id` → `t_callback_config.callback_config_id`
- `t_transcode_job.assigned_node_id` → `t_cluster_node.node_id`
- `t_transcode_job.job_config_version` → `t_config_version.config_version`

### 10.1.2 请求覆盖表
- `t_transcode_job_request_override.job_id` → `t_transcode_job.job_id`
- `t_transcode_job_request_override.override_profile_id` → `t_profile.profile_id`

- `t_transcode_rendition.job_id` → `t_transcode_job.job_id`
- `t_profile_rendition.video_codec` → `t_worker_codec_capability(codec_name, cap_type=2)`（调度/校验使用）
- `t_transcode_rendition.encode_hw_type / decode_hw_type` → `t_worker_codec_capability`（逻辑能力映射，按节点能力校验）

### 10.1.4 分片表
- `t_transcode_segment.job_id` → `t_transcode_job.job_id`
- `t_transcode_segment.rendition_id` → `t_transcode_rendition.rendition_id`

### 10.1.5 失败队列表
- `t_transcode_failure_queue.job_id` → `t_transcode_job.job_id`

## 10.2 模板与配置关系

- `t_profile_rendition.profile_id` → `t_profile.profile_id`
- `t_config_version.changed_by_admin_id` → `t_admin_user.user_id`
- `t_config_transcode_runtime.config_version` → `t_config_version.config_version`
- `t_config_segmenting_vod.config_version` → `t_config_version.config_version`
- `t_config_segmenting_live.config_version` → `t_config_version.config_version`
- `t_config_naming_template.config_version` → `t_config_version.config_version`
- `t_config_transcode_runtime.default_profile_id` → `t_profile.profile_id`

- `t_worker_codec_capability.node_id` → `t_cluster_node.node_id`
- `t_worker_instance.node_id` → `t_cluster_node.node_id`
- `t_node_metrics_realtime.node_id` → `t_cluster_node.node_id`
- `t_node_fault_event.node_id` → `t_cluster_node.node_id`
- `t_transcode_job.assigned_worker_id` → `t_worker_instance.worker_id`（逻辑外键）

## 10.4 后台权限关系

- `t_admin_user_role.user_id` → `t_admin_user.user_id`
- `t_admin_user_role.role_id` → `t_admin_role.role_id`
- `t_admin_role_permission.role_id` → `t_admin_role.role_id`
- `t_admin_role_permission.perm_id` → `t_admin_permission.perm_id`
- `t_audit_log.actor_user_id` → `t_admin_user.user_id`

## 10.5 直播关系

- `t_live_channel.profile_id` → `t_profile.profile_id`
- `t_live_profile_rendition.profile_id` → `t_profile.profile_id`
- `t_live_channel.auth_callback_config_id` → `t_callback_config.callback_config_id`
- `t_live_channel.auth_rpc_route_id` → `t_rpc_client_route.route_id`
- `t_live_publish_session.channel_id` → `t_live_channel.channel_id`
- `t_live_auth_attempt.channel_id` → `t_live_channel.channel_id`
- `t_live_auth_attempt.session_id` → `t_live_publish_session.session_id`
- `t_live_channel.profile_id` 必须关联至少 1 条 `t_live_profile_rendition.enabled=1` 的记录
- 同一 `profile_id` 下必须且只能有 1 条 `is_source=1` 且 `rendition_name='source'` 的原画档记录

## 10.6 事件投递关系

- `t_event_outbox.job_id` → `t_transcode_job.job_id`
- `t_event_delivery_attempt.event_id` → `t_event_outbox.event_id`

---

## 12. 统一业务网关风格接口设计与请求/响应 JSON 示例

> 说明：本系统接口设计**不采用标准 RESTful 资源风格**，统一参考大厂常见的“业务网关 + 动作语义 + 统一报文”模式。
>
> 统一约定：
> - 公共业务接口统一使用 `/api/...` 前缀。
> - 后台管理接口统一使用 `/admin/...` 前缀。
> - 路径按“业务域 + 对象 + 动作”命名，例如：`/api/transcode/job/create`。
> - HTTP 接口只承载传输协议语义，业务动作由路径表达，不强制套用标准资源增删改查命名。
> - 查询/获取类接口统一使用 **GET**。
> - 新增、修改、重试、发布、启动、停止等带业务动作的接口统一使用 **POST**。
> - 所有响应统一包装为：`code`、`message`、`data`。
> - HTTP、gRPC、MQ 三种入口必须共享同一套字段语义、校验规则与内部 DTO。
>
> 设计原则：
> - 对外接口优先强调业务可读性、稳定性与可审计性。
> - 后台接口优先强调动作清晰、权限边界明确、便于网关鉴权与日志留痕。
> - 不使用纯 RESTful 风格的 `/resources/{id}` 作为唯一标准，而是采用更贴近业务系统的统一接口规范。

## 12.1 公共创建任务接口

### 11.1.1 请求示例

**POST** `/api/transcode/job/create`

```json
{
  "request_id": "req_20260328_0001",
  "biz_key": "video_9527",
  "source_url": "https://cdn.example.com/9527.mp4",
  "profile_id": 1,
  "priority": 50,
  "enable_watermark": true,
  "watermark": {
    "image_url": "https://cdn.example.com/logo.png",
    "anchor": 2,
    "x_ratio": 0.03,
    "y_ratio": 0.04,
    "width_ratio": 0.10,
    "opacity": 0.95,
    "safe_margin_ratio": 0.01
  },
  "video_options": {
    "output_aspect_keep": true,
    "aspect_fill_mode": "pad_black"
  },
  "segment_options": {
    "segment_duration_sec": 4,
    "support_dash": true,
    "support_hls": true,
    "naming_template_id": 1
  },
  "thumbnail_options": {
    "enable_sprite": true,
    "sprite_rows": 10,
    "sprite_cols": 10,
    "thumb_interval_sec": 10,
    "thumb_width": 214,
    "thumb_height": 120,
    "sprite_image_format": "jpeg",
    "sprite_storage_prefix": "vod/9527/thumbs",
    "enable_binary_index": true,
    "binary_storage_prefix": "vod/9527/thumbbin",
    "binary_max_size_bytes": 10485760
  },
  "storage_options": {
    "storage_id": 1,
    "bucket_prefix": "vod/9527",
    "segment_prefix": "vod/9527/segments"
  },
  "schedule_options": {
    "preferred_hwaccel": "nvidia",
    "allow_software_decode_fallback": true
  }
}
```

### 11.1.2 成功响应示例

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "job_id": 10001,
    "request_id": "req_20260328_0001",
    "status": 2,
    "status_name": "QUEUED"
  }
}
```

### 11.1.3 幂等冲突响应示例

```json
{
  "code": 4091001,
  "message": "request_id 已存在且请求参数不一致",
  "data": {
    "existing_job_id": 10001,
    "request_id": "req_20260328_0001"
  }
}
```

### 12.1.4 编码格式不支持的失败响应示例

```json
{
  "code": 4001008,
  "message": "请求的输出编码格式不在硬件编码支持范围内",
  "data": {
    "unsupported_video_codec": "av1",
    "allowed_video_codecs": ["h264", "hevc"],
    "reason": "当前集群没有任何节点支持该编码的硬件编码"
  }
}
```

## 12.1b 公共查询任务进度接口

### 请求

**GET** `/api/transcode/job/progress?job_id={job_id}`

### 响应

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "job_id": 10001,
    "status": 4,
    "stage": "TRANSCODING",
    "progress_permille": 650,
    "current_fps": 60.5,
    "current_bitrate_kbps": 5000.2,
    "current_speed": 2.0,
    "elapsed_ms": 120000,
    "estimated_remaining_ms": 60000,
    "renditions": [
      {
        "rendition_id": 20001,
        "rendition_name": "1080p",
        "status": 4,
        "progress_permille": 700,
        "frame": 4200,
        "fps": 60.5,
        "time_ms": 70000,
        "bitrate_kbps": 5000,
        "speed": 2.0,
        "segment_count_video": 17,
        "segment_count_audio": 17,
        "uploaded_count": 15
      },
      {
        "rendition_id": 20002,
        "rendition_name": "720p",
        "status": 4,
        "progress_permille": 600,
        "frame": 3600,
        "fps": 58.2,
        "time_ms": 60000,
        "bitrate_kbps": 2800,
        "speed": 1.95,
        "segment_count_video": 15,
        "segment_count_audio": 15,
        "uploaded_count": 13
      }
    ]
  }
}
```

### 错误码

| code | 含义 |
|---|---|
| 4001001 | 任务不存在 |
| 4001002 | 无权查看该任务进度 |

## 12.2 后台登录接口

### 11.2.1 请求

**POST** `/admin/auth/login`

```json
{
  "username": "admin",
  "password": "******",
  "otp_code": "123456"
}
```

### 11.2.2 响应

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "access_token": "jwt_xxx",
    "expires_in": 7200,
    "user": {
      "user_id": 1,
      "username": "admin",
      "display_name": "系统管理员",
      "roles": ["super_admin"]
    }
  }
}
```

## 12.3 任务列表接口

### 11.3.1 请求

**GET** `/admin/transcode/job/list?status=4&page=1&page_size=20`

### 11.3.2 响应

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "page": 1,
    "page_size": 20,
    "total": 2,
    "items": [
      {
        "job_id": 10001,
        "request_id": "req_20260328_0001",
        "mode": 1,
        "status": 4,
        "status_name": "RUNNING",
        "source_url": "https://cdn.example.com/9527.mp4",
        "priority": 50,
        "assigned_node_id": 7,
        "progress_permille": 650,
        "created_at": "2026-03-28 10:00:00"
      }
    ]
  }
}
```

## 12.4 任务详情接口

### 11.4.1 请求

**GET** `/admin/transcode/job/detail?job_id=10001`

### 11.4.2 响应

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "job": {
      "job_id": 10001,
      "request_id": "req_20260328_0001",
      "status": 4,
      "progress_permille": 650,
      "assigned_node_id": 7,
      "source_probe_width": 1920,
      "source_probe_height": 1080,
      "segment_duration_sec": 4,
      "support_dash": true,
      "support_hls": true
    },
    "renditions": [
      {
        "rendition_id": 20001,
        "rendition_name": "1080p",
        "status": 4,
        "progress_permille": 700,
        "segment_count_video": 120,
        "segment_count_audio": 120
      },
      {
        "rendition_id": 20002,
        "rendition_name": "720p",
        "status": 4,
        "progress_permille": 600,
        "segment_count_video": 120,
        "segment_count_audio": 120
      }
    ],
    "segments_summary": {
      "total_segments": 482,
      "uploaded_segments": 470,
      "failed_segments": 0
    }
  }
}
```

## 12.5 重试失败任务接口

### 11.5.1 请求

**POST** `/admin/transcode/job/retry`

```json
{
  "job_id": 10001,
  "retry_mode": "continue",
  "reason": "上传临时失败，人工恢复"
}
```

### 11.5.2 响应

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "job_id": 10001,
    "status": 2,
    "status_name": "QUEUED"
  }
}
```

## 12.6 节点列表接口

### 11.6.1 请求

**GET** `/admin/system/node/list`

### 11.6.2 响应

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "items": [
      {
        "node_id": 7,
        "node_name": "gpu-node-01",
        "enabled": true,
        "quarantined": false,
        "cpu_cores": 32,
        "memory_total_mb": 131072,
        "active_transcode_sessions": 5,
        "last_heartbeat_at": "2026-03-28 10:10:01"
      }
    ]
  }
}
```

## 12.7 更新运行配置接口

### 11.7.1 请求

**POST** `/admin/config/transcode/runtime/update`

```json
{
  "default_profile_id": 1,
  "max_global_transcode_sessions": 200,
  "job_lease_ttl_sec": 60,
  "worker_heartbeat_timeout_sec": 20,
  "allow_request_override_profile": true,
  "allow_request_override_segment_duration": true,
  "allow_request_override_hwaccel": true,
  "allow_request_override_rendition_codec": true,
  "change_summary": "提升全局并发并开放分片时长与清晰度编码覆盖"
}
```

### 11.7.2 响应

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "pending_config_version": 18,
    "published": false
  }
}
```

## 12.8 发布热配置接口

### 11.8.1 请求

**POST** `/admin/config/publish`

```json
{
  "config_version": 18,
  "publish_reason": "业务高峰前配置生效"
}
```

### 11.8.2 响应

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "config_version": 18,
    "published": true,
    "effective_scope": {
      "new_jobs": true,
      "queued_jobs": true,
      "running_jobs": false
    }
  }
}
```

---

## 12.9 直播频道管理接口

### 12.9.1 创建直播频道

**POST** `/admin/live/channel/create`

```json
{
  "channel_key": "room_1001",
  "channel_name": "游戏直播间1001",
  "ingest_type": 2,
  "ingest_url": null,
  "profile_id": 9,
  "enable_source_rendition": true,
  "source_passthrough_mode": 2,
  "enable_watermark": true,
  "watermark_image_url": "https://cdn.example.com/live/logo.png",
  "play_domain": "live.example.com",
  "hls_path_prefix": "/hls",
  "flv_path_prefix": "/flv",
  "push_domain": "push.example.com",
  "push_app_name": "live",
  "publisher_auth_mode": 4,
  "viewer_auth_mode": 2,
  "auth_callback_config_id": 3,
  "auth_rpc_route_id": 2,
  "resume_timeout_sec": 30,
  "session_idle_timeout_sec": 90,
  "restart_on_failure": true,
  "max_restart_times": 10,
  "restart_window_sec": 60
}
```

响应：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "channel_id": 30001,
    "channel_key": "room_1001",
    "status": 1,
    "status_name": "CREATED",
    "publish": {
      "push_url": "rtmp://push.example.com/live",
      "stream_key": "sk_live_room_1001_xxx"
    }
  }
}
```

### 12.9.2 直播频道详情

**GET** `/admin/live/channel/detail?channel_id=30001`

响应：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "channel_id": 30001,
    "channel_key": "room_1001",
    "channel_name": "游戏直播间1001",
    "status": 2,
    "status_name": "RUNNING",
    "profile_id": 9,
    "enable_source_rendition": true,
    "source_passthrough_mode": 2,
    "source_runtime_mode": "passthrough",
    "publish": {
      "push_url": "rtmp://push.example.com/live",
      "stream_key_masked": "sk_live_room_1001_***"
    },
    "playback": {
      "master_hls_url": "https://live.example.com/hls/room_1001/master.m3u8",
      "renditions": [
        {
          "rendition_name": "source",
          "is_source": true,
          "hls_url": "https://live.example.com/hls/room_1001/source.m3u8",
          "http_flv_url": "https://live.example.com/flv/live/room_1001_source.flv"
        },
        {
          "rendition_name": "720p",
          "is_source": false,
          "hls_url": "https://live.example.com/hls/room_1001/720p.m3u8",
          "http_flv_url": "https://live.example.com/flv/live/room_1001_720p.flv"
        },
        {
          "rendition_name": "480p",
          "is_source": false,
          "hls_url": "https://live.example.com/hls/room_1001/480p.m3u8",
          "http_flv_url": "https://live.example.com/flv/live/room_1001_480p.flv"
        }
      ]
    },
    "runtime_metrics": {
      "publisher_bitrate_kbps": 4820,
      "viewer_count": 0,
      "last_segment_seq": {
        "source": 8122,
        "720p": 8122,
        "480p": 8122
      }
    }
  }
}
```

### 12.9.3 对外获取直播播放信息（HTTP）

**GET** `/api/live/channel/playback?channel_key=room_1001&user_token=jwt_xxx`

响应：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "channel_key": "room_1001",
    "status": "RUNNING",
    "play_token": "pt_xxx",
    "expire_at": "2026-03-28 13:00:00",
    "master_hls_url": "https://live.example.com/hls/room_1001/master.m3u8?token=pt_xxx",
    "renditions": [
      {
        "rendition_name": "source",
        "display_name": "原画",
        "is_source": true,
        "width": 1920,
        "height": 1080,
        "video_bitrate_kbps": 4500,
        "hls_url": "https://live.example.com/hls/room_1001/source.m3u8?token=pt_xxx",
        "http_flv_url": "https://live.example.com/flv/live/room_1001_source.flv?token=pt_xxx"
      },
      {
        "rendition_name": "720p",
        "display_name": "高清",
        "is_source": false,
        "width": 1280,
        "height": 720,
        "video_bitrate_kbps": 2000,
        "hls_url": "https://live.example.com/hls/room_1001/720p.m3u8?token=pt_xxx",
        "http_flv_url": "https://live.example.com/flv/live/room_1001_720p.flv?token=pt_xxx"
      }
    ]
  }
}
```

### 12.9.4 对外获取直播播放信息（gRPC）

```proto
service LivePlaybackPublicService {
  rpc GetPlaybackInfo(GetPlaybackInfoRequest) returns (GetPlaybackInfoResponse);
}
```

### 12.9.5 更新直播频道

**POST** `/admin/live/channel/update`

```json
{
  "channel_id": 30001,
  "profile_id": 10,
  "enable_source_rendition": true,
  "source_passthrough_mode": 3,
  "enable_watermark": false,
  "restart_on_failure": true,
  "max_restart_times": 20,
  "restart_window_sec": 120
}
```

响应：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "channel_id": 30001,
    "pending_reload": true,
    "effective_scope": {
      "new_publish_session": true,
      "current_running_session": false
    }
  }
}
```

### 12.9.6 启动直播频道

**POST** `/admin/live/channel/start`

```json
{
  "channel_id": 30001,
  "operator_comment": "晚高峰开播"
}
```

响应：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "channel_id": 30001,
    "status": 2,
    "status_name": "RUNNING"
  }
}
```

### 12.9.7 停止直播频道

**POST** `/admin/live/channel/stop`

```json
{
  "channel_id": 30001,
  "operator_comment": "主播下播"
}
```

响应：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "channel_id": 30001,
    "status": 3,
    "status_name": "STOPPED"
  }
}
```

### 12.9.8 直播 Profile 清晰度详情

**GET** `/admin/live/profile/rendition/list?profile_id=9`

响应：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "profile_id": 9,
    "items": [
      {
        "rendition_name": "source",
        "is_source": true,
        "enabled": true,
        "source_process_mode": 3,
        "out_width": 1920,
        "out_height": 1080,
        "video_codec": "h264",
        "flv_stream_name": "{channel_key}_source"
      },
      {
        "rendition_name": "1080p",
        "is_source": false,
        "enabled": true,
        "out_width": 1920,
        "out_height": 1080,
        "video_codec": "h264",
        "flv_stream_name": "{channel_key}_1080p"
      },
      {
        "rendition_name": "720p",
        "is_source": false,
        "enabled": true,
        "out_width": 1280,
        "out_height": 720,
        "video_codec": "h264",
        "flv_stream_name": "{channel_key}_720p"
      }
    ]
  }
}
```

## 12.10 集群内部接口完整 JSON 示例

> 这些接口仅内网开放，建议 mTLS + 签名鉴权。

### 11.9.1 Worker 心跳上报

**POST** `/internal/v1/worker/heartbeat`

请求：
```json
{
  "node_id": 7,
  "worker_id": "worker-gpu-07-01",
  "version": "1.0.0",
  "status": 1,
  "ts_ms": 1777777777000
}
```

响应：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "accepted": true,
    "server_ts_ms": 1777777777999
  }
}
```

### 11.9.2 节点指标上报

**POST** `/internal/v1/worker/metrics`

请求：
```json
{
  "node_id": 7,
  "worker_id": "worker-gpu-07-01",
  "cpu_usage_permille": 420,
  "mem_used_mb": 32768,
  "disk_free_gb": 900,
  "net_tx_mbps": 850,
  "net_rx_mbps": 220,
  "gpu": [
    {
      "index": 0,
      "usage_permille": 650,
      "mem_used_mb": 4096,
      "active_sessions": 3
    }
  ],
  "active_transcode_sessions": 5,
  "uploader_queue_depth": 40,
  "local_queue_depth": 2,
  "collected_at": "2026-03-28 11:00:00"
}
```

响应：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "accepted": true
  }
}
```

### 11.9.3 任务进度上报

**POST** `/internal/v1/jobs/10001/progress`

请求：
```json
{
  "worker_id": "worker-gpu-07-01",
  "stage": "UPLOAD_SEGMENTS",
  "progress_permille": 650,
  "message": "1080p 视频分片已上传 200/240",
  "renditions": [
    {
      "rendition_id": 20001,
      "progress_permille": 700,
      "segment_count_video": 240,
      "segment_count_audio": 240
    }
  ]
}
```

响应：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "job_id": 10001,
    "accepted": true
  }
}
```

### 11.9.4 任务租约续租

**POST** `/internal/v1/jobs/10001/lease/renew`

请求：
```json
{
  "worker_id": "worker-gpu-07-01",
  "lease_owner": "worker-gpu-07-01",
  "renew_seconds": 60
}
```

响应：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "job_id": 10001,
    "lease_expire_at": "2026-03-28 11:01:00"
  }
}
```

### 11.9.5 分片上报

**POST** `/internal/v1/jobs/10001/segments/report`

请求：
```json
{
  "worker_id": "worker-gpu-07-01",
  "segments": [
    {
      "rendition_id": 20001,
      "media_type": 1,
      "is_init_segment": 0,
      "sequence_no": 1,
      "duration_ms": 4000,
      "support_dash": true,
      "support_hls": true,
      "codec_name": "h264",
      "object_key": "vod/10001/1080p/video/chunk-1.m4s",
      "object_size_bytes": 524288,
      "object_etag": "etag001",
      "sha256": "abc123",
      "start_pts_ms": 0,
      "end_pts_ms": 4000,
      "upload_status": 3
    }
  ]
}
```

响应：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "saved_count": 1,
    "updated_count": 0
  }
}
```

### 11.9.6 分片上传失败回传

**POST** `/internal/v1/jobs/10001/segments/upload-failed`

请求：
```json
{
  "worker_id": "worker-gpu-07-01",
  "rendition_id": 20001,
  "media_type": 1,
  "sequence_no": 8,
  "is_init_segment": 0,
  "retry_count": 3,
  "error_message": "S3 timeout"
}
```

响应：
```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "accepted": true,
    "escalated_to_failure_queue": false
  }
}
```

## 13. WebSocket 实时消息协议

当前后台监控实时通道统一收敛为：`GET /api/admin/transcode/monitor/ws`

约束：
- 连接前先通过后台登录态建立 `admin_session`
- WebSocket 鉴权复用后台管理员 session 校验逻辑，不单独放宽权限
- 前端首屏先拉 HTTP 快照 `/api/admin/transcode/monitor/snapshot`
- Redis 只保存运行时态，不承担永久历史
- 当前实现优先保证可用性，采用“连接即全量快照 + 周期快照 + 心跳”模式

## 13.1 建连

**GET** `/api/admin/transcode/monitor/ws`

鉴权方式：
- `Authorization: Bearer <admin_session>`
- 或 Cookie：`admin_session=<token>`

## 13.2 服务端消息类型

### 13.2.1 `snapshot`

连接建立后立即返回一次，之后服务端周期性再次推送。

```json
{
  "type": "snapshot",
  "timestamp": "1777777777777",
  "data": {
    "overview": {
      "online_worker_count": 2,
      "online_channel_count": 1,
      "running_job_count": 3,
      "queued_job_count": 4,
      "finished_job_count": 10,
      "failed_job_count": 1,
      "delivered_outbox_count": 8,
      "failed_outbox_count": 2,
      "pending_outbox_failed_count": 1
    },
    "workers": [],
    "jobs": [],
    "outboxes": [],
    "channels": [],
    "outbox_failure_summary": {
      "total_failed": 2,
      "retryable_count": 1,
      "callback_not_configured_count": 0,
      "http_failed_count": 1,
      "rpc_failed_count": 0,
      "mq_failed_count": 0,
      "dispatch_failed_count": 1
    }
  }
}
```

### 13.2.2 `heartbeat`

服务端周期性推送保活消息。

```json
{
  "type": "heartbeat",
  "timestamp": "1777777777999"
}
```

## 13.3 客户端可发送消息

### 13.3.1 `ping`

客户端发送文本 `ping`，服务端立即返回 `heartbeat`。

### 13.3.2 `snapshot`

客户端发送文本 `snapshot`，服务端立即返回最新一包 `snapshot`。

## 13.4 当前推送策略

- 握手成功：立即推送 1 次 `snapshot`
- 保持连接期间：服务端每 2 秒检查一次监控快照，仅当快照内容变化时再推送新的 `snapshot`
- 每 3 次检查额外附带 1 次 `heartbeat`

后续如需进一步降低流量，可在此基础上演进为“HTTP 首屏快照 + WebSocket 增量 delta”。

---

## 14. gRPC Proto 设计（完整版示例）

```proto
syntax = "proto3";
package transcode.v1;

message Error {
  int32 code = 1;
  string message = 2;
}

message Watermark {
  string image_url = 1;
  int32 anchor = 2;
  double x_ratio = 3;
  double y_ratio = 4;
  double width_ratio = 5;
  double opacity = 6;
  double safe_margin_ratio = 7;
}

message RenditionOption {
  string name = 1;
  int32 width = 2;
  int32 height = 3;
  int32 video_bitrate_kbps = 4;
  int32 video_maxrate_kbps = 5;
  int32 video_bufsize_kbps = 6;
  string preset = 7;
}

message SegmentOptions {
  bool support_dash = 1;
  bool support_hls = 2;
  int32 segment_duration_sec = 3;
  bool independent_segments = 4;
  bool save_init_segment = 5;
  uint64 naming_template_id = 6;
}

message StorageOptions {
  uint64 storage_id = 1;
  string bucket_prefix = 2;
}

message ScheduleOptions {
  string preferred_hwaccel = 1;
  bool allow_software_decode_fallback = 2;
  int32 max_wait_seconds = 3;
}

message ThumbnailOptions {
  bool enable_sprite = 1;
  int32 sprite_rows = 2;
  int32 sprite_cols = 3;
  int32 thumb_interval_sec = 4;
  int32 thumb_width = 5;
  int32 thumb_height = 6;
  string sprite_image_format = 7;
  string sprite_storage_prefix = 8;
  bool enable_binary_index = 9;
  string binary_storage_prefix = 10;
  uint64 binary_max_size_bytes = 11;
}

message VideoOptions {
  bool output_aspect_keep = 1;
  string aspect_fill_mode = 2; // pad_black
}

message CreateJobRequest {
  string request_id = 1;
  string biz_key = 2;
  string source_url = 3;
  uint64 profile_id = 4;
  int32 priority = 5;
  bool enable_watermark = 6;
  Watermark watermark = 7;
  VideoOptions video_options = 8;
  SegmentOptions segment_options = 9;
  StorageOptions storage_options = 10;
  ThumbnailOptions thumbnail_options = 11;
  ScheduleOptions schedule_options = 12;
  repeated RenditionOption renditions = 13;
}

message CreateJobResponse {
  Error error = 1;
  uint64 job_id = 2;
  string request_id = 3;
  int32 status = 4;
  string status_name = 5;
}

message GetPlaybackInfoRequest {
  string channel_key = 1;
  string user_token = 2;
  string client_ip = 3;
  string device_id = 4;
}

message PlaybackRendition {
  string rendition_name = 1;
  string display_name = 2;
  bool is_source = 3;
  int32 width = 4;
  int32 height = 5;
  int32 video_bitrate_kbps = 6;
  string hls_url = 7;
  string http_flv_url = 8;
}

message GetPlaybackInfoResponse {
  Error error = 1;
  string channel_key = 2;
  string status = 3;
  string play_token = 4;
  string expire_at = 5;
  string master_hls_url = 6;
  repeated PlaybackRendition renditions = 7;
}

message GetJobProgressRequest {
  uint64 job_id = 1;
}

message RenditionProgressInfo {
  uint64 rendition_id = 1;
  string rendition_name = 2;
  int32 status = 3;
  int32 progress_permille = 4;
  int64 frame = 5;
  double fps = 6;
  int64 time_ms = 7;
  double bitrate_kbps = 8;
  double speed = 9;
  int32 segment_count_video = 10;
  int32 segment_count_audio = 11;
  int32 uploaded_count = 12;
}

message GetJobProgressResponse {
  Error error = 1;
  uint64 job_id = 2;
  int32 status = 3;
  string stage = 4;
  int32 progress_permille = 5;
  double current_fps = 6;
  double current_bitrate_kbps = 7;
  double current_speed = 8;
  int64 elapsed_ms = 9;
  int64 estimated_remaining_ms = 10;
  repeated RenditionProgressInfo renditions = 11;
}

service TranscodePublicService {
  rpc CreateJob(CreateJobRequest) returns (CreateJobResponse);
  rpc GetJobProgress(GetJobProgressRequest) returns (GetJobProgressResponse);
}

service LivePlaybackPublicService {
  rpc GetPlaybackInfo(GetPlaybackInfoRequest) returns (GetPlaybackInfoResponse);
}

message WorkerHeartbeatRequest {
  uint64 node_id = 1;
  string worker_id = 2;
  int64 ts_ms = 3;
}

message WorkerHeartbeatResponse {
  Error error = 1;
  bool accepted = 2;
  int64 server_ts_ms = 3;
}

message ReportJobProgressRequest {
  uint64 job_id = 1;
  string worker_id = 2;
  string stage = 3;
  int32 progress_permille = 4;
  string message = 5;
}

message ReportJobProgressResponse {
  Error error = 1;
  uint64 job_id = 2;
  bool accepted = 3;
}

service TranscodeInternalService {
  rpc WorkerHeartbeat(WorkerHeartbeatRequest) returns (WorkerHeartbeatResponse);
  rpc ReportJobProgress(ReportJobProgressRequest) returns (ReportJobProgressResponse);
}
```
}

message ReportJobProgressRequest {
  uint64 job_id = 1;
  string worker_id = 2;
  int32 progress_permille = 3;
  string stage = 4;
}

message ReportJobProgressResponse {
  Error error = 1;
}

service TranscodeInternalService {
  rpc WorkerHeartbeat(WorkerHeartbeatRequest) returns (WorkerHeartbeatResponse);
  rpc ReportJobProgress(ReportJobProgressRequest) returns (ReportJobProgressResponse);
}
```

---

## 15. 完整权限点清单

> 采用 `模块.资源.动作` 规范。

## 14.1 认证与用户
- `auth.session.read`
- `auth.session.write`
- `system.user.read`
- `system.user.create`
- `system.user.update`
- `system.user.enable`
- `system.user.disable`
- `system.user.role_bind`

## 14.2 角色与权限
- `system.role.read`
- `system.role.create`
- `system.role.update`
- `system.role.delete`
- `system.permission.read`
- `system.role.permission_bind`

## 14.3 转码任务
- `job.read`
- `job.detail.read`
- `job.retry`
- `job.restart`
- `job.cancel`
- `job.export`

## 14.4 失败队列
- `failure.read`
- `failure.retry`
- `failure.restart`
- `failure.manual_mark`

## 14.5 节点与集群
- `cluster.read`
- `cluster.node.read`
- `cluster.node.quarantine`
- `cluster.node.enable`
- `cluster.node.metrics.read`
- `cluster.dispatch.read`

## 14.6 配置中心
- `config.version.read`
- `config.version.publish`
- `config.runtime.read`
- `config.runtime.update`
- `config.profile.read`
- `config.profile.create`
- `config.profile.update`
- `config.profile.delete`
- `config.segment.read`
- `config.segment.update`
- `config.naming.read`
- `config.naming.update`
- `config.storage.read`
- `config.storage.create`
- `config.storage.update`
- `config.callback.read`
- `config.callback.update`
- `config.mq.read`
- `config.mq.update`
- `config.rpc.read`
- `config.rpc.update`
- `config.registry.read`
- `config.registry.update`

## 14.7 直播
- `live.channel.read`
- `live.channel.create`
- `live.channel.update`
- `live.channel.start`
- `live.channel.stop`
- `live.channel.restart`
- `live.channel.detail.read`
- `live.channel.metrics.read`
- `live.profile.read`
- `live.profile.update`
- `live.profile.rendition.read`
- `live.profile.rendition.update`

## 14.8 审计与报表
- `audit.read`
- `report.dashboard.read`
- `report.realtime.read`

---

## 16. 后台菜单与按钮级权限设计

## 15.1 菜单设计

### 15.1.1 顶级菜单
- 仪表盘
- 转码任务
- 失败队列
- 集群管理
- 配置中心
- 直播管理
- 审计日志
- 系统管理

## 15.2 菜单与权限映射

### 15.2.1 仪表盘
- 菜单权限：`report.dashboard.read`
- 按钮权限：
  - 查看实时图表：`report.realtime.read`

### 15.2.2 转码任务
- 菜单权限：`job.read`
- 页面按钮：
  - 查看详情：`job.detail.read`
  - 重试：`job.retry`
  - 从头重跑：`job.restart`
  - 取消：`job.cancel`
  - 导出：`job.export`

### 15.2.3 失败队列
- 菜单权限：`failure.read`
- 页面按钮：
  - 重试：`failure.retry`
  - 重跑：`failure.restart`
  - 标记人工处理：`failure.manual_mark`

### 15.2.4 集群管理
- 菜单权限：`cluster.read`
- 页面按钮：
  - 查看节点详情：`cluster.node.read`
  - 查看指标：`cluster.node.metrics.read`
  - 隔离节点：`cluster.node.quarantine`
  - 启用节点：`cluster.node.enable`

### 15.2.5 配置中心
- 菜单权限：`config.version.read`
- 页面按钮：
  - 修改运行配置：`config.runtime.update`
  - 修改存储：`config.storage.update`
  - 修改 MQ：`config.mq.update`
  - 修改 gRPC：`config.rpc.update`
  - 修改注册中心：`config.registry.update`
  - 发布配置：`config.version.publish`

### 15.2.6 直播管理
- 菜单权限：`live.channel.read`
- 页面按钮：
  - 查看频道详情：`live.channel.detail.read`
  - 查看实时指标：`live.channel.metrics.read`
  - 创建频道：`live.channel.create`
  - 编辑频道：`live.channel.update`
  - 启动：`live.channel.start`
  - 停止：`live.channel.stop`
  - 重启：`live.channel.restart`
  - 查看直播 Profile：`live.profile.read`
  - 编辑直播 Profile：`live.profile.update`
  - 查看清晰度配置：`live.profile.rendition.read`
  - 编辑清晰度配置：`live.profile.rendition.update`

### 15.2.7 审计日志
- 菜单权限：`audit.read`

### 15.2.8 系统管理
- 菜单权限：`system.user.read`
- 页面按钮：
  - 新建用户：`system.user.create`
  - 编辑用户：`system.user.update`
  - 启用/禁用用户：`system.user.enable` / `system.user.disable`
  - 分配角色：`system.user.role_bind`
  - 角色编辑：`system.role.update`
  - 角色删改：`system.role.delete`
  - 绑定权限：`system.role.permission_bind`

---

## 17. 单机模式 UI 与集群模式 UI 的差异化页面设计

## 16.1 单机模式 UI 设计重点

### 16.1.1 展示目标
- 简洁
- 聚焦当前机器
- 减少集群概念干扰

### 16.1.2 页面特征
- Dashboard 重点展示：
  - 当前机器 CPU/GPU/内存/磁盘
- 软解 CPU 限额、节点 CPU 保护阈值、上传并发与 GPU 会话上限
  - 当前转码任务数
  - 等待队列数
  - 上传带宽
- 集群页可弱化为“本机状态”页
- 节点数量固定为 1，不展示复杂调度拓扑

## 16.2 集群模式 UI 设计重点

### 16.2.1 展示目标
- 强调全局视图
- 强调节点差异
- 强调调度与故障恢复

### 16.2.2 页面特征
- Dashboard 顶部展示集群总览：
  - 在线节点数
  - 隔离节点数
  - 总运行任务数
  - 总等待队列深度
  - 总上传吞吐
- 集群管理页必须支持：
  - 节点卡片视图
  - 节点表格视图
  - 按地域/标签筛选
- 直播管理页在集群模式下额外展示：
  - 各频道当前落在哪个节点/Worker
  - 原画档当前运行模式（透传/重编码）
  - 每个清晰度实时码率、帧率、最近分片序号
  - HLS/HTTP-FLV 播放地址复制与健康检查
  - 节点热力图
- 调度页展示：
  - 当前调度权重
  - 节点评分
  - 节点最近失败率

## 16.3 页面差异化总结

### 16.3.1 单机模式更适合
- 小团队
- 一体化部署
- 本机运维

### 16.3.2 集群模式更适合
- 多机 GPU 集群
- 横向扩容
- 高吞吐任务池
- 节点级故障隔离和全局运维

---

## 18. 消息队列消费失败、回调失败、S3 上传失败的时序图级说明

> 使用文本时序图说明。

## 17.1 消息队列消费失败时序

### 17.1.1 场景
消费者收到创建任务消息，但解析或落库失败。

```text
MQ -> Consumer(API/Scheduler): 投递 job.create
Consumer -> Consumer: 反序列化消息
Consumer -> Consumer: basic.get 成功后立即 ack，再进入统一 DTO 校验与落库流程
Consumer -> MySQL: 写 t_transcode_job
MySQL --> Consumer: 失败（例如主键冲突/连接异常）
Consumer -> MySQL: 写 t_transcode_failure_queue（failure_stage=MQ消费）
Consumer -> Audit: 记录审计/错误日志
Consumer -> MQ: 不回滚已 ack 消息，失败由库内 failure queue / outbox 体系接管
后台管理员 -> FailureQueue: 查看失败消息并人工处理
```

### 17.1.2 处理原则
- 可重试错误：重试
- 不可重试错误（参数非法）：写失败队列并转死信
- 当前 RabbitMQ create-job 入站实现保持“`basic.get` 拉取后立即 ack”语义，因此后续解析/落库失败不依赖 broker 重新投递，而是进入库内失败记录与人工处理链路
- 必须保留原始消息体摘要，便于人工排查

## 17.2 回调失败时序

### 17.2.1 场景
任务已完成，但对外 HTTP/gRPC/MQ 通知失败。

```text
Worker -> MySQL: 更新任务状态为 COMPLETED
Worker -> MySQL: 写 t_event_outbox(status=PENDING)
Notifier -> MySQL: 拉取待发送 outbox
Notifier -> TargetApp(HTTP/gRPC/MQ): 发送完成通知
TargetApp --> Notifier: 超时/失败
Notifier -> MySQL: 写 t_event_delivery_attempt(result=失败)
Notifier -> MySQL: 更新 t_event_outbox(status=FAILED, retry_count+1, next_retry_at=...)
定时任务 -> MySQL: 扫描可重试 outbox
Notifier -> TargetApp: 再次发送
TargetApp --> Notifier: 成功
Notifier -> MySQL: 更新 t_event_outbox(status=SENT)
```

### 17.2.2 处理原则
- 回调失败**不能回滚主任务完成状态**。
- 回调使用 Outbox 保证最终一致性。
- 达到最大重试次数后进入人工告警。

## 17.3 S3 上传失败时序

### 17.3.1 场景
某个 m4s 分片上传对象存储失败。

```text
FFmpeg -> LocalDisk: 生成 chunk-001.m4s
Uploader -> LocalDisk: 发现新分片
Uploader -> S3: 上传 chunk-001.m4s
S3 --> Uploader: 网络超时/5xx
Uploader -> MySQL: 更新 t_transcode_segment(upload_status=FAILED, retry_count+1)
Uploader -> RetryQueue: 投递该分片重试任务
RetryWorker -> S3: 再次上传该分片
S3 --> RetryWorker: 成功
RetryWorker -> MySQL: 更新 t_transcode_segment(upload_status=UPLOADED, etag=...)
RetryWorker -> JobProgress: 推进任务进度
```

### 17.3.2 升级策略
- 单分片失败：先分片级重试
- 同一任务连续多分片失败：
  - 标记任务为上传异常
  - 进入失败队列
- 同一节点大面积上传失败：
  - 提升节点失败率
  - 必要时自动隔离节点

---

## 19. 工程落地附录（可直接开工开发）

### 19.1 所有表对应的建索引建议说明

> 说明：DDL 中已给出核心索引，这里给出“为什么 + 额外建议”。

#### 19.1.1 任务相关表

- `t_transcode_job`
  - `uk_request_id(request_id)`：幂等与快速定位任务
  - `idx_status_created(status, created_at)`：后台列表、调度扫描
  - `idx_node_status(assigned_node_id, status)`：按节点查看运行任务
  - `idx_lease_expire(lease_expire_at)`：租约过期接管扫描
  - 建议追加：`idx_request_id_created(request_id, created_at)`（如常按 request_id+时间过滤）

- `t_transcode_rendition`
  - `idx_job_status(job_id, status)`：任务详情页按状态聚合

- `t_transcode_segment`
  - 仅用于**点播分片**持久化，不用于直播分片
  - `uk_rendition_media_seq_init(...)`：分片幂等写入/更新
  - `idx_upload_status(upload_status, updated_at)`：上传失败重试扫描
  - 建议追加：`idx_job_media(job_id, media_type, is_init_segment, sequence_no)`（按任务与轨道顺序快速取分片元数据）

- `t_transcode_failure_queue`
  - `idx_status_next_retry(status, next_retry_at)`：失败队列重试扫描

#### 19.1.2 配置与审计

- `t_config_version`
  - `idx_admin_id(changed_by_admin_id)`：追踪谁改了配置

- `t_audit_log`
  - `idx_action_time(action, created_at)`：按动作查
  - `idx_resource(resource_type, resource_id)`：按资源查

#### 19.1.3 节点能力与集群指标

- `t_worker_codec_capability`
  - `uk_node_gpu_codec_type(node_id, gpu_index, codec_name, cap_type, hw_type)`：能力去重
  - `idx_node_type(node_id, cap_type)`：按节点快速取“支持哪些硬编/硬解”
  - `idx_codec_type(codec_name, cap_type)`：调度器按 codec 找节点能力时加速

- `t_node_metrics_realtime`
  - `idx_node_time(node_id, collected_at)`：按节点时间序列取数据
  - 建议：指标量大时按天分区（见 19.2）

### 19.2 表分区/归档策略

> 原则：高频写入、无限增长的“时序/日志/分片明细”必须归档，否则 MySQL 会被拖垮。

- `t_worker_codec_capability`
  - 建议只保留最新能力快照（每节点每GPU每codec每类型一行），历史变更可保留最近 N 次或写审计

- `t_node_metrics_realtime`
  - 建议按 `collected_at` 做 **RANGE 分区（按天/按月）**
  - 只保留最近 N 天热数据（如 7/30 天），更久数据落到冷库（ClickHouse/ES/对象存储）

- `t_audit_log`
  - 建议按 `created_at` 分区或按月归档

- `t_transcode_segment`
  - 若单视频分片量巨大（长视频+多清晰度），建议：
    - 按 `job_id` 水平拆分（逻辑分库分表），或
    - 按 `created_at` 做分区
  - 也可对“完成任务”的分片做归档到冷表，播放网关查不到再回源冷库（视业务访问模式决定）

- `t_event_delivery_attempt`
  - 建议只保留 7~30 天，超过归档

### 19.3 Redis Key 设计

> 目的：配置缓存、限流、实时进度与集群状态都要走 Redis（高性能）。

命名约定：`{app}:{env}:{module}:{key}`，示例以 `lts:prod` 为前缀。

- 配置缓存
  - `lts:prod:cfg:current_version` → 当前生效版本号
  - `lts:prod:cfg:version:{ver}:runtime` → runtime 配置
  - `lts:prod:cfg:version:{ver}:segmenting_vod`
  - `lts:prod:cfg:version:{ver}:naming_template`

- 节点能力缓存
  - `lts:prod:node:{node_id}:codec_caps` → 节点支持的硬解/硬编 codec 集合
  - `lts:prod:node:{node_id}:gpu:{gpu_index}:sessions` → GPU 当前会话数

- 任务实时状态（写 Redis、定期落库）
  - `lts:prod:job:{job_id}:progress`（hash）
  - `lts:prod:job:{job_id}:stage`
  - `lts:prod:job:{job_id}:node`

- 直播短期状态缓存（不持久化）
  - `lts:prod:live:{channel_id}:status` → 当前频道状态
  - `lts:prod:live:{channel_id}:segments:{rendition_name}` → 各清晰度最近分片窗口（list/zset，短TTL）
  - `lts:prod:live:{channel_id}:bitrate:{rendition_name}` → 当前码率
  - `lts:prod:live:{channel_id}:fps:{rendition_name}` → 当前帧率
  - `lts:prod:live:{channel_id}:view` → 当前实时展示聚合数据
  - `lts:prod:live:{channel_id}:source_mode` → 原画档当前实际模式（passthrough/transcode）

- 队列深度与统计
  - `lts:prod:metric:queue:job_create_depth`
  - `lts:prod:metric:queue:failure_depth`
  - `lts:prod:metric:s3:upload_mbps`

- 分布式锁/租约
  - `lts:prod:lock:scheduler:leader`
  - `lts:prod:lease:job:{job_id}`

### 19.4 MQ topic / queue / routing key 命名规范

当前实现约定：
- RabbitMQ 为当前跨平台 MQ 主实现，统一通过 `amqp091-go` 接入
- create-job 入站消费使用 queue 轮询拉取（`basic.get`），并保持“取到后立即 ack”的既有语义
- outbox 的 MQ 回调发布要求 broker confirm ack 后才视为成功
- MQ 回调发布使用 `mandatory=1`；若 broker 返回 `basic.return`（例如 routing key 无匹配队列），则按失败处理
- 动态 callback config 的 MQ 通道显式使用 `mq_exchange + mq_routing_key`
- legacy runtime config 回退路径继续使用默认 exchange + `callbackMqTopic`

统一规范：`lts.{env}.{domain}.{action}.v1`

- Kafka topic 示例
  - `lts.prod.transcode.job.create.v1`
  - `lts.prod.transcode.job.completed.v1`
  - `lts.prod.transcode.job.failed.v1`

- RabbitMQ exchange + routing key
  - 动态 callback config：显式使用 `mq_exchange` + `mq_routing_key`
  - legacy runtime config 回退：默认 exchange + `callbackMqTopic`
  - exchange：`lts.prod.transcode.v1`
  - routing_key：
    - `job.create`
    - `job.completed`
    - `job.failed`
  - queue：
    - `lts.prod.transcode.job.create.q`

### 19.5 Go 项目目录结构设计

建议采用 Go 标准项目布局，把"控制面"和"数据面"拆成多个可执行程序（进程隔离更稳）：

```
hvc/
├── cmd/
│   ├── api/            # API 服务器入口
│   ├── scheduler/      # 调度器入口
│   └── worker/         # Worker 入口
├── internal/
│   ├── common/         # 通用工具（日志、配置、时间、字符串、错误码）
│   │   ├── db/         # MySQL DAO 基础（GORM）
│   │   ├── redis/      # Redis client 封装（go-redis）
│   │   ├── mq/         # RabbitMQ 封装（amqp091-go）/ Kafka（confluent-kafka-go）/ Redis Streams
│   │   ├── rpc/        # gRPC client/server 封装
│   │   ├── s3/         # S3 client 封装（minio-go，连接池、multipart）
│   │   └── ffmpeg/     # ffprobe/ffmpeg 命令构建器与进度解析
│   ├── api/            # 对外 HTTP/gRPC + 后台业务接口 + WS
│   │   ├── handler/    # HTTP handler（Gin）
│   │   ├── service/    # 业务逻辑
│   │   └── dao/        # 数据库读写
│   ├── scheduler/      # 调度器核心逻辑
│   │   ├── lease/      # 租约管理
│   │   ├── scoring/    # 节点评分
│   │   └── dispatch/   # 任务派发
│   ├── worker/         # Worker 核心逻辑
│   │   ├── fetcher/    # 源视频下载
│   │   ├── executor/   # FFmpeg 命令执行与进度解析
│   │   ├── uploader/   # S3 上传
│   │   ├── watcher/    # 分片文件监听（fsnotify）
│   │   └── reporter/     # 进度上报
│   ├── live/             # 直播编排、原画档策略判定、推流地址生成、分发层对接
│   └── cluster/          # 集群协调、节点通信、热路径状态同步
├── pkg/                  # 可被外部引用的公共包
├── api/proto/            # gRPC Proto 定义
├── configs/              # 配置文件模板
├── migrations/           # 数据库迁移脚本
├── go.mod
└── go.sum
```

建议把“控制面”和“数据面”拆成多个可执行程序（进程隔离更稳）：
### 19.6 Gin Handler / Service / DAO 分层设计

- Handler（Gin handler）：只做参数解析、鉴权、调用 Service、返回响应
- Service：业务逻辑（幂等、调度入队、配置快照、权限校验）
- DAO（GORM）：纯数据库读写

推荐约定：
- `*Handler` 不直接写 SQL
- `*Service` 不直接拼 SQL
- `*Dao` 不做业务判断

### 19.7 分片元数据服务边界

- `t_transcode_segment` 仅用于保存点播 init/media 分片对象元数据。
- 本系统不提供 MPD、M3U8、master playlist、media playlist 的生成接口。
- 若播放体系需要清单文件，应由外部播放网关、CDN、边缘服务或业务侧专用播放服务处理。
- 因此本仓库中的服务拆分不再包含独立 `play-gateway` 进程。


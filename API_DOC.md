# HVC 视频转码服务 - 完整接口文档

> 版本：2.0.0 | 更新日期：2026-05-12

---

## 目录

- [一、系统概述](#一系统概述)
- [二、功能模块总览](#二功能模块总览)
- [三、HTTP 公共接口](#三http-公共接口)
- [四、HTTP 后台管理接口](#四http-后台管理接口)
- [五、HTTP 集群内部接口](#五http-集群内部接口)
- [六、gRPC 公共接口](#六grpc-公共接口)
- [七、gRPC 集群内部接口](#七grpc-集群内部接口)
- [八、消息队列接口](#八消息队列接口)
- [九、WebSocket 接口](#九websocket-接口)
- [十、回调载荷格式](#十回调载荷格式)
- [十一、分片命名模板](#十一分片命名模板)
- [十二、数据模型](#十二数据模型)

---

## 一、系统概述

HVC（High-performance Video Cloud）是一个分布式视频转码平台，支持：

- **点播转码**：提交源视频，自动生成多清晰度 CMAF 分片
- **直播转码**：推流鉴权、会话管理、动态清单生成
- **集群调度**：多节点负载均衡、GPU 能力匹配、故障自动转移
- **硬件加速**：NVIDIA/QSV/AMF/VAAPI/VideoToolbox 硬件编解码，软件降级
- **动态配置**：后台管理界面实时修改命名模板、存储类型、回调地址
- **版权保护**：清单接口支持清晰度过滤，按用户权限返回不同画质

---

## 二、功能模块总览

| 模块 | 功能 | 入口 |
|------|------|------|
| 转码任务 | 创建/查询/取消/重试 | HTTP + gRPC + MQ |
| 进度查询 | 实时帧率/码率/速度/千分比 | HTTP + gRPC + WebSocket |
| 清单生成 | 动态 MPD/m3u8（支持清晰度过滤） | HTTP |
| 回调通知 | 转码完成/失败回调（HTTP/gRPC/MQ） | 后台配置驱动 |
| 命名模板 | 6种预置方案 + 自定义模板 | 后台管理 |
| 存储管理 | S3 对象存储 + 本地存储动态切换 | 后台配置 |
| 集群管理 | 节点注册/心跳/隔离/排空/启用 | 后台管理 + gRPC |
| 直播管理 | 频道创建/推流鉴权/播放令牌 | HTTP |
| RBAC 权限 | 角色/权限/用户管理 | 后台管理 |
| 配置中心 | Bootstrap 基础配置源绑定（MySQL/集群/ID 等） | 后台管理 |
| 审计日志 | 管理员操作记录 | 后台管理 |

---

## 三、HTTP 公共接口

基础地址：`http://localhost:8080`

### 3.1 健康检查

```
GET /healthz
```

**响应体：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "name": "hili-video-cloud",
    "status": "ok",
    "mode": "standalone"
  }
}
```

### 3.2 创建转码任务

```
POST /v1/transcode/job/create
Content-Type: application/json
```

**请求体：**

```json
{
  "request_id": "req-001",
  "biz_key": "biz-video-001",
  "source_url": "https://example.com/video.mp4",
  "profile_id": 0,
  "priority": 5,
  "enable_watermark": false,
  "watermark": {
    "image_url": "https://example.com/wm.png",
    "anchor": 4,
    "x_ratio": 0.05,
    "y_ratio": 0.05,
    "width_ratio": 0.15,
    "opacity": 0.8,
    "safe_margin_ratio": 0.02
  },
  "segment_options": {
    "segment_duration_sec": 6,
    "support_dash": true,
    "support_hls": true
  },
  "thumbnail_options": {
    "enable_sprite": true,
    "sprite_rows": 5,
    "sprite_cols": 10,
    "thumb_interval_sec": 10,
    "thumb_width": 160,
    "thumb_height": 90,
    "sprite_image_format": "jpg",
    "sprite_storage_prefix": "hvc/thumbnails",
    "enable_binary_index": true,
    "binary_storage_prefix": "hvc/thumbnails/bin",
    "binary_max_size_bytes": 1048576
  },
  "storage_options": {
    "bucket_prefix": "hvc"
  },
  "schedule_options": {
    "preferred_hwaccel": "nvidia"
  },
  "renditions": [
    {
      "name": "1080p",
      "width": 0,
      "height": 1080,
      "video_codec": "",
      "video_bitrate_kbps": 5000,
      "video_maxrate_kbps": 5500,
      "video_bufsize_kbps": 10000,
      "preset": "p4"
    }
  ]
}
```

| 字段 | 类型 | 必填 | 说明 |
|------|------|------|------|
| request_id | string | ✅ | 幂等控制，重复提交返回已有任务 |
| biz_key | string | ❌ | 业务关联键 |
| source_url | string | ✅ | 源视频地址（HTTP/HTTPS/S3/本地路径） |
| profile_id | uint64 | ❌ | 转码模板ID（空=默认编码阶梯） |
| priority | int | ❌ | 优先级，数值越大越优先 |
| enable_watermark | bool | ❌ | 是否启用水印 |
| watermark | object | ❌ | 水印配置（enable_watermark=true时必填） |
| video_options | object | ❌ | 视频输出选项 |
| segment_options | object | ❌ | 分片配置 |
| thumbnail_options | object | ❌ | 缩略图/雪碧图配置 |
| storage_options | object | ❌ | 存储配置 |
| schedule_options | object | ❌ | 调度配置 |
| callback_url | string | ❌ | 单任务回调目标，优先于系统级回调配置，支持 `https://...`、`grpc://host:port/pkg.Service/Method`、`mq://exchange/routing.key` |
| renditions | array | ❌ | 自定义清晰度列表（空=默认阶梯） |

`callback_url` 示例：
- `https://callback.example.com/task/req-001`
- `grpc://127.0.0.1:9000/transcode.callback.Service/Notify`
- `mq://callback.exchange/transcode.job.completed`

> 当前版本字段生效范围说明：
> - `video_options` 暂未落入执行链路，服务端收到非空配置会直接拒绝请求；
> - `segment_options.naming_template_id` 暂未开放单任务覆盖，命名模板仍通过后台全局配置生效；
> - `storage_options.storage_id`、`storage_options.segment_prefix` 暂未开放单任务覆盖；
> - `schedule_options.allow_software_decode_fallback`、`schedule_options.max_wait_seconds` 暂未开放单任务覆盖；
> - 以上字段会返回 `400`，请不要在生产调用中依赖。

**响应体：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "job_id": 1893456789012345678,
    "request_id": "req-001",
    "status": 1,
    "status_name": "CREATED"
  }
}
```

### 3.3 查询转码进度

```
GET /v1/transcode/job/progress?request_id=req-001
```

**响应体：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "job_id": 1893456789012345678,
    "status": 4,
    "stage": "TRANSCODING",
    "progress_permille": 450,
    "current_fps": 120.5,
    "current_bitrate_kbps": 4800.0,
    "current_speed": 4.02,
    "elapsed_ms": 3600000,
    "estimated_remaining_ms": 4400000,
    "updated_at": "2026-05-07T10:30:00Z"
  }
}
```

### 3.4 动态构建 DASH MPD

```
GET /v1/manifest/dash/{job_id}
GET /v1/manifest/dash/{job_id}?renditions=720p,480p
GET /v1/manifest/dash/{job_id}?max_height=720
```

| 参数 | 位置 | 说明 |
|------|------|------|
| job_id | path | 任务ID |
| renditions | query | 清晰度白名单（逗号分隔），版权保护用 |
| max_height | query | 最大允许高度，版权保护用 |

**响应：** `Content-Type: application/dash+xml`，`Cache-Control: public, max-age=5`

### 3.5 动态构建 HLS Master

```
GET /v1/manifest/hls/{job_id}
GET /v1/manifest/hls/{job_id}?renditions=720p,480p
GET /v1/manifest/hls/{job_id}?max_height=720
```

**响应：** `Content-Type: application/vnd.apple.mpegurl`，`Cache-Control: public, max-age=5`

### 3.6 动态构建 HLS Variant

```
GET /v1/manifest/hls/{job_id}/{rendition}
```

**响应：** `Content-Type: application/vnd.apple.mpegurl`，`Cache-Control: public, max-age=5`

### 3.7 查询直播播放信息

```
GET /v1/live/channel/playback?channel_key=live-001
```

**响应体：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "channel_key": "live-001",
    "status": "RUNNING",
    "play_token": "signed-token",
    "expire_at": 1893456000,
    "master_hls_url": "https://cdn.example.com/live/live-001.m3u8",
    "http_flv_url": "https://cdn.example.com/live/live-001.flv",
    "rendition_names": ["1080p", "720p", "480p"],
    "renditions": [
      {
        "rendition_name": "1080p",
        "is_source": false,
        "hls_url": "https://cdn.example.com/live/live-001/1080p.m3u8",
        "http_flv_url": "https://cdn.example.com/live/live-001_1080p.flv"
      }
    ]
  }
}
```

说明补充：
- 当频道不存在时，接口返回 `404`
- 当前返回结果已包含 `play_token`、`expire_at`、签名后的 `master_hls_url` / `http_flv_url`
- `renditions` 字段会给出每个清晰度的独立 HLS / HTTP-FLV 播放地址

### 3.8 直播推流鉴权

```
GET /v1/live/channel/push-auth?channel_key=live-001&expire=1700000000&sign=xxxx
```

用于对接推流入口鉴权回调。返回字段包括 `allowed`、`reason`、`channel_key`、`expire_at`。

### 3.9 直播播放鉴权

```
GET /v1/live/channel/play-auth?channel_key=live-001&expire=1700000000&sign=xxxx
```

用于对接播放层鉴权回调。返回字段包括 `allowed`、`reason`、`channel_key`、`expire_at`。

---

## 四、HTTP 后台管理接口

> 所有后台接口需要会话认证（Cookie: admin_session={session_token} 或 Authorization: Bearer {session_token}）和 RBAC 权限校验。

### 4.1 认证

| 接口 | 方法 | 说明 |
|------|------|------|
| `/v1/admin/auth/login` | POST | 管理员登录（表单提交），通过 HttpOnly Cookie 建立后台会话 |
| `/v1/admin/auth/logout` | POST | 管理员登出（需认证） |
| `/v1/admin/auth/current-user` | GET | 获取当前管理员会话与菜单/权限数据（首选路径，需认证） |
| `/v1/admin/auth/profile` | GET | 获取当前管理员会话与菜单/权限数据（兼容推荐路径，需认证） |
| `/v1/admin/auth/session` | GET | 获取当前管理员会话（兼容别名，需认证） |
| `/v1/admin/auth/me` | GET | 获取当前管理员会话（兼容别名） |
| `/v1/admin/ping` | GET | 后台接口存活检查（需认证） |

**登录请求（表单提交）：**

```
POST /v1/admin/auth/login
Content-Type: application/x-www-form-urlencoded

username=admin&password=admin123&otp_code=
```

**登录响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "authenticated": true,
    "token_transport": {
      "type": "cookie",
      "cookie_name": "admin_session",
      "http_only": true
    }
  }
}
```

登录成功后自动设置 Cookie：`admin_session=<opaque_session_token>; Path=/; HttpOnly; SameSite=Lax; Expires=24h`

**WhoAmI 响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "authenticated": true,
    "admin_user_id": 1,
    "user": {
      "admin_user_id": 1,
      "username": "admin",
      "display_name": "系统管理员",
      "status": 1,
      "created_at": "2026-05-09T09:00:00+08:00",
      "updated_at": "2026-05-09T09:00:00+08:00"
    },
    "permission_keys": [
      "system.user.read",
      "system.menu.read"
    ],
    "tree": true,
    "items": [
      {
        "id": 100,
        "menu_id": 100,
        "parent_id": 0,
        "menu_key": "system",
        "menu_name": "系统管理",
        "route_path": "/system",
        "component": "Layout",
        "icon_name": "settings",
        "menu_type": "catalog",
        "permission_key": "",
        "sort_no": 10,
        "hidden": false,
        "status": 1,
        "children": []
      }
    ],
    "menu_tree": [
      {
        "id": 100,
        "menu_id": 100,
        "parent_id": 0,
        "menu_key": "system",
        "menu_name": "系统管理",
        "route_path": "/system",
        "component": "Layout",
        "icon_name": "settings",
        "menu_type": "catalog",
        "permission_key": "",
        "sort_no": 10,
        "hidden": false,
        "status": 1,
        "children": []
      }
    ]
  }
}
```

说明：

- `auth/session` 现以 `data.items + data.tree=true` 作为统一菜单树口径
- `menu_tree` 仍保留为兼容字段，方便旧前端平滑迁移
- 菜单树节点统一包含 `id`, `menu_id`, `parent_id`, `component` 等完整字段，前端可直接用 `id` 做唯一键

### 4.2 运行配置管理

说明：

- 运行期业务配置的唯一生效源是数据库中的已发布 `runtime config`
- 本地 `configs/config.yaml` 已收敛为 bootstrap 配置；首版 runtime config 使用程序内置默认值初始化
- `configs/config.yaml` 中的 `server.node_mode` 明确声明节点部署角色：`standalone / cluster-control / cluster-worker / cluster-allinone`
- `node_mode` 会对本节点最终生效的模块边界做硬约束：
  - `cluster-control` 不会被共享 runtime config 热改成执行节点
  - `cluster-worker` 不会被共享 runtime config 热改成调度节点，也不会在本机启动 `http/public gRPC/mq consumer/callback`
- 若启动配置里仍然写入 `scheduler/worker/callback/storage/grpc/mq` 段，服务会在启动校验阶段直接报错
- 外部“配置中心”绑定不直接下发运行时业务配置，也不参与运行期热更新

| 接口 | 方法 | 权限 | 说明 |
|------|------|------|------|
| `/v1/admin/config/runtime/versions` | GET | config.version.read | 查询配置版本列表 |
| `/v1/admin/config/runtime/server/update` | POST | config.runtime.update | 仅更新 HTTP/WebSocket 对外暴露开关（创建待发布版本） |
| `/v1/admin/config/runtime/scheduler/update` | POST | config.runtime.update | 仅更新调度策略相关运行配置（创建待发布版本） |
| `/v1/admin/config/runtime/worker/update` | POST | config.runtime.update | 仅更新 Worker 轮询/心跳/上传并发配置（创建待发布版本） |
| `/v1/admin/config/runtime/storage/update` | POST | config.runtime.update | 仅更新存储相关运行配置（创建待发布版本） |
| `/v1/admin/config/runtime/callback/update` | POST | config.runtime.update | 仅更新默认回调兜底配置（创建待发布版本） |
| `/v1/admin/config/runtime/mq/update` | POST | config.runtime.update | 仅更新 MQ 消费相关运行配置（创建待发布版本） |
| `/v1/admin/config/runtime/grpc/update` | POST | config.runtime.update | 仅更新对外 public gRPC 相关运行配置（创建待发布版本） |
| `/v1/admin/config/publish` | POST | config.version.publish | 发布配置版本 |

**配置版本列表查询参数：** `page`, `page_size`, `published`

**配置版本列表返回字段：**
- 分页统一为：`page`, `page_size`, `total`, `items`
- `items[*]` 关键字段包括：`config_version`, `published`, `enable_http_server`, `enable_grpc_server`, `public_grpc_registry_id`, `enable_mq_consumer`, `storage_type`, `change_summary`, `config_source`, `published_by`, `published_at`
- 出于安全考虑，`callback_rpc_endpoint`、`mq_password`、`storage_access_key_id`、`storage_secret_access_key` 在列表接口里返回的是脱敏值，不会回传明文

**按域更新运行配置说明：**

- 运行时配置对外只保留分域更新接口，不再暴露全量 `runtime/update`
- `server/update` 负责 HTTP 主入口与 WebSocket 监控入口热启停
- `scheduler/update` 负责调度总并发、租约 TTL、请求覆盖、硬件编码约束、节点安全阈值等
- `worker/update` 负责 Worker 心跳超时、轮询周期、单任务上传并发
- `storage/update` 负责存储与对象前缀
- `callback/update` 负责默认回调兜底开关与 `callback_*`
- `mq/update` 负责 MQ 创建任务消费者参数
- `grpc/update` 负责对外 public gRPC 开关、监听地址与 etcd 绑定
- 更新后均生成新的待发布版本，再通过 `/v1/admin/config/publish` 生效

**分域更新请求示例（以 gRPC 为例）：**

```json
{
  "enable_grpc_server": true,
  "grpc_listen_address": ":9090",
  "public_grpc_registry_id": 3001,
  "change_summary": "更新对外 gRPC 暴露"
}
```

发布后生效范围：

- `scheduler` / `worker` / `callback` 运行参数会按最新生效配置热更新
- `HTTP` 对外入口、`WebSocket` 监控入口跟随 `enable_http_server` 热启停；监听地址仍由 bootstrap `server.listen_address` 固定
- `callback` 投递模块支持通过 `enable_callback` 热启停
- 对外 `public gRPC` 服务支持动态启停与监听地址切换；若 `public_grpc_registry_id > 0`，服务启动后会把当前实例可访问地址注册到对应 etcd 命名空间，并在配置发布导致 registry 变化时自动重建注册
- `internal_grpc` 属于 bootstrap 配置，不参与运行期热更新，也不会被 `enable_grpc_server` 影响
- `MQ` 创建任务消费者支持动态启停与队列/连接参数切换；启用时必须同时提供 `mq_queue_name` 和 `mq_host`
- `cluster-worker` 节点即使读到全局开启的 runtime config，也会在本机被硬约束为不启动 `HTTP/public gRPC/MQ consumer/callback/scheduler`，只保留 worker 与 internal gRPC
- 配置发布后会先更新 MySQL，再刷新 Redis 缓存版本；所有节点通过 Redis 版本探测自动同步到最新已发布版本
- 已在运行中的转码任务不会被强制中断，但后续轮询、上传、回调和新任务派发会按新配置执行

说明补充：

- `/v1/admin/config/runtime/server|scheduler|worker|storage|callback|mq|grpc/update` 适合后台按模块拆分编辑，未传字段会继续沿用当前已发布版本。

**发布配置请求：**

```json
{
  "config_version": 1893456789012345678,
  "publish_reason": "发布新配置"
}
```

**发布配置响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "config_version": 1893456789012345678,
    "published": true,
    "effective_scope": {
      "new_jobs": true,
      "queued_jobs": true,
      "running_jobs": false
    }
  }
}
```

**更新运行配置成功响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "pending_config_version": 1893456789012345679,
    "published": false
  }
}
```

### 4.3 回调配置管理

| 接口 | 方法 | 权限 | 说明 |
|------|------|------|------|
| `/v1/admin/config/callback/list` | GET | config.callback.read | 查询回调配置列表 |
| `/v1/admin/config/callback/upsert` | POST | config.callback.update | 创建/更新回调配置 |
| `/v1/admin/config/callback/enabled` | POST | config.callback.update | 启用/禁用回调 |

**回调配置列表查询参数：** `page`, `page_size`, `callback_name`, `enabled`

**回调配置列表返回字段：**
- 分页统一为：`page`, `page_size`, `total`, `items`
- `items[*]` 关键字段包括：`callback_config_id`, `callback_name`, `callback_type`, `target_url`, `rpc_endpoint`, `rpc_service_name`, `mq_exchange`, `mq_routing_key`, `enabled`, `priority`

**回调配置请求：**

```json
{
  "callback_config_id": 0,
  "callback_name": "业务系统回调",
  "callback_type": 1,
  "target_url": "https://your-system.example.com/callback",
  "rpc_endpoint": "",
  "rpc_service_name": "",
  "mq_exchange": "",
  "mq_routing_key": "",
  "timeout_ms": 5000,
  "retry_times": 3,
  "enabled": true,
  "priority": 10,
  "registry_id": 0
}
```

| callback_type | 说明 |
|---------------|------|
| 1 | HTTP 回调（POST 到 target_url） |
| 2 | gRPC 回调（优先调用 rpc_endpoint；若为空则用 registry_id + rpc_service_name 走 etcd 发现） |
| 3 | MQ 回调（发送到 mq_exchange + mq_routing_key） |

说明：
- 系统级回调目标通过 `/v1/admin/config/callback/upsert` 和 `/v1/admin/config/callback/enabled` 保存后立即生效，无需发布运行时配置版本。
- 单任务 `callback_url` 优先级高于系统级回调配置。
- 系统级回调同时支持 HTTP、gRPC、MQ 三种形式；其中 gRPC 既支持 `rpc_endpoint + rpc_service_name` 固定地址，也支持 `registry_id + rpc_service_name` 通过 etcd 动态发现，MQ 使用 `mq_exchange + mq_routing_key`。
- runtime config 里的 `callback_http_url` / `callback_mq_topic` 仅保留兼容兜底语义；主配置入口仍然是 `callback_config` 表。
- 单次投递失败会写入补偿记录，记录中会落具体的 `callback_url` 实际目标值（字段名为 `callback_target`），便于排障和审计。

### 4.3.1 etcd 注册配置管理

| 接口 | 方法 | 权限 | 说明 |
|------|------|------|------|
| `/v1/admin/registry/etcd/list` | GET | config.version.read | 查询 etcd 注册配置分页列表 |
| `/v1/admin/registry/etcd/upsert` | POST | config.runtime.update | 创建/更新 etcd 注册配置 |
| `/v1/admin/registry/etcd/enabled` | POST | config.runtime.update | 启用/禁用 etcd 注册配置 |

用途说明：

- 对外 `public gRPC` 的 etcd 注册使用 `runtime config.public_grpc_registry_id` 绑定。
- gRPC 类型回调配置可使用 `callback_config.registry_id` 绑定。
- 服务注册 key 采用 `/{service_namespace}/{service_name}/{endpoint}` 形式；当前 public gRPC 服务名为 `transcode.v1.TranscodePublicService`。

**etcd 注册配置列表查询参数：** `page`, `page_size`, `registry_name`, `enabled`

**etcd 注册配置列表返回字段：**
- 分页统一为：`page`, `page_size`, `total`, `items`
- `items[*]` 关键字段包括：`registry_id`, `registry_name`, `endpoints`, `service_namespace`, `lease_ttl_sec`, `dial_timeout_ms`, `enabled`, `priority`, `created_at`, `updated_at`

**回调配置写入成功响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "callback_config_id": 1001
  }
}
```

### 4.4 命名模板管理

| 接口 | 方法 | 权限 | 说明 |
|------|------|------|------|
| `/v1/admin/config/naming-template/list` | GET | config.naming_template.read | 分页查询6种预置模板+当前生效模板 |
| `/v1/admin/config/naming-template/configure` | POST | config.naming_template.update | 配置模板（待发布） |
| `/v1/admin/config/naming-template/activate` | POST | config.naming_template.update | 立即生效模板，并发布新的 runtime config 版本 |

说明：

- 该接口虽然总共只有 6 条预置方案，但返回结构已统一为分页格式：`page/page_size/total/items`
- `current_template` 单独返回当前运行时生效模板
- `saved_templates` 会额外返回最近保存过的模板记录，字段包括：`id`, `config_version`, `output_base_tpl`, `init_seg_name_tpl`, `media_seg_name_tpl`, `created_at`

**配置模板请求：**

```json
{
  "template_id": 2,
  "custom_template": ""
}
```

| template_id | 模板 | 示例（video init） |
|-------------|------|-------------------|
| 1 | `{job_id}-{rendition_key}-{media_type}-{number}.m4s` | `1893456789012345678-Ab3kP9xQ-video-0.m4s` |
| 2 | `{job_id}-{resolution}-{rendition_key}-{media_type}-{number}.m4s` | `1893456789012345678-1920_1080-Ab3kP9xQ-video-0.m4s` |
| 3 | `{job_id}-{quality}-{rendition_key}-{media_type}-{number}.m4s` | `1893456789012345678-1080p-Ab3kP9xQ-video-0.m4s` |
| 4 | `{job_id}-{rendition_key}-{media_type}-{number}-{timestamp}.m4s` | `1893456789012345678-Ab3kP9xQ-video-0-0.m4s` |
| 5 | `{job_id}-{resolution}-{rendition_key}-{media_type}-{number}-{timestamp}.m4s` | `1893456789012345678-1920_1080-Ab3kP9xQ-video-0-0.m4s` |
| 6 | `{job_id}-{quality}-{rendition_key}-{media_type}-{number}-{timestamp}.m4s` | `1893456789012345678-1080p-Ab3kP9xQ-video-0-0.m4s` |

**立即生效请求：**

```json
{
  "template": "{job_id}-{resolution}-{rendition_key}-{media_type}-{number}.m4s"
}
```

**立即生效响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "template": "{job_id}-{resolution}-{rendition_key}-{media_type}-{number}.m4s",
    "config_version": 1893456789012345678,
    "published": true,
    "effective_scope": "新提交的转码任务",
    "running_jobs": "不受影响，继续使用原模板"
  }
}
```

说明：

- `activate` 不再只是修改当前节点内存；现在会生成并发布新的 runtime config 版本
- 已发布版本会先写 MySQL，再刷新 Redis，最后由各节点通过版本同步自动收敛
- 已经开始执行的任务仍沿用各自首次锁定的 `segment_template`，避免重试和清单生成漂移

### 4.5 配置中心绑定（Bootstrap 基础配置源）

| 接口 | 方法 | 权限 | 说明 |
|------|------|------|------|
| `/v1/admin/config-center/list` | GET | config.version.read | 查询 bootstrap 配置源绑定列表 |
| `/v1/admin/config-center/upsert` | POST | config.version.publish | 创建/更新 bootstrap 配置源绑定 |
| `/v1/admin/config-center/enabled` | POST | config.version.publish | 启用/禁用 bootstrap 配置源绑定 |

**配置中心绑定列表查询参数：** `page`, `page_size`, `provider_type`, `enabled`

**配置中心绑定列表返回字段：**
- 顶层除分页字段外，还会返回：`config_scope`, `affects_runtime`, `runtime_update_paths`, `runtime_publish_path`
- `items[*]` 关键字段包括：`binding_id`, `binding_name`, `provider_type`, `endpoint`, `namespace`, `auth_mode`, `enabled`, `priority`, `last_sync_status`, `last_sync_at`, `binding_usage`
- `access_key`、`secret_key`、`token` 在列表接口中同样只返回脱敏值，便于确认是否已配置，但不泄露明文

说明：

- 该组接口用于维护外部 bootstrap 配置源元数据，例如 MySQL/集群注册/节点身份这类启动基础配置来源
- 该组接口不直接发布 `scheduler/worker/callback/http/grpc/mq` 等运行期业务配置
- 本地 `configs/config.yaml` 同样不允许再携带这些业务动态段，误写会导致启动失败
- 运行期业务配置通过分域接口 `/v1/admin/config/runtime/server|scheduler|worker|storage|callback|mq|grpc/update` + `/v1/admin/config/publish` 生效
- `list`/`upsert`/`enabled` 返回都会显式带上 `config_scope=bootstrap` 与 `affects_runtime=false`

**配置中心绑定写入成功响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "binding_id": 2001,
    "config_scope": "bootstrap",
    "affects_runtime": false
  }
}
```

**配置中心绑定请求：**

```json
{
  "binding_id": 0,
  "binding_name": "生产环境配置",
  "provider_type": "nacos",
  "endpoint": "http://nacos:8848",
  "namespace": "production",
  "auth_mode": "token",
  "access_key": "",
  "secret_key": "",
  "token": "",
  "enabled": true,
  "priority": 10
}
```

### 4.6 RBAC 权限管理

| 接口 | 方法 | 权限 | 说明 |
|------|------|------|------|
| `/v1/admin/system/user/list` | GET | system.user.read | 用户分页列表 |
| `/v1/admin/system/user/upsert` | POST | system.user.create | 创建/更新用户 |
| `/v1/admin/system/user/status` | POST | system.user.update | 启用/禁用用户 |
| `/v1/admin/system/role/list` | GET | system.role.read | 角色分页列表 |
| `/v1/admin/system/role/all` | GET | system.role.read | 全部角色列表（不分页） |
| `/v1/admin/system/role/upsert` | POST | system.role.update | 创建/更新角色 |
| `/v1/admin/system/permission/list` | GET | system.permission.read | 权限树（兼容别名） |
| `/v1/admin/system/permission/tree` | GET | system.permission.read | 权限树 |
| `/v1/admin/system/permission/upsert` | POST | system.role.permission_bind | 创建/更新权限 |
| `/v1/admin/system/menu/tree` | GET | system.menu.read | 全量菜单树 |
| `/v1/admin/system/menu/current-tree` | GET | auth.session.read | 当前登录管理员可见菜单树 |
| `/v1/admin/system/role/menu/tree` | GET | system.menu.read | 角色菜单树与已选菜单 ID |
| `/v1/admin/system/menu/upsert` | POST | system.menu.update | 创建/更新菜单 |
| `/v1/admin/system/menu/delete` | POST | system.menu.delete | 删除菜单 |
| `/v1/admin/system/user-role/bind` | POST | system.user.role_bind | 绑定用户角色 |
| `/v1/admin/system/role-permission/bind` | POST | system.role.permission_bind | 绑定角色权限 |
| `/v1/admin/system/role-menu/assign` | POST | system.role.menu_bind | 覆盖分配角色菜单 |

说明：

- `user/list`、`role/list`、`audit/list` 为分页接口
- 权限和菜单改为树形返回，不分页，统一放在 `data.items`
- `role/all` 用于下拉选择器，返回全部角色，统一放在 `data.items`
- `role/menu/tree` 同样统一返回 `data.items`，并通过 `role_id`、`menu_ids` 标识当前角色上下文
- 树接口和非分页全量列表接口统一返回 `data.items`；树接口额外返回 `data.tree=true`
- 用户列表、登录态用户信息、用户写接口响应均已屏蔽 `password_hash` 与 `password_salt`
- 创建/更新管理员用户接口统一接收明文 `password`，由后端负责做哈希；`password_hash` 仅保留旧客户端兼容，不再推荐使用
- `role/upsert`、`permission/upsert`、`menu/upsert` 不再直接返回数据库结构体，而是统一返回 snake_case 视图对象

**用户列表查询参数：** `page`, `page_size`, `username`, `status`

**角色列表查询参数：** `page`, `page_size`, `role_key`, `status`

**用户列表返回字段：**
- `items[*]` 关键字段包括：`admin_user_id`, `username`, `display_name`, `status`, `last_login_at`, `last_login_ip`, `created_at`, `updated_at`

**角色列表返回字段：**
- `items[*]` 关键字段包括：`role_id`, `role_key`, `role_name`, `role_desc`, `status`, `created_at`, `updated_at`

**创建/更新用户请求：**

```json
{
  "username": "operator",
  "password": "admin123",
  "display_name": "运维人员",
  "status": 1
}
```

- 新建管理员时 `password` 必填
- 更新管理员时不传 `password` 表示保留原密码

**权限树返回说明：**

- `data.items[*]` 统一包含 `id`, `parent_id`, `perm_id`, `key`, `label`, `node_type`, `module`, `children`
- 目录节点（`module` / `group`）主要用于树结构组织；叶子权限节点额外携带真实 `perm_id`

**菜单树返回说明：**

- `/v1/admin/system/menu/tree`、`/v1/admin/system/menu/current-tree`、`/v1/admin/system/role/menu/tree` 的 `data.items[*]` 统一包含 `id`, `menu_id`, `parent_id`, `menu_key`, `menu_name`, `route_path`, `component`, `icon_name`, `menu_type`, `permission_key`, `sort_no`, `hidden`, `status`, `children`
- `component` 用于保存前端组件路径；`component_name` 仅作为旧客户端兼容入参，后续应统一只传 `component`

**启用/禁用用户请求：**

```json
{
  "admin_user_id": 2,
  "status": 2
}
```

**创建/更新角色请求：**

```json
{
  "role_key": "operator",
  "role_name": "运维人员",
  "role_desc": "负责日常运维操作",
  "status": 1
}
```

**创建/更新权限请求：**

```json
{
  "perm_key": "transcode.job.read",
  "perm_name": "查看转码任务",
  "perm_desc": "允许查看转码任务列表和详情",
  "module": "transcode"
}
```

**绑定用户角色请求：**

```json
{
  "admin_user_id": 2,
  "role_id": 2
}
```

**绑定角色权限请求：**

```json
{
  "role_id": 2,
  "perm_id": 3
}
```

**菜单创建/更新请求：**

```json
{
  "menu_id": 0,
  "parent_id": 100,
  "menu_key": "system.user",
  "menu_name": "用户管理",
  "route_path": "/system/user",
  "component": "system/user/index",
  "icon_name": "users",
  "menu_type": "menu",
  "permission_key": "system.user.read",
  "sort_no": 20,
  "hidden": false,
  "status": 1
}
```

**角色菜单分配请求：**

```json
{
  "role_id": 2,
  "menu_ids": [101, 102, 103]
}
```

**用户写入成功响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "admin_user_id": 2,
    "username": "operator",
    "display_name": "运维人员",
    "status": 1,
    "created_at": "2026-05-09T10:00:00+08:00",
    "updated_at": "2026-05-09T10:00:00+08:00"
  }
}
```

**角色写入成功响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "role_id": 2,
    "role_key": "operator",
    "role_name": "运维人员",
    "role_desc": "负责日常运维操作",
    "status": 1,
    "created_at": "2026-05-09T10:00:00+08:00",
    "updated_at": "2026-05-09T10:00:00+08:00"
  }
}
```

**权限写入成功响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "perm_id": 3,
    "perm_key": "transcode.job.read",
    "perm_name": "查看转码任务",
    "perm_desc": "允许查看转码任务列表和详情",
    "module": "transcode",
    "created_at": "2026-05-09T10:00:00+08:00"
  }
}
```

**菜单写入成功响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "id": 101,
    "menu_id": 101,
    "parent_id": 100,
    "menu_key": "system.user",
    "menu_name": "用户管理",
    "route_path": "/system/user",
    "component": "system/user/index",
    "icon_name": "users",
    "menu_type": "menu",
    "permission_key": "system.user.read",
    "sort_no": 20,
    "hidden": false,
    "status": 1
  }
}
```

**角色菜单分配成功响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "role_id": 2,
    "menu_ids": [101, 102, 103]
  }
}
```

### 4.7 集群节点管理

| 接口 | 方法 | 权限 | 说明 |
|------|------|------|------|
| `/v1/admin/cluster/node/list` | GET | cluster.node.read | 节点分页列表 |
| `/v1/admin/cluster/node/detail` | GET | cluster.node.read | 节点详情（Query） |
| `/v1/admin/cluster/node/metrics` | GET | cluster.node.metrics.read | 节点指标分页列表 |
| `/v1/admin/cluster/overview` | GET | cluster.read | 集群运行总览 |
| `/v1/admin/cluster/realtime` | GET | cluster.read | 集群实时快照 |
| `/v1/admin/cluster/topology` | GET | cluster.read | 集群拓扑与控制面摘要 |
| `/v1/admin/cluster/resource/distribution` | GET | cluster.read | 节点资源承载分布 |
| `/v1/admin/cluster/member/list` | GET | cluster.read | 集群成员分页列表 |
| `/v1/admin/cluster/worker/list` | GET | cluster.read | Worker 实例分页列表 |
| `/v1/admin/cluster/scheduler/insight` | GET | cluster.read | 调度洞察 |
| `/v1/admin/cluster/worker/offline` | POST | cluster.worker.offline | 手动下线 Worker，优先远程下发到目标节点 |
| `/v1/admin/cluster/worker/exit` | POST | cluster.worker.exit | 手动标记 Worker 退出，优先远程下发到目标节点 |
| `/v1/admin/cluster/job/takeover` | POST | cluster.job.takeover | 强制接管节点/Worker上的活跃任务 |
| `/v1/admin/cluster/node/enabled` | POST | cluster.node.enable | 启用/禁用节点 |
| `/v1/admin/cluster/node/quarantined` | POST | cluster.node.quarantine | 隔离/取消隔离节点 |
| `/v1/admin/cluster/node/draining` | POST | cluster.node.drain | 排空/恢复节点 |

`/v1/admin/cluster/overview` 返回重点包括：

- 当前运行模式：`standalone / cluster-control / cluster-worker / cluster-allinone`
- `node_mode`：节点 bootstrap 部署角色，稳定不因运行时模块热停而漂移
- `mode`：当前激活模块组合摘要，用于观察运行态，不用于表达部署角色
- 集群拓扑：当前节点角色、当前节点是否控制面、控制面节点总数、可访问后台节点总数、worker-only 节点总数、控制面节点列表
- 集群拓扑还会给出推荐后台入口：`recommended_admin_node_id`、`recommended_admin_base_url`
- 模块状态：HTTP、public gRPC、internal gRPC、MQ consumer、scheduler、worker、callback
- public gRPC 观测：当前启停状态、运行时监听地址、是否绑定 etcd、绑定的 registry_id、service_namespace、租约 TTL、dial timeout
- public gRPC 观测还会返回：`registry_key`、`registry_key_exists`、`registry_value_json`、`registry_lease_id`、`registry_lease_ttl_sec`，用于直接判断当前实例应该注册的 etcd key 是否已经落进去，以及当前 lease 是否还活着
- `governance`：最近 24 小时的集群治理动作摘要，除 `recent_action_total`、`latest_action` 和 `recent_actions` 外，还会返回启用/停用、隔离/解除隔离、排空/恢复调度、worker 下线/退出、任务接管等动作计数
- 调度器状态：`max_global_transcode_sessions`、`active_execution_total`、`remaining_execution_capacity`、`worker_heartbeat_timeout_sec`
- 集群统计：节点总数、启用节点数、隔离节点数、排空节点数、在线节点数、成员数、GPU 总数、可调度 GPU 数、活动会话数、上传队列深度
- Worker 统计：`worker_total`、`worker_online_total`
- 节点摘要：每个节点的启用/隔离/在线状态、在线判定来源、是否可调度、不可调度原因、GPU 总数、可调度 GPU 数、活动转码会话、上传队列深度
- 版本信息：MySQL 服务端版本、Redis 服务端版本、Redis 模式、运行时配置数据库版本、运行时配置 Redis 缓存版本、缓存 TTL
- 在线判定口径：优先看 worker metrics，其次看 `t_cluster_node.last_heartbeat_at`，控制面节点还会额外参考 registry 心跳，因此不跑 worker 的 control-plane 节点也不会被误判离线

`/v1/admin/cluster/realtime` 适合后台轮询，除 `overview` 的聚合字段外，还直接返回：

- 任务队列计数：`queued / assigned / running / uploading / completed / failed / canceled`
- `nodes` 节点聚合视图：节点主档、`metrics_available`、`metrics_fresh`、`scheduler_ready`、`state_reason`、`online_estimate`、`last_metrics_at`
- `topology`：补充当前节点角色、控制面节点列表与后台可访问节点统计，便于判断后台当前连到的是不是控制面节点
- `topology` 还会返回推荐后台入口，后台轮询页和巡检脚本可直接用来选择控制面目标
- `resource/distribution`：返回每台节点的活跃执行数、转码会话数、上传队列深度、剩余转码容量、剩余上传容量，以及每张 GPU 的活跃会话与实时利用率
- `resource/distribution` 还会补充 `control_plane`、`admin_accessible`、`online_signal_source`，便于一眼区分控制面节点与执行节点
- `gpu_summary`：单节点 GPU 数、健康数、可调度数、最大并发会话数
- `overview` / `realtime` / `topology` / `resource/distribution` 当前会写入 2~3 秒的 Redis 摘要缓存，用于削平后台高频轮询尖峰；正常运维刷新通常不会感知到差异，但不应把这些接口当作亚秒级强实时流使用

说明：
- `cluster/realtime` 面向后台轮询，返回的是节点聚合视图，不返回 `gpu_devices`
- `gpu_devices[*].runtime_capability` 仅在 `cluster/node/detail` 中返回，适合排障和能力核查
- 示例中的 `internal_grpc.listen_address` 属于 bootstrap 固定入口，默认示例端口为 `:19090`；对外 `public gRPC` 监听地址由 runtime config 中的 `grpc_listen_address` 决定，默认示例端口为 `:9090`
- `overview` 与 `realtime` 中的 `public_grpc` 用于回答“当前北向 RPC 是否真的启着、是否绑定了 etcd、绑定的是哪一组注册配置”

**集群治理写接口返回说明：**

- `/cluster/node/enabled` 返回：`node_id`, `enabled`, `target_status`, `takeover_active_jobs`, `takeover_result`
- `/cluster/node/quarantined` 返回：`node_id`, `quarantined`, `target_status`, `takeover_active_jobs`, `takeover_result`
- `/cluster/node/draining` 返回：`node_id`, `draining`, `target_status`, `takeover_active_jobs`, `takeover_result`
- `/cluster/worker/offline` 与 `/cluster/worker/exit` 返回：`worker_id`, `node_id`, `control_path`, `target_status`, `takeover_active_jobs`, `takeover_result`
- `/cluster/job/takeover` 返回：`matched_job_total`, `reset_job_total`, `abandoned_execution_total`

**集群总览响应关键结构：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "mode": "cluster-allinone",
    "module_status": {
      "http_enabled": true,
      "public_grpc_enabled": true,
      "internal_grpc_enabled": true,
      "mq_consumer_enabled": true,
      "scheduler_enabled": true,
      "worker_enabled": true,
      "callback_enabled": true
    },
    "scheduler": {
      "dynamic_concurrency_control": true,
      "max_global_transcode_sessions": 24,
      "max_node_transcode_sessions": 12,
      "worker_heartbeat_timeout_sec": 30,
      "active_execution_total": 6,
      "remaining_execution_capacity": 18,
      "require_hardware_encode": false,
      "allow_software_decode_fallback": true
    },
    "internal_grpc": {
      "listen_address": ":19090"
    },
    "public_grpc": {
      "enabled": true,
      "listen_address": ":9090",
      "effective_endpoint": ":9090",
      "registry_bound": true,
      "registry_id": 3001,
      "service_name": "transcode.v1.TranscodePublicService",
      "discovery_mode": "etcd",
      "service_namespace": "/vod/transcoding",
      "registry_endpoints": [
        "127.0.0.1:2379"
      ],
      "registry_enabled": true,
      "lease_ttl_sec": 30,
      "dial_timeout_ms": 3000
    },
    "governance": {
      "window_minutes": 1440,
      "recent_action_total": 3,
      "node_enable_total": 0,
      "node_disable_total": 0,
      "node_quarantine_total": 0,
      "node_unquarantine_total": 0,
      "node_drain_total": 1,
      "node_resume_total": 0,
      "worker_offline_total": 1,
      "worker_exit_total": 0,
      "job_takeover_total": 1,
      "latest_action": {
        "action_name": "cluster.node.drain",
        "target_type": "cluster_node",
        "target_id": "1",
        "result_code": 0,
        "result_message": "{\"target_status\":\"draining\",\"takeover_active_jobs\":true,\"takeover_result\":{\"matched_job_total\":3,\"reset_job_total\":2,\"abandoned_execution_total\":1},\"reason\":\"rolling_upgrade\"}",
        "result": {
          "target_status": "draining",
          "takeover_active_jobs": true,
          "takeover_result": {
            "matched_job_total": 3,
            "reset_job_total": 2,
            "abandoned_execution_total": 1
          },
          "extras": {
            "reason": "rolling_upgrade"
          }
        },
        "request_ip": "127.0.0.1:54321",
        "created_at": "2026-05-11T10:00:00+08:00",
        "admin_user_id": 1,
        "username": "admin"
      },
      "recent_actions": [
        {
          "action_name": "cluster.node.drain",
          "target_type": "cluster_node",
          "target_id": "1",
          "result_code": 0,
          "result_message": "{\"target_status\":\"draining\",\"takeover_active_jobs\":true,\"takeover_result\":{\"matched_job_total\":3,\"reset_job_total\":2,\"abandoned_execution_total\":1},\"reason\":\"rolling_upgrade\"}",
          "result": {
            "target_status": "draining",
            "takeover_active_jobs": true,
            "takeover_result": {
              "matched_job_total": 3,
              "reset_job_total": 2,
              "abandoned_execution_total": 1
            },
            "extras": {
              "reason": "rolling_upgrade"
            }
          },
          "request_ip": "127.0.0.1:54321",
          "created_at": "2026-05-11T10:00:00+08:00",
          "admin_user_id": 1,
          "username": "admin"
        }
      ]
    },
    "cluster": {
      "node_total": 2,
      "node_enabled_total": 2,
      "node_quarantined_total": 0,
      "node_draining_total": 0,
      "node_online_total": 2,
      "member_total": 2,
      "gpu_total": 4,
      "gpu_healthy_total": 4,
      "gpu_schedulable_total": 4,
      "active_sessions": 6,
      "upload_queue_depth": 3,
      "max_transcode_sessions": 24,
      "nodes": [
        {
          "node_id": 1,
          "node_name": "node-a",
          "node_role": "cluster-allinone",
          "enabled": true,
          "quarantined": false,
          "draining": false,
          "control_plane": true,
          "admin_accessible": true,
          "online_estimate": true,
          "online_signal_source": "metrics",
          "scheduler_ready": true,
          "state_reason": "ready",
          "gpu_total": 2,
          "gpu_schedulable_total": 2,
          "active_transcode_sessions": 3,
          "upload_queue_depth": 1
        }
      ]
    },
    "version": {
      "mysql_server_version": "8.0.36",
      "redis_server_version": "7.2.5",
      "redis_mode": "standalone",
      "runtime_config_db_version": 1893456789012345678,
      "runtime_config_cache_version": 1893456789012345678,
      "runtime_config_cache_exists": true,
      "runtime_config_cache_ttl_sec": 286
    }
  }
}
```

**节点详情请求（GET Query）：**

```
GET /v1/admin/cluster/node/detail?node_id=1
```

**节点指标请求（GET Query）：**

```
GET /v1/admin/cluster/node/metrics
GET /v1/admin/cluster/node/metrics?node_id=1
```

说明：
- `node.list` 查询参数：`page`, `page_size`, `keyword`, `enabled`, `quarantined`, `draining`
- `node.detail` 统一从 query string 读取 `node_id`
- `node.metrics` 查询参数：`page`, `page_size`, `keyword`, `enabled`, `quarantined`, `draining`, `node_id`
- `member.list` 查询参数：`page`, `page_size`, `keyword`, `enabled`, `quarantined`, `draining`
- `node.metrics` 现以节点分页为基准，只拼当前页节点的最新运行态指标；传 `node_id` 时仅返回指定节点
- `member.list` 现以节点主表分页为基准，再补充 registry / metrics / worker 汇总字段，不再先构造全量成员快照后内存分页
- `overview` / `realtime` / `resource.distribution` 中的任务状态计数、按节点活跃任务数、Worker 总数与按节点在线数，现已改为数据库聚合查询，避免后台轮询时反复把整张任务表和 Worker 表拉回进程内统计
- `overview` / `realtime` / `resource.distribution` / `node.list` / `node.metrics` 读取节点运行态指标时，现已改为按当前节点集合批量执行 Redis `MGET`，不再逐节点单独往返 Redis
- `node.list` / `member.list` 构建节点聚合视图时，只会查询当前节点集合对应的 GPU 主档，不再为分页结果额外拉取整张 GPU 表
- `overview` / `realtime` 的 GPU 总数、健康数、可调度数、最大会话数，现已直接走数据库按节点聚合，不再先构造所有 GPU 明细对象后再汇总
- `scheduler.insight` 构建候选节点快照时，也会按当前节点集合批量读取 Redis metrics，并复用数据库 GPU 活跃会话聚合结果
- 调度器真实分配循环不会再每轮全量拉取所有 queued 任务，而是按“优先级倒序 + 创建时间正序”拉取一个与剩余全局并发容量相关的有限批次
- 调度真实分配和 `scheduler.insight` 的 codec 过滤现在都会消费任务中锁定的 `renditions[*].video_codec`，不会再出现请求指定编码格式但调度过滤阶段未生效的问题
- 节点在线估算所依赖的 registry 心跳也会先一次性构造成内存快照，再参与节点视图拼装，避免按节点重复扫描 registry
- `node.list` 与 `node.detail` 返回聚合视图，而不是单纯数据库原始行；后台不需要再自行拼装 GPU 与实时指标
- `node.list.items[*]` 关键字段包括：`node_id`, `node_name`, `node_role`, `host_ip`, `grpc_host`, `http_host`, `enabled`, `quarantined`, `draining`, `drain_reason`, `control_plane`, `admin_accessible`, `metrics_fresh`, `online_signal_source`, `scheduler_ready`, `state_reason`, `last_metrics_at`, `metrics`, `gpu_summary`
- `overview.cluster.nodes[*]` 关键字段包括：`node_role`, `control_plane`, `admin_accessible`, `online_signal_source`, `scheduler_ready`, `state_reason`
- `member.list.items[*]` 关键字段包括：`node_role`, `control_plane`, `admin_accessible`
- `member.list.items[*]` 关键字段包括：`node_id`, `node_name`, `host`, `host_ip`, `grpc_host`, `http_host`, `online_estimate`, `online_signal_source`, `scheduler_ready`, `state_reason`, `source`, `last_heartbeat_at`, `registry_heartbeat_at`, `last_metrics_at`, `worker_total`, `worker_online_total`, `draining`
- `worker.list` 查询参数：`page`, `page_size`, `node_id`, `worker_id`, `status`, `online_only`
- `worker.list.items[*]` 关键字段包括：`worker_id`, `logical_worker_id`, `physical_worker_id`, `startup_instance_id`, `online_estimate`, `status_source`, `last_heartbeat_at`
- `worker.list.items[*].status_name` 当前取值：`online`、`offline`、`exited`；其中 `offline` 由心跳超时自动收口
- `worker.list.items[*].status_source` 当前取值包括：`heartbeat`、`heartbeat_stale`、`heartbeat_timeout`、`manual_offline`、`manual_exit`、`process_exit`
- `draining=true` 表示节点处于排空维护模式，只阻止新任务调度，不会中断已在执行的任务
- `online_signal_source` 当前取值包括：`metrics`、`node_heartbeat`、`registry`、`unknown`。其中 control-plane 节点即使不跑 worker metrics，也会通过 registry 心跳保持在线判定一致
- `scheduler.insight` 查询参数：`preferred_hw_accel`, `video_codec`, `enable_watermark`
- `scheduler.insight` 会返回当前调度器快照、候选节点列表、过滤原因、Top-K 池和推荐结果，主要用于后台排障与容量分析
- `scheduler.insight.candidates[*]` 关键字段包括：`node_id`, `node_name`, `node_role`, `enabled`, `quarantined`, `draining`, `control_plane`, `admin_accessible`, `shielded`, `metrics_available`, `metrics_fresh`, `online`, `online_signal_source`, `scheduler_ready`, `state_reason`, `score`, `score_rank`, `filter_passed`, `filter_reasons`, `in_top_k_pool`, `decision_preview`, `metrics`

**集群实时快照请求（GET）：**

```
GET /v1/admin/cluster/realtime
```

**Worker 实例列表请求（GET）：**

```
GET /v1/admin/cluster/worker/list?page=1&page_size=20&node_id=1&worker_id=&status=1&online_only=true
```

**Worker 手动下线请求：**

```json
{
  "worker_id": "worker-node1-001",
  "reason": "manual isolate",
  "takeover_active_jobs": true
}
```

成功响应会额外返回：
- `node_id`：目标 Worker 所属节点
- `control_path`：本次控制实际走的是 `local_worker_module` 还是 `internal_grpc`
- `target_status`：目标状态，当前为 `offline`
- `takeover_active_jobs`：是否已请求联动任务接管
- `takeover_result`：当 `takeover_active_jobs=true` 时返回接管统计，包括 `matched_job_total`、`reset_job_total`、`abandoned_execution_total`

**Worker 手动标记退出请求：**

```json
{
  "worker_id": "worker-node1-001",
  "reason": "process terminated by operator",
  "takeover_active_jobs": true
}
```

成功响应会额外返回：
- `node_id`：目标 Worker 所属节点
- `control_path`：本次控制实际走的是 `local_worker_module` 还是 `internal_grpc`
- `target_status`：目标状态，当前为 `exited`
- `takeover_active_jobs`：是否已请求联动任务接管
- `takeover_result`：当 `takeover_active_jobs=true` 时返回接管统计，包括 `matched_job_total`、`reset_job_total`、`abandoned_execution_total`

**调度洞察请求（GET）：**

```
GET /v1/admin/cluster/scheduler/insight?preferred_hw_accel=nvidia&video_codec=h264&enable_watermark=false
```

**启用/禁用节点请求：**

```json
{
  "node_id": 1,
  "enabled": false,
  "reason": "host maintenance",
  "takeover_active_jobs": true
}
```

成功响应会额外返回：
- `target_status`：当前为 `enabled` 或 `disabled`
- `takeover_active_jobs`：仅当 `enabled=false` 时才可能为 `true`
- `takeover_result`：当 `takeover_active_jobs=true` 时返回接管统计，包括 `matched_job_total`、`reset_job_total`、`abandoned_execution_total`

**隔离/取消隔离节点请求：**

```json
{
  "node_id": 1,
  "quarantined": true,
  "reason": "GPU故障维护",
  "takeover_active_jobs": true
}
```

成功响应会额外返回：
- `target_status`：当前为 `quarantined` 或 `active`
- `takeover_active_jobs`：仅当 `quarantined=true` 时才可能为 `true`
- `takeover_result`：当 `takeover_active_jobs=true` 时返回接管统计，包括 `matched_job_total`、`reset_job_total`、`abandoned_execution_total`

**排空/恢复节点请求：**

```json
{
  "node_id": 1,
  "draining": true,
  "reason": "rolling upgrade",
  "takeover_active_jobs": true
}
```

成功响应会额外返回：
- `target_status`：当前为 `draining` 或 `schedulable`
- `takeover_active_jobs`：仅当 `draining=true` 时才可能为 `true`
- `takeover_result`：当 `takeover_active_jobs=true` 时返回接管统计，包括 `matched_job_total`、`reset_job_total`、`abandoned_execution_total`

### 4.8 转码任务管理

| 接口 | 方法 | 权限 | 说明 |
|------|------|------|------|
| `/v1/admin/transcode/job/list` | GET | transcode.job.read | 任务列表（分页+筛选） |
| `/v1/admin/transcode/job/detail` | GET | transcode.job.detail.read | 任务详情 |
| `/v1/admin/transcode/job/progress` | GET | transcode.job.read | 后台进度查询（含实时信息） |
| `/v1/admin/transcode/job/retry` | POST | transcode.job.retry | 重试失败任务 |
| `/v1/admin/transcode/job/cancel` | POST | transcode.job.cancel | 取消任务 |

**任务列表查询参数：** `page`, `page_size`, `status`, `biz_key`, `request_id`

**任务列表返回字段：**
- `items[*]` 关键字段包括：`job_id`, `request_id`, `biz_key`, `source_url`, `status`, `status_name`, `progress_permille`, `stage`, `assigned_node_id`, `assigned_worker_id`, `created_at`, `updated_at`

**任务详情查询参数：** `job_id`

**后台进度查询参数：** `job_id`

**后台进度响应（含实时信息）：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "job_id": 1893456789012345678,
    "status": 4,
    "stage": "TRANSCODING",
    "progress_permille": 450,
    "current_fps": 120.5,
    "current_bitrate_kbps": 4800.0,
    "current_speed": 4.02,
    "elapsed_ms": 3600000,
    "estimated_remaining_ms": 4400000,
    "updated_at": "2026-05-07T10:30:00Z"
  }
}
```

说明：
- Redis 中存在实时进度快照时，会返回 `current_fps/current_bitrate_kbps/current_speed/elapsed_ms/estimated_remaining_ms/updated_at`
- Redis 中不存在实时快照时，会退化为数据库摘要，仅返回 `job_id/status/stage/progress_permille`

**任务详情响应关键结构：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "job_id": 1893456789012345678,
    "request_id": "req-001",
    "biz_key": "video-001",
    "source_url": "https://example.com/source.mp4",
    "status": 4,
    "status_name": "转码中",
    "progress_permille": 450,
    "stage": "TRANSCODING",
    "profile_id": 2,
    "enable_watermark": false,
    "segment_duration_sec": 6,
    "support_dash": true,
    "support_hls": true,
    "selected_execution_hw": "nvenc",
    "assigned_node_id": 1,
    "assigned_worker_id": "worker-a-01",
    "lease_generation": 8,
    "realtime_progress": {
      "current_fps": 120.5,
      "current_bitrate_kbps": 4800.0,
      "current_speed": 4.02,
      "elapsed_ms": 3600000,
      "estimated_remaining_ms": 4400000
    },
    "created_at": "2026-05-09T10:00:00+08:00",
    "updated_at": "2026-05-09T10:30:00+08:00"
  }
}
```

**重试/取消请求：**

```json
{
  "job_id": 1893456789012345678
}
```

### 4.9 直播频道管理

| 接口 | 方法 | 权限 | 说明 |
|------|------|------|------|
| `/v1/admin/live/channel/create` | POST | live.channel.create | 创建频道 |
| `/v1/admin/live/channel/list` | GET | live.channel.read | 频道列表（数据库分页/过滤） |
| `/v1/admin/live/channel/detail` | GET | live.channel.read | 频道详情 |
| `/v1/admin/live/channel/update` | POST | live.channel.update | 更新频道 |
| `/v1/admin/live/channel/start` | POST | live.channel.start | 启动频道 |
| `/v1/admin/live/channel/stop` | POST | live.channel.stop | 停止频道 |
| `/v1/admin/live/channel/delete` | POST | live.channel.delete | 删除频道 |
| `/v1/admin/live/session/list` | GET | live.session.read | 直播会话列表（数据库分页/过滤） |

**创建频道请求：**

```json
{
  "channel_key": "live-001",
  "channel_name": "测试频道",
  "profile_id": 0
}
```

**频道列表查询参数：** `page`, `page_size`, `status`, `channel_key`

**直播会话列表查询参数：** `page`, `page_size`, `channel_id`, `channel_key`, `status`

**频道列表返回字段：**
- `items[*]` 关键字段包括：`channel_id`, `channel_key`, `channel_name`, `profile_id`, `status`, `enable_source_rendition`, `enable_watermark`, `play_domain`, `push_domain`, `assigned_node_id`, `assigned_worker_id`, `created_at`, `updated_at`

**直播会话列表返回字段：**
- `items[*]` 关键字段包括：`session_id`, `channel_id`, `channel_key`, `session_key`, `status`, `ingest_url`, `playback_hls_url`, `push_protocol`, `assigned_node_id`, `assigned_worker_id`, `started_at`, `stopped_at`, `resume_count`

**频道详情查询参数：** `channel_id` 或 `channel_key`

**直播控制接口说明：**

- `/v1/admin/live/channel/start` 支持按 `node_id` 指定目标节点；若目标不是当前节点，控制面会自动转发到目标节点内部 HTTP 控制接口
- `/v1/admin/live/channel/stop` 会优先根据频道当前 `assigned_node_id` 选择本地执行或远程转发
- `start / stop` 会返回 `channel_id`, `control_path`, `target_node_id`, `target_worker_id`，便于后台明确看到本次控制是本地执行还是内部转发
- `delete` 仍返回统一成功结构；最终状态请以 `channel/detail` 和列表接口为准

说明：
- 频道详情响应会同时返回 `channel`、`playback`，以及存在时的 `active_session`。
- 当使用 `channel_key` 查询详情时，返回的仍然是完整频道详情，而不是仅播放地址快照。

**频道详情响应关键结构：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "channel": {
      "channel_id": 1,
      "channel_key": "live-001",
      "channel_name": "测试频道",
      "profile_id": 2,
      "status": "LIVE",
      "enable_source_rendition": true,
      "enable_watermark": false,
      "play_domain": "https://play.example.com/live",
      "push_domain": "rtmp://push.example.com/live",
      "assigned_node_id": 1,
      "assigned_worker_id": "worker-a-01",
      "created_at": "2026-05-09T10:00:00+08:00",
      "updated_at": "2026-05-09T10:10:00+08:00"
    },
    "playback": {
      "channel_key": "live-001",
      "status": "LIVE",
      "play_token": "token-abc",
      "expire_at": 1894000000,
      "master_hls_url": "https://play.example.com/live/live-001/index.m3u8",
      "http_flv_url": "https://play.example.com/live/live-001.flv",
      "rendition_names": ["source", "1080p"],
      "renditions": [
        {
          "rendition_name": "1080p",
          "is_source": false,
          "hls_url": "https://play.example.com/live/live-001/1080p.m3u8",
          "http_flv_url": "https://play.example.com/live/live-001-1080p.flv"
        }
      ]
    },
    "active_session": {
      "session_id": 1001,
      "channel_id": 1,
      "channel_key": "live-001",
      "session_key": "sess-001",
      "status": "PUBLISHING",
      "ingest_url": "rtmp://push.example.com/live/live-001",
      "playback_hls_url": "https://play.example.com/live/live-001/index.m3u8",
      "push_protocol": "rtmp",
      "assigned_node_id": 1,
      "assigned_worker_id": "worker-a-01",
      "started_at": "2026-05-09T10:20:00+08:00",
      "resume_count": 0
    }
  }
}
```

**更新频道请求：**

```json
{
  "channel_id": 1,
  "channel_name": "新名称",
  "profile_id": 2,
  "enable_source_rendition": true,
  "enable_watermark": true,
  "play_domain": "https://play.example.com/live",
  "push_domain": "rtmp://push.example.com/live"
}
```

说明：
- `update` 只会修改请求体中明确传入的字段；未传字段保持原值，不会被零值覆盖。

**启动频道请求：**

```json
{
  "channel_id": 1,
  "node_id": 0,
  "worker_id": ""
}
```

**删除频道请求：**

```json
{
  "channel_id": 1
}
```

说明：
- 仅允许删除不存在活动会话且未处于运行中的频道

**停止频道请求：**

```json
{
  "channel_id": 1
}
```

**会话列表查询参数：** `page`, `page_size`, `channel_id`, `channel_key`, `status`

### 4.10 审计日志

| 接口 | 方法 | 权限 | 说明 |
|------|------|------|------|
| `/v1/admin/audit/list` | GET | audit.read | 查询审计日志 |

**审计日志查询参数：** `page`, `page_size`, `action_name`, `admin_user_id`, `start_time`, `end_time`

**审计日志返回字段：**
- `items[*]` 关键字段包括：`audit_log_id`, `admin_user_id`, `username`, `action_name`, `target_type`, `target_id`, `request_id`, `request_ip`, `result_code`, `result_message`, `created_at`

**回调配置列表示例响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "page": 1,
    "page_size": 20,
    "total": 1,
    "items": [
      {
        "callback_config_id": 1001,
        "callback_name": "biz-http-callback",
        "callback_type": 1,
        "target_url": "https://callback.example.com/transcode",
        "rpc_endpoint": "",
        "rpc_service_name": "",
        "mq_exchange": "",
        "mq_routing_key": "",
        "timeout_ms": 5000,
        "retry_times": 3,
        "enabled": true,
        "priority": 10,
        "registry_id": 0,
        "created_at": "2026-05-09T10:00:00+08:00",
        "updated_at": "2026-05-09T10:00:00+08:00"
      }
    ]
  }
}
```

**配置中心绑定列表示例响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "config_scope": "bootstrap",
    "affects_runtime": false,
    "runtime_update_paths": {
      "server": "/v1/admin/config/runtime/server/update",
      "scheduler": "/v1/admin/config/runtime/scheduler/update",
      "worker": "/v1/admin/config/runtime/worker/update",
      "storage": "/v1/admin/config/runtime/storage/update",
      "callback": "/v1/admin/config/runtime/callback/update",
      "mq": "/v1/admin/config/runtime/mq/update",
      "grpc": "/v1/admin/config/runtime/grpc/update"
    },
    "runtime_publish_path": "/v1/admin/config/publish",
    "page": 1,
    "page_size": 20,
    "total": 1,
    "items": [
      {
        "binding_id": 2001,
        "binding_name": "prod-bootstrap-source",
        "provider_type": "nacos",
        "endpoint": "http://nacos:8848",
        "namespace": "production",
        "auth_mode": "token",
        "access_key": "",
        "secret_key": "",
        "token": "",
        "enabled": true,
        "priority": 10,
        "last_sync_status": "ok",
        "last_sync_message": "",
        "created_at": "2026-05-09T10:00:00+08:00",
        "updated_at": "2026-05-09T10:00:00+08:00",
        "config_scope": "bootstrap",
        "affects_runtime": false,
        "binding_usage": ["mysql", "redis", "cluster_registry", "node_identity"]
      }
    ]
  }
}
```

**审计日志列表示例响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "page": 1,
    "page_size": 20,
    "total": 1,
    "items": [
      {
        "audit_log_id": 3001,
        "admin_user_id": 1,
        "username": "admin",
        "action_name": "cluster.node.enable",
        "target_type": "cluster_node",
        "target_id": "1",
        "request_id": "req-audit-001",
        "request_ip": "127.0.0.1",
        "request_user_agent": "PostmanRuntime/7.44.0",
        "result_code": 0,
        "result_message": "ok",
        "created_at": "2026-05-09T10:00:00+08:00"
      }
    ]
  }
}
```

### 4.11 运行日志

| 接口 | 方法 | 权限 | 说明 |
|------|------|------|------|
| `/v1/admin/system/log/list` | GET | system.log.read | 查询系统运行日志 |

**运行日志查询参数：** `page`, `page_size`, `level`, `action_name`, `start_time`, `end_time`

**运行日志返回字段：**
- `items[*]` 关键字段包括：`log_id`, `service_name`, `log_level`, `action_name`, `fields`, `logged_at`
- `action_name` 查询当前支持模糊匹配，便于按动作前缀或关键片段检索

实现说明：

- 运行日志写入链路采用“进程内异步队列 + 数据库批量落库”模式，不阻塞业务主线程
- 文件日志仍按 `log/YYYY-MM-DD.log` 按天滚动
- 终端只镜像少量启动/监听类信息日志，错误日志仍会输出，避免控制台被普通 info 刷屏
- 对高频路径（如 `/healthz`、`/v1/manifest/*`、转码进度查询、监控快照、集群总览轮询、internal heartbeat/metrics/lease`）会省略 request/response body，只保留 `request_body_size`、`response_body_size` 以及 `*_body_omitted=true` 摘要字段

**运行日志示例响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "page": 1,
    "page_size": 20,
    "total": 1,
    "items": [
      {
        "log_id": 3001,
        "service_name": "hili-video-cloud",
        "log_level": "info",
        "action_name": "http.server.listening",
        "fields": {
          "address": ":8080"
        },
        "logged_at": "2026-05-09T12:00:00+08:00"
      }
    ]
  }
}
```

### 4.12 WebSocket 实时监控

```
GET /v1/admin/transcode/monitor/ws
Upgrade: websocket
Authorization: Bearer {session_token}
```

连接后接收实时转码进度快照推送。

### 4.13 HTTP 快照接口

```
GET /v1/admin/transcode/monitor/snapshot
Authorization: Bearer {session_token}
```

一次性拉取完整监控快照。
该接口返回统一 `code/message/data` 响应结构，`data` 中承载完整监控快照。

实现说明：
- `pending_jobs` 直接来自数据库状态计数，不再通过拉取全部 queued 任务后在进程内取长度
- `nodes[*].online` 会优先参考 Redis 中最近的节点 metrics，缺失时再退化为节点主档心跳时间
- HTTP `monitor/snapshot` 当前也会写入 2 秒 Redis 摘要缓存，避免多个后台页面并发轮询时重复做同一份快照拼装
- Worker 与 control-plane 节点心跳采用“Redis 高频、MySQL 节流持久化”模型：
  - Redis 心跳每轮刷新，供调度器、监控和实时接口优先读取
  - `t_cluster_node.last_heartbeat_at`、`t_worker_instance.last_heartbeat_at` 只按节流窗口落库，避免数据库变成高频时序写热点
  - 节流窗口始终小于 `worker_heartbeat_timeout`，不会改变离线判定语义
- 任务进度与执行实例心跳也采用同样思路：
  - Redis `progress_store` 继续保存实时帧率、码率、速度、剩余时长等高频进度
  - `t_transcode_job.progress_*` 只在阶段切换、进度明显跃迁或节流窗口到达时持久化
  - `t_transcode_job_execution.last_heartbeat_at` 只按节流窗口更新，后台活跃执行数仍能正确统计

**示例响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "timestamp": 1746756000000,
    "mode": "cluster",
    "pending_jobs": 2,
    "active_jobs": 5,
    "node_count": 3,
    "system_metrics": {
      "total_active_sessions": 5,
      "total_pending_jobs": 2
    },
    "nodes": [
      {
        "node_id": 1,
        "online": true
      }
    ]
  }
}
```

---

## 五、HTTP 集群内部接口

> 仅允许集群内部网段访问，不对外暴露。

### 5.1 Worker 心跳上报

```
POST /v1/internal/worker/heartbeat
Content-Type: application/json
```

```json
{
  "node_id": 1,
  "worker_id": "worker-node1-001",
  "startup_instance_id": "inst-20260507-001",
  "machine_fingerprint": "fp-abc123",
  "timestamp": "2026-05-07T10:00:00Z"
}
```

**响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "accepted": true
  }
}
```

实现说明：

- 该接口先写 Redis 热路径，再按节流策略更新 `t_cluster_node` 与 `t_worker_instance`
- 如果请求体携带 `timestamp`，数据库持久化会保留该时间戳，不会被服务端重新改写成接收时间

### 5.2 Worker 指标上报

```
POST /v1/internal/worker/metrics
Content-Type: application/json
```

```json
{
  "node_id": 1,
  "worker_id": "worker-node1-001",
  "startup_instance_id": "inst-20260507-001",
  "machine_fingerprint": "fp-abc123",
  "cpu_usage_percent": 45,
  "memory_usage_percent": 60,
  "gpu_memory_usage_percent": 70,
  "upload_queue_depth": 2,
  "active_transcode_sessions": 3,
  "gpu_capabilities": [
    {
      "gpu_device_id": 1,
      "gpu_uuid": "GPU-abc123",
      "gpu_index": 0,
      "encode_codecs": ["h264", "hevc"],
      "decode_codecs": ["h264", "hevc"],
      "execution_hw_types": ["nvidia"],
      "max_sessions": 3,
      "supports_filter": true
    }
  ],
  "timestamp": "2026-05-07T10:00:00Z"
}
```

**响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "accepted": true
  }
}
```

### 5.3 任务租约续约

```
POST /v1/internal/jobs/lease/renew
Content-Type: application/json
```

```json
{
  "job_id": 1893456789012345678,
  "worker_id": "worker-node1-001",
  "lease_generation": 1,
  "expire_at": "2026-05-07T10:05:00Z"
}
```

**响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "accepted": true,
    "lease_generation": 1
  }
}
```

### 5.4 分片上传成功通知

```
POST /v1/internal/segments/upload-succeeded
Content-Type: application/json
```

```json
{
  "segment_id": 100,
  "object_etag": "abc123def456",
  "object_size_bytes": 524288
}
```

### 5.5 分片上传失败通知

```
POST /v1/internal/segments/upload-failed
Content-Type: application/json
```

```json
{
  "segment_id": 100,
  "retry_count": 1,
  "error_message": "connection refused"
}
```

### 5.6 直播推流建立通知

```
POST /v1/internal/live/publish-connected
Content-Type: application/json
```

```json
{
  "channel_key": "live-001",
  "node_id": 1,
  "worker_id": "worker-node1-001",
  "stream_key": "live-001",
  "publish_ip": "10.0.0.10",
  "push_protocol": "rtmp",
  "ingest_url": "rtmp://push.example.com/live/live-001",
  "playback_hls_url": "https://play.example.com/live/live-001/master.m3u8"
}
```

### 5.7 直播推流断开通知

```
POST /v1/internal/live/publish-disconnected
Content-Type: application/json
```

```json
{
  "channel_key": "live-001",
  "stream_key": "live-001"
}
```

### 5.8 直播推流中断通知

```
POST /v1/internal/live/publish-interrupted
Content-Type: application/json
```

```json
{
  "channel_key": "live-001",
  "stream_key": "live-001"
}
```

### 5.9 内部启动直播频道

```
POST /v1/internal/live/channel/start
Content-Type: application/json
```

```json
{
  "channel_id": 1,
  "node_id": 1,
  "worker_id": "worker-node1-001"
}
```

### 5.10 内部停止直播频道

```
POST /v1/internal/live/channel/stop
Content-Type: application/json
```

```json
{
  "channel_id": 1
}
```

---

## 六、gRPC 公共接口

服务名：`transcode.v1.TranscodePublicService`

监听地址：`:9090`

### 6.1 CreateJob

```protobuf
rpc CreateJob(CreateJobRequest) returns (CreateJobResponse);
```

**请求字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| request_id | string | 幂等控制 |
| biz_key | string | 业务关联键 |
| source_url | string | 源视频地址 |
| profile_id | uint64 | 转码模板ID |
| priority | int32 | 优先级 |
| enable_watermark | bool | 是否水印 |
| segment_options | SegmentOptions | 分片配置 |
| storage_options | StorageOptions | 存储配置 |
| schedule_options | ScheduleOptions | 调度配置 |
| watermark | Watermark | 水印配置 |
| renditions | repeated RenditionOption | 自定义清晰度 |
| video_options | VideoOptions | 视频输出选项 |
| thumbnail_options | ThumbnailOptions | 缩略图选项 |
| callback_url | string | 单任务回调目标，优先于系统级回调配置，支持 HTTP/gRPC/MQ |

**响应字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| error | Error | 错误信息（code + message） |
| job_id | uint64 | 任务ID（雪花ID） |
| request_id | string | 请求ID |
| status | int32 | 状态码 |
| status_name | string | 状态名称 |

### 6.2 QueryProgress

```protobuf
rpc QueryProgress(QueryProgressRequest) returns (QueryProgressResponse);
```

**请求字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| request_id | string | 外部请求唯一标识 |

**响应字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| error | Error | 错误信息 |
| job_id | uint64 | 任务ID |
| status | int32 | 状态码 |
| stage | string | 当前阶段 |
| progress_permille | int32 | 进度千分比（0~1000） |
| current_fps | double | 当前帧率 |
| current_bitrate_kbps | double | 当前码率 |
| current_speed | double | 当前速度倍率 |

---

## 七、gRPC 集群内部接口

服务名：`cluster.v1.ClusterInternalService`

### 7.1 WorkerHeartbeat

```protobuf
rpc WorkerHeartbeat(WorkerHeartbeatRequest) returns (WorkerHeartbeatResponse);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| node_id | uint64 | 节点ID |
| worker_id | string | Worker实例标识 |
| startup_instance_id | string | 启动实例ID |
| machine_fingerprint | string | 机器指纹 |

**响应字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| accepted | bool | 是否接受心跳 |
| lease_generation | uint64 | 当前租约代数 |

实现说明：

- internal gRPC 心跳与 internal HTTP 心跳的落库语义保持一致：都会同步维护 `t_cluster_node` 与 `t_worker_instance`
- gRPC 请求体当前没有 `timestamp` 字段，因此数据库落库时间使用服务端接收时间
- 落库路径同样采用“Redis 高频、MySQL 节流持久化”策略，避免集群内部高频心跳把数据库打成热点

### 7.2 ReportProgress

```protobuf
rpc ReportProgress(ReportProgressRequest) returns (ReportProgressResponse);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| job_id | uint64 | 任务ID |
| status | int32 | 状态码 |
| stage | string | 当前阶段 |
| progress_permille | int32 | 进度千分比 |
| fps | double | 当前帧率 |
| bitrate_kbps | double | 当前码率 |
| speed | double | 当前速度倍率 |

**响应字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| accepted | bool | 是否接受进度上报 |

### 7.3 SegmentUploaded

```protobuf
rpc SegmentUploaded(SegmentUploadedRequest) returns (SegmentUploadedResponse);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| job_id | uint64 | 任务ID |
| segment_id | uint64 | 分片ID |
| object_key | string | 对象存储键（按模板命名） |
| object_size_bytes | uint64 | 对象大小 |
| object_etag | string | 对象ETag |

**响应字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| accepted | bool | 是否接受通知 |

### 7.4 SetWorkerOffline

```protobuf
rpc SetWorkerOffline(SetWorkerOfflineRequest) returns (SetWorkerOfflineResponse);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| worker_id | string | 目标 Worker 实例标识 |
| reason | string | 运维下线原因 |

**响应字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| accepted | bool | 是否接受控制动作 |
| message | string | 控制结果说明 |

### 7.5 RequestWorkerExit

```protobuf
rpc RequestWorkerExit(RequestWorkerExitRequest) returns (RequestWorkerExitResponse);
```

| 字段 | 类型 | 说明 |
|------|------|------|
| worker_id | string | 目标 Worker 实例标识 |
| reason | string | 退出原因 |

**响应字段：**

| 字段 | 类型 | 说明 |
|------|------|------|
| accepted | bool | 是否接受控制动作 |
| message | string | 控制结果说明 |

---

## 八、消息队列接口

### 8.1 Exchange 定义

| 名称 | 类型 | 说明 |
|------|------|------|
| hvc.transcode | topic | 转码事件交换机 |

### 8.2 队列定义

| 队列名 | Routing Key | 说明 |
|--------|-------------|------|
| hvc.transcode.create | transcode.job.create | 创建转码任务 |
| hvc.transcode.completed | transcode.job.completed | 转码完成通知 |
| hvc.transcode.failed | transcode.job.failed | 转码失败通知 |
| hvc.transcode.progress | transcode.job.progress | 进度更新 |
| hvc.live.stream.start | live.stream.start | 直播推流开始 |
| hvc.live.stream.stop | live.stream.stop | 直播推流停止 |

### 8.3 触发转码消息（transcode.job.create）

```json
{
  "request_id": "req-001",
  "biz_key": "biz-video-001",
  "source_url": "https://example.com/video.mp4",
  "profile_id": 0,
  "priority": 5,
  "enable_watermark": false,
  "watermark": {
    "image_url": "",
    "anchor": 4,
    "x_ratio": 0.05,
    "y_ratio": 0.05,
    "width_ratio": 0.15,
    "opacity": 0.8,
    "safe_margin_ratio": 0.02
  },
  "segment_options": {
    "segment_duration_sec": 6,
    "support_dash": true,
    "support_hls": true
  },
  "thumbnail_options": {
    "enable_sprite": true,
    "sprite_rows": 5,
    "sprite_cols": 10,
    "thumb_interval_sec": 10,
    "thumb_width": 160,
    "thumb_height": 90,
    "sprite_image_format": "jpg",
    "sprite_storage_prefix": "hvc/thumbnails",
    "enable_binary_index": true,
    "binary_storage_prefix": "hvc/thumbnails/bin",
    "binary_max_size_bytes": 1048576
  },
  "storage_options": {
    "bucket_prefix": "hvc"
  },
  "schedule_options": {
    "preferred_hwaccel": "nvidia"
  },
  "renditions": [
    {
      "name": "1080p",
      "width": 0,
      "height": 1080,
      "video_bitrate_kbps": 5000
    }
  ]
}
```

> MQ / public gRPC 创建任务的字段约束与 HTTP `/v1/transcode/job/create` 完全一致；
> 上述“暂未开放单任务覆盖”的字段同样会被服务端拒绝。

### 8.4 转码完成消息（transcode.job.completed）

```json
{
  "job_id": 1893456789012345678,
  "request_id": "req-001",
  "status": 6,
  "segment_count": 48,
  "duration_ms": 3600000,
  "renditions": ["1080p", "720p", "480p"]
}
```

### 8.5 转码失败消息（transcode.job.failed）

```json
{
  "job_id": 1893456789012345679,
  "request_id": "req-002",
  "error_code": "PROBE_FAILED",
  "error_message": "无法探测源视频信息"
}
```

### 8.6 进度更新消息（transcode.job.progress）

```json
{
  "job_id": 1893456789012345678,
  "stage": "TRANSCODING",
  "progress_permille": 450,
  "current_fps": 120.5,
  "current_bitrate_kbps": 4800.0,
  "current_speed": 4.02
}
```

---

## 九、WebSocket 接口

### 9.1 实时监控

```
ws://localhost:8080/v1/admin/transcode/monitor/ws
Authorization: Bearer {session_token}
```

**协议：**

1. 连接建立后立即发送全量快照（type=snapshot）
2. 每 3 秒推送周期快照（type=snapshot）
3. 每 30 秒发送心跳（type=ping）
4. 客户端发送 type=pong 响应心跳
5. 连接断开后自动清理

**消息格式：**

```json
{
  "type": "snapshot",
  "timestamp": 1715054400000,
  "data": {
    "timestamp": 1715054400000,
    "mode": "standalone",
    "pending_jobs": 5,
    "active_jobs": 3,
    "node_count": 1,
    "system_metrics": {
      "total_active_sessions": 3,
      "total_pending_jobs": 5
    },
    "nodes": [
      {
        "node_id": 1,
        "online": true
      }
    ]
  }
}
```

**心跳消息：**

```json
{
  "type": "ping",
  "timestamp": 1715054400000
}
```

### 9.2 HTTP 快照接口

```
GET /v1/admin/transcode/monitor/snapshot
Authorization: Bearer {session_token}
```

一次性拉取完整监控快照，响应格式同 WebSocket snapshot 的 data 字段。
HTTP 接口外层仍统一包裹 `code/message/data`，其中 `data` 字段结构与 WebSocket `snapshot.data` 一致。

---

## 十、回调载荷格式

回调投递采用标准信封格式，通过 HTTP POST 发送到后台配置的目标地址。

### 10.1 HTTP 回调请求

```
POST {target_url}
Content-Type: application/json
```

### 10.2 回调信封格式

```json
{
  "event_id": 123,
  "event_type": "transcode.completed",
  "job_id": 1893456789012345678,
  "request_id": "req-001",
  "payload_json": "{\"job_id\":1893456789012345678,...}"
}
```

| 字段 | 类型 | 说明 |
|------|------|------|
| event_id | uint64 | 事件ID |
| event_type | string | 事件类型（transcode.completed / transcode.failed） |
| job_id | uint64 | 任务ID |
| request_id | string | 请求ID |
| payload_json | string | 载荷JSON字符串 |

补充说明：
- HTTP 回调使用 `POST {target_url}` 发送上述信封。
- gRPC 回调使用 `grpc://host:port/pkg.Service/Method` 作为目标，传递的仍然是同一份业务载荷。
- MQ 回调使用 `mq://exchange/routing.key` 作为目标，投递的消息体仍然是同一份业务载荷。

### 10.3 完成回调载荷（payload_json 解析后）

```json
{
  "job_id": 1893456789012345678,
  "request_id": "req-001",
  "biz_key": "biz-video-001",
  "source_url": "https://example.com/video.mp4",
  "status": 6,
  "status_name": "已完成",
  "duration_ms": 3600000,
  "segment_duration_sec": 6,
  "support_dash": true,
  "support_hls": true,
  "segment_template": "{job_id}-{rendition_key}-{media_type}-{number}.m4s",
  "storage_type": "s3",
  "storage_bucket": "hvc-media",
  "play_domain": "https://cdn.example.com",
  "source_info": {
    "width": 1920,
    "height": 1080,
    "video_codec": "h264",
    "video_bitrate_kbps": 8000,
    "audio_codec": "aac",
    "audio_bitrate_kbps": 128,
    "fps": 30.0,
    "duration_ms": 3600000
  },
  "renditions": [
    {
      "rendition_name": "1080p",
      "quality_label": "1080p",
      "width": 1920,
      "height": 1080,
      "video_codec": "h264_nvenc",
      "video_bitrate_kbps": 5000,
      "audio_bitrate_kbps": 128,
      "segment_count": 12,
      "init_segment_object_key": "hvc/1893456789012345678-1920_1080-video-0.m4s",
      "manifest_dash_url": "/v1/manifest/dash/1893456789012345678",
      "manifest_hls_url": "/v1/manifest/hls/1893456789012345678",
      "manifest_hls_variant_url": "/v1/manifest/hls/1893456789012345678/1080p"
    }
  ],
  "total_segment_count": 48,
  "total_size_bytes": 5368709120
}
```

### 10.4 失败回调载荷（payload_json 解析后）

```json
{
  "job_id": 1893456789012345679,
  "request_id": "req-002",
  "biz_key": "biz-video-002",
  "source_url": "https://example.com/broken.mp4",
  "status": 7,
  "status_name": "失败",
  "error_code": "PROBE_FAILED",
  "error_message": "无法探测源视频信息",
  "failed_stage": "PROBING",
  "retry_count": 0
}
```

---

## 十一、分片命名模板

### 11.1 占位符定义

| 占位符 | 含义 | 示例值 |
|--------|------|--------|
| `{job_id}` | 任务ID（雪花ID） | `1893456789012345678` |
| `{media_type}` | 媒体类型 | `video` / `audio` |
| `{number}` | 分片序号 | `0`=init, `1/2/3...`=media |
| `{resolution}` | 视频分辨率（宽_高） | `1920_1080` |
| `{quality}` | 清晰度标签 | `1080p` / `720p` / `480p` |
| `{timestamp}` | 分片起始时间戳（毫秒） | `0` / `6000` / `12000` |

### 11.2 清晰度标签映射

| 视频高度 | 标签 |
|---------|------|
| ≥ 2160 | 4k |
| ≥ 1440 | 2k |
| ≥ 1080 | 1080p |
| ≥ 720 | 720p |
| ≥ 480 | 480p |
| ≥ 360 | 360p |
| < 360 | 240p |

### 11.3 自定义模板校验规则

模板必须包含以下三个占位符：`{job_id}`、`{media_type}`、`{number}`

---

## 十二、数据模型

### 12.1 任务状态流转

```
Created(1) → Queued(2) → Assigned(3) → Running(4) → Uploading(5) → Completed(6)
                 ↓            ↓             ↓
             Canceled(8)   Failed(7)     Failed(7)
```

| 状态 | 值 | 中文名 |
|------|---|--------|
| Created | 1 | 已创建 |
| Queued | 2 | 排队中 |
| Assigned | 3 | 已分配 |
| Running | 4 | 转码中 |
| Uploading | 5 | 上传中 |
| Completed | 6 | 已完成 |
| Failed | 7 | 失败 |
| Canceled | 8 | 已取消 |

### 12.2 任务阶段

| 阶段 | 常量值 | 说明 |
|------|--------|------|
| QUEUED | QUEUED | 排队等待 |
| DOWNLOADING | DOWNLOADING | 下载源文件 |
| PROBING | PROBING | 探测源信息 |
| TRANSCODING | TRANSCODING | 转码执行中 |
| UPLOADING | UPLOADING | 分片上传中 |
| FINALIZING | FINALIZING | 收尾处理 |
| COMPLETED | COMPLETED | 已完成 |
| FAILED | FAILED | 已失败 |

### 12.3 分片上传状态

| 状态 | 值 | 说明 |
|------|---|------|
| Pending | 1 | 待上传 |
| Uploading | 2 | 上传中 |
| Uploaded | 3 | 已上传 |
| Failed | 4 | 上传失败 |

### 12.4 硬件加速类型

| 类型 | 常量值 | 说明 |
|------|--------|------|
| software | software | 纯软件编解码 |
| nvidia | nvidia | NVIDIA CUDA/NVENC/NVDEC |
| intel_qsv | intel_qsv | Intel Quick Sync Video |
| amd_amf | amd_amf | AMD Advanced Media Framework |
| vaapi | vaapi | Video Acceleration API (Linux) |
| apple_videotoolbox | apple_videotoolbox | Apple VideoToolbox (macOS) |

### 12.5 直播频道状态

| 状态 | 常量值 | 说明 |
|------|--------|------|
| IDLE | IDLE | 空闲 |
| STARTING | STARTING | 启动中 |
| LIVE | LIVE | 直播中 |
| STOPPED | STOPPED | 已停止 |
| ERROR | ERROR | 异常 |

### 12.6 推流会话状态

| 状态 | 常量值 | 说明 |
|------|--------|------|
| CONNECTING | CONNECTING | 连接中 |
| PUBLISHING | PUBLISHING | 推流中 |
| INTERRUPT_WAIT_RESUME | INTERRUPT_WAIT_RESUME | 中断等待恢复 |
| RESUMED | RESUMED | 已恢复 |
| STOPPED | STOPPED | 已停止 |
| REJECTED | REJECTED | 已拒绝 |

### 12.7 Outbox 事件状态

| 状态 | 值 | 说明 |
|------|---|------|
| Pending | 1 | 待投递 |
| Sending | 2 | 投递中 |
| Delivered | 3 | 已投递 |
| Failed | 4 | 投递失败 |

### 12.8 RBAC 权限点清单

| 权限键 | 说明 |
|--------|------|
| auth.session.write | 会话写操作（登出） |
| auth.session.read | 会话读操作（查看自身信息） |
| config.version.read | 查看配置版本 |
| config.runtime.update | 更新运行配置 |
| config.version.publish | 发布配置版本 |
| config.callback.read | 查看回调配置 |
| config.callback.update | 更新回调配置 |
| config.naming_template.read | 查看命名模板 |
| config.naming_template.update | 更新命名模板 |
| system.user.read | 查看用户列表 |
| system.user.create | 创建/更新用户 |
| system.user.update | 修改用户状态 |
| system.role.read | 查看角色列表 |
| system.role.update | 创建/更新角色 |
| system.permission.read | 查看权限树 |
| system.log.read | 查看运行日志 |
| system.role.permission_bind | 创建/更新权限、绑定角色权限 |
| system.menu.read | 查看菜单树 |
| system.menu.update | 创建/更新菜单 |
| system.menu.delete | 删除菜单 |
| system.role.menu_bind | 绑定角色菜单 |
| system.user.role_bind | 绑定用户角色 |
| cluster.node.read | 查看节点列表/详情 |
| cluster.node.metrics.read | 查看节点指标 |
| cluster.read | 查看集群成员 |
| cluster.node.enable | 启用/禁用节点 |
| cluster.node.quarantine | 隔离/取消隔离节点 |
| cluster.node.drain | 排空/恢复节点 |
| cluster.worker.offline | 下线 Worker |
| cluster.worker.exit | 标记 Worker 退出 |
| transcode.job.read | 查看任务列表/进度 |
| transcode.job.detail.read | 查看任务详情 |
| transcode.job.retry | 重试任务 |
| transcode.job.cancel | 取消任务 |
| live.channel.read | 查看频道详情 |
| live.session.read | 查看直播会话 |
| live.channel.create | 创建频道 |
| live.channel.update | 更新频道 |
| live.channel.start | 启动频道 |
| live.channel.stop | 停止频道 |
| audit.read | 查看审计日志 |

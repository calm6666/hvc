# HVC 视频转码服务 - 完整接口文档

> 版本：2.0.0 | 更新日期：2026-05-07

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
| 集群管理 | 节点注册/心跳/隔离/启用 | 后台管理 + gRPC |
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
  "video_options": {
    "output_aspect_keep": true,
    "aspect_fill_mode": "fit"
  },
  "segment_options": {
    "segment_duration_sec": 6,
    "support_dash": true,
    "support_hls": true,
    "naming_template_id": 2
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
    "storage_id": 0,
    "bucket_prefix": "hvc",
    "segment_prefix": ""
  },
  "schedule_options": {
    "preferred_hwaccel": "nvidia",
    "allow_software_decode_fallback": true,
    "max_wait_seconds": 300
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
GET /v1/manifest/dash/{job_id}.mpd
GET /v1/manifest/dash/{job_id}.mpd?renditions=720p,480p
GET /v1/manifest/dash/{job_id}.mpd?max_height=720
```

| 参数 | 位置 | 说明 |
|------|------|------|
| job_id | path | 任务ID |
| renditions | query | 清晰度白名单（逗号分隔），版权保护用 |
| max_height | query | 最大允许高度，版权保护用 |

**响应：** `Content-Type: application/dash+xml`，`Cache-Control: public, max-age=5`

### 3.5 动态构建 HLS Master

```
GET /v1/manifest/hls/{job_id}.m3u8
GET /v1/manifest/hls/{job_id}.m3u8?renditions=720p,480p
GET /v1/manifest/hls/{job_id}.m3u8?max_height=720
```

**响应：** `Content-Type: application/vnd.apple.mpegurl`，`Cache-Control: public, max-age=5`

### 3.6 动态构建 HLS Variant

```
GET /v1/manifest/hls/{job_id}/{rendition}.m3u8
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
    "status": "LIVE",
    "master_hls_url": "https://cdn.example.com/live/live-001.m3u8",
    "http_flv_url": "https://cdn.example.com/live/live-001.flv",
    "rendition_names": ["1080p", "720p", "480p"]
  }
}
```

---

## 四、HTTP 后台管理接口

> 所有后台接口需要会话认证（Cookie: admin_session={session_token} 或 Authorization: Bearer {session_token}）和 RBAC 权限校验。

### 4.1 认证

| 接口 | 方法 | 说明 |
|------|------|------|
| `/v1/admin/auth/login` | POST | 管理员登录（表单提交），返回 session_token |
| `/v1/admin/auth/logout` | POST | 管理员登出（需认证） |
| `/v1/admin/auth/me` | GET | 获取当前用户信息（需认证） |
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
    "session_token": "eyJhbGciOiJIUzI1NiIs..."
  }
}
```

登录成功后自动设置 Cookie：`admin_session={session_token}; Path=/; HttpOnly; Expires=24h`

**WhoAmI 响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "authenticated": true,
    "admin_user_id": 1
  }
}
```

### 4.2 运行配置管理

说明：

- 运行期业务配置的唯一生效源是数据库中的已发布 `runtime config`
- 本地 `configs/config.yaml` 里的 `scheduler/worker/callback/storage/grpc/mq` 仅在空库首启时用于初始化默认值
- 外部“配置中心”绑定不直接下发运行时业务配置，也不参与运行期热更新

| 接口 | 方法 | 权限 | 说明 |
|------|------|------|------|
| `/v1/admin/config/runtime/versions` | GET | config.version.read | 查询配置版本列表 |
| `/v1/admin/config/runtime/update` | POST | config.runtime.update | 更新运行配置（创建待发布版本） |
| `/v1/admin/config/publish` | POST | config.version.publish | 发布配置版本 |

**更新运行配置请求：**

```json
{
  "enable_http_server": true,
  "enable_callback": true,
  "default_profile_id": 0,
  "max_global_transcode_sessions": 100,
  "job_lease_ttl_sec": 300,
  "worker_heartbeat_timeout_sec": 30,
  "allow_request_override_profile": true,
  "allow_request_override_segment_duration": true,
  "allow_request_override_hwaccel": true,
  "scheduler_loop_interval_ms": 1000,
  "worker_loop_interval_ms": 1000,
  "single_job_upload_concurrency": 4,
  "require_hardware_encode": false,
  "allow_software_decode_fallback": true,
  "require_hardware_watermark": false,
  "node_cpu_safety_limit_percent": 85,
  "node_memory_safety_limit_percent": 85,
  "node_gpu_safety_limit_percent": 90,
  "callback_http_url": "https://callback.example.com/transcode",
  "callback_mq_topic": "transcode.job.callback",
  "enable_grpc_server": true,
  "grpc_listen_address": ":9090",
  "enable_mq_consumer": true,
  "mq_queue_name": "hvc.transcode.create",
  "mq_host": "127.0.0.1",
  "mq_port": 5672,
  "mq_username": "guest",
  "mq_password": "guest",
  "mq_vhost": "/",
  "mq_consumer_tag": "hvc-runtime",
  "mq_prefetch_count": 16,
  "mq_loop_interval_ms": 15000,
  "worker_object_prefix": "hvc/runtime",
  "change_summary": "增加全局并发数"
}
```

发布后生效范围：

- `scheduler` / `worker` / `callback` 运行参数会按最新生效配置热更新
- `HTTP` 对外入口、`WebSocket` 监控入口跟随 `enable_http_server` 热启停
- `callback` 投递模块支持通过 `enable_callback` 热启停
- 对外 `gRPC` 服务支持动态启停与监听地址切换
- `MQ` 创建任务消费者支持动态启停与队列/连接参数切换
- 已在运行中的转码任务不会被强制中断，但后续轮询、上传、回调和新任务派发会按新配置执行

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

### 4.3 回调配置管理

| 接口 | 方法 | 权限 | 说明 |
|------|------|------|------|
| `/v1/admin/config/callback/list` | GET | config.callback.read | 查询回调配置列表 |
| `/v1/admin/config/callback/upsert` | POST | config.callback.update | 创建/更新回调配置 |
| `/v1/admin/config/callback/enabled` | POST | config.callback.update | 启用/禁用回调 |

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
| 2 | gRPC 回调（调用 rpc_endpoint） |
| 3 | MQ 回调（发送到 mq_exchange + mq_routing_key） |

说明：
- 系统级回调目标通过 `/v1/admin/config/callback/upsert` 和 `/v1/admin/config/callback/enabled` 保存后立即生效，无需发布运行时配置版本。
- 单任务 `callback_url` 优先级高于系统级回调配置。
- 单次投递失败会写入补偿记录，记录中带具体 `callback_target`，便于排障和审计。

### 4.4 命名模板管理

| 接口 | 方法 | 权限 | 说明 |
|------|------|------|------|
| `/v1/admin/config/naming-template/list` | GET | config.naming_template.read | 查询6种预置模板+当前生效模板 |
| `/v1/admin/config/naming-template/configure` | POST | config.naming_template.update | 配置模板（待发布） |
| `/v1/admin/config/naming-template/activate` | POST | config.naming_template.update | 立即生效模板 |

**配置模板请求：**

```json
{
  "template_id": 2,
  "custom_template": ""
}
```

| template_id | 模板 | 示例（video init） |
|-------------|------|-------------------|
| 1 | `{job_id}-{media_type}-{number}.m4s` | `1893456789012345678-video-0.m4s` |
| 2 | `{job_id}-{resolution}-{media_type}-{number}.m4s` | `1893456789012345678-1920_1080-video-0.m4s` |
| 3 | `{job_id}-{quality}-{media_type}-{number}.m4s` | `1893456789012345678-1080p-video-0.m4s` |
| 4 | `{job_id}-{media_type}-{number}-{timestamp}.m4s` | `1893456789012345678-video-0-0.m4s` |
| 5 | `{job_id}-{resolution}-{media_type}-{number}-{timestamp}.m4s` | `1893456789012345678-1920_1080-video-0-0.m4s` |
| 6 | `{job_id}-{quality}-{media_type}-{number}-{timestamp}.m4s` | `1893456789012345678-1080p-video-0-0.m4s` |

**立即生效请求：**

```json
{
  "template": "{job_id}-{resolution}-{media_type}-{number}.m4s"
}
```

**立即生效响应：**

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "template": "{job_id}-{resolution}-{media_type}-{number}.m4s",
    "published": true,
    "effective_scope": "新提交的转码任务",
    "running_jobs": "不受影响，继续使用原模板"
  }
}
```

### 4.5 配置中心绑定（Bootstrap 基础配置源）

| 接口 | 方法 | 权限 | 说明 |
|------|------|------|------|
| `/v1/admin/config-center/list` | GET | config.version.read | 查询 bootstrap 配置源绑定列表 |
| `/v1/admin/config-center/upsert` | POST | config.version.publish | 创建/更新 bootstrap 配置源绑定 |
| `/v1/admin/config-center/enabled` | POST | config.version.publish | 启用/禁用 bootstrap 配置源绑定 |

说明：

- 该组接口用于维护外部 bootstrap 配置源元数据，例如 MySQL/集群注册/节点身份这类启动基础配置来源
- 该组接口不直接发布 `scheduler/worker/callback/http/grpc/mq` 等运行期业务配置
- 运行期业务配置仍通过 `/v1/admin/config/runtime/update` + `/v1/admin/config/publish` 生效

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
| `/v1/admin/system/user/list` | GET | system.user.read | 用户列表 |
| `/v1/admin/system/user/upsert` | POST | system.user.create | 创建/更新用户 |
| `/v1/admin/system/user/status` | POST | system.user.update | 启用/禁用用户 |
| `/v1/admin/system/role/list` | GET | system.role.read | 角色列表 |
| `/v1/admin/system/role/upsert` | POST | system.role.update | 创建/更新角色 |
| `/v1/admin/system/permission/list` | GET | system.permission.read | 权限列表 |
| `/v1/admin/system/permission/upsert` | POST | system.role.permission_bind | 创建/更新权限 |
| `/v1/admin/system/user-role/bind` | POST | system.user.role_bind | 绑定用户角色 |
| `/v1/admin/system/role-permission/bind` | POST | system.role.permission_bind | 绑定角色权限 |

**创建/更新用户请求：**

```json
{
  "username": "operator",
  "password_hash": "$2a$10$...",
  "display_name": "运维人员",
  "status": 1
}
```

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
  "user_id": 2,
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

### 4.7 集群节点管理

| 接口 | 方法 | 权限 | 说明 |
|------|------|------|------|
| `/v1/admin/cluster/node/list` | GET | cluster.node.read | 节点列表 |
| `/v1/admin/cluster/node/detail` | GET | cluster.node.read | 节点详情（JSON Body） |
| `/v1/admin/cluster/node/metrics` | GET | cluster.node.metrics.read | 节点指标 |
| `/v1/admin/cluster/member/list` | GET | cluster.read | 集群成员列表 |
| `/v1/admin/cluster/node/enabled` | POST | cluster.node.enable | 启用/禁用节点 |
| `/v1/admin/cluster/node/quarantined` | POST | cluster.node.quarantine | 隔离/取消隔离节点 |

**节点详情请求（GET + JSON Body）：**

```json
{
  "node_id": 1
}
```

**启用/禁用节点请求：**

```json
{
  "node_id": 1,
  "enabled": true
}
```

**隔离/取消隔离节点请求：**

```json
{
  "node_id": 1,
  "quarantined": true,
  "reason": "GPU故障维护"
}
```

### 4.8 转码任务管理

| 接口 | 方法 | 权限 | 说明 |
|------|------|------|------|
| `/v1/admin/transcode/job/list` | GET | transcode.job.read | 任务列表（分页+筛选） |
| `/v1/admin/transcode/job/detail` | GET | transcode.job.detail.read | 任务详情 |
| `/v1/admin/transcode/job/progress` | GET | transcode.job.read | 后台进度查询（含实时信息） |
| `/v1/admin/transcode/job/retry` | POST | transcode.job.retry | 重试失败任务 |
| `/v1/admin/transcode/job/cancel` | POST | transcode.job.cancel | 取消任务 |

**任务列表查询参数：** `page`, `page_size`, `status`, `biz_key`

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
| `/v1/admin/live/channel/detail` | GET | live.channel.read | 频道详情 |
| `/v1/admin/live/channel/update` | POST | live.channel.update | 更新频道 |
| `/v1/admin/live/channel/start` | POST | live.channel.start | 启动频道 |
| `/v1/admin/live/channel/stop` | POST | live.channel.stop | 停止频道 |

**创建频道请求：**

```json
{
  "channel_key": "live-001",
  "channel_name": "测试频道",
  "profile_id": 0
}
```

**频道详情查询参数：** `channel_id` 或 `channel_key`

**更新频道请求：**

```json
{
  "channel_id": 1,
  "channel_name": "新名称",
  "enable_watermark": true
}
```

**启动频道请求：**

```json
{
  "channel_id": 1,
  "node_id": 0,
  "worker_id": ""
}
```

**停止频道请求：**

```json
{
  "channel_id": 1
}
```

### 4.10 审计日志

| 接口 | 方法 | 权限 | 说明 |
|------|------|------|------|
| `/v1/admin/audit/list` | GET | audit.read | 查询审计日志 |

### 4.11 WebSocket 实时监控

```
GET /v1/admin/transcode/monitor/ws
Upgrade: websocket
Authorization: Bearer {session_token}
```

连接后接收实时转码进度快照推送。

### 4.12 HTTP 快照接口

```
GET /v1/admin/transcode/monitor/snapshot
Authorization: Bearer {session_token}
```

一次性拉取完整监控快照。
注意：该接口直接返回原始 snapshot JSON，不包裹 `code/message/data` 通用响应结构。

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
  "video_options": {
    "output_aspect_keep": true,
    "aspect_fill_mode": "fit"
  },
  "segment_options": {
    "segment_duration_sec": 6,
    "support_dash": true,
    "support_hls": true,
    "naming_template_id": 2
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
    "storage_id": 0,
    "bucket_prefix": "hvc",
    "segment_prefix": ""
  },
  "schedule_options": {
    "preferred_hwaccel": "nvidia",
    "allow_software_decode_fallback": true,
    "max_wait_seconds": 300
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
  "segment_template": "{job_id}-{resolution}-{media_type}-{number}.m4s",
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
      "manifest_dash_url": "/v1/manifest/dash/1893456789012345678.mpd",
      "manifest_hls_url": "/v1/manifest/hls/1893456789012345678.m3u8",
      "manifest_hls_variant_url": "/v1/manifest/hls/1893456789012345678/1080p.m3u8"
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
| system.permission.read | 查看权限列表 |
| system.role.permission_bind | 创建/更新权限、绑定角色权限 |
| system.user.role_bind | 绑定用户角色 |
| cluster.node.read | 查看节点列表/详情 |
| cluster.node.metrics.read | 查看节点指标 |
| cluster.read | 查看集群成员 |
| cluster.node.enable | 启用/禁用节点 |
| cluster.node.quarantine | 隔离/取消隔离节点 |
| transcode.job.read | 查看任务列表/进度 |
| transcode.job.detail.read | 查看任务详情 |
| transcode.job.retry | 重试任务 |
| transcode.job.cancel | 取消任务 |
| live.channel.read | 查看频道详情 |
| live.channel.create | 创建频道 |
| live.channel.update | 更新频道 |
| live.channel.start | 启动频道 |
| live.channel.stop | 停止频道 |
| audit.read | 查看审计日志 |

Nginx 部署说明

- `hvc.conf` 提供生产向入口示例，覆盖 HTTP API、管理后台、WebSocket 监控和 public gRPC
- `hvc.docker.conf` 提供 Docker 单机演示入口，不依赖证书文件，可直接被 compose 挂载
- `hvc.conf` 已把 `limit_req_zone` 放在文件顶层 `http` 级别上下文，避免 Nginx 因指令位置错误启动失败
- 健康检查入口统一代理到 `/healthz`
- 集群模式下可按节点能力继续扩展 `upstream` 列表
- 对外 public gRPC 建议统一走负载均衡入口；节点自身通过 runtime config + etcd 完成注册，Nginx 负责北向接入稳定地址

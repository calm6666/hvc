Nginx 部署说明

- `hvc.conf` 提供 HTTP API、管理后台、WebSocket 监控和内部 gRPC 的反向代理示例
- 健康检查入口统一代理到 `/healthz`
- 集群模式下可按节点能力继续扩展 `upstream` 列表

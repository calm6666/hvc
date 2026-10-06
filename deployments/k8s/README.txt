Kubernetes 部署说明

- 本示例使用 `ConfigMap` 挂载 `/etc/hvc/config.yaml`
- 业务动态配置不放在 Deployment 里，而是通过后台 runtime config 接口维护
- control-plane 健康检查使用 HTTP `/healthz`
- worker 健康检查建议直接探测 `internal_grpc` 监听端口 `19090`
- `config.yaml` 中的 `server.node_mode` 明确声明节点角色
- `deployment.yaml` 是 control-plane 单副本示例
- `worker-deployment.yaml` 是 worker 单副本示例
- 当前架构下 `node_id` / `worker_id` 必须唯一，不能直接把同一份 Deployment 扩成多个共享相同 `node_id` 的副本
- 如果需要多副本稳定标识，请改用 `StatefulSet + 每 Pod 唯一 bootstrap 配置`
- 因为当前示例是固定 `node_id` 的 control-plane 单副本模板，所以不再附带 HPA，避免误扩容出重复节点身份

部署命令：

- `kubectl apply -f deployments/k8s/deployment.yaml`
- `kubectl apply -f deployments/k8s/worker-deployment.yaml`
- `kubectl get pods -n hvc`
- `kubectl logs -f deployment/hvc-server -n hvc`

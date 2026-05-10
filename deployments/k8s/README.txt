Kubernetes 部署说明

- 本示例使用 `ConfigMap` 挂载 `/etc/hvc/config.yaml`
- 业务动态配置不放在 Deployment 里，而是通过后台 runtime config 接口维护
- 健康检查统一使用 `/healthz`

部署命令：

- `kubectl apply -f deployments/k8s/deployment.yaml`
- `kubectl get pods -n hvc`
- `kubectl logs -f deployment/hvc-server -n hvc`

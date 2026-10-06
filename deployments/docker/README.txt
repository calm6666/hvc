Docker 部署说明

- `Dockerfile` 负责编译 `./cmd` 并生成运行镜像
- 容器内默认通过 `--config /etc/hvc/config.yaml` 加载 bootstrap 配置
- `docker-compose.yml` 首次启动只会向 MySQL 导入 `sql/000_full_project_schema.sql`
- `docker-compose.yml` 是单机演示栈：`hvc-server + mysql + redis + etcd + rabbitmq + minio + nginx`
- Compose 示例默认更适合 `standalone`；如果改成 `cluster-allinone` 也能演示一体化节点，但不等价于真实多机集群
- MinIO bucket 会通过 `minio-init` 容器自动创建 `hvc-media`
- Compose 使用专用 bootstrap 配置文件 `deployments/docker/config.compose.yaml`，不会直接复用仓库根的本机示例配置

常用命令：

- `docker compose -f deployments/docker/docker-compose.yml up -d`
- `docker compose -f deployments/docker/docker-compose.yml logs -f hvc-server`
- `docker compose -f deployments/docker/docker-compose.yml down`
- 单机 HTTP 统一入口：`http://127.0.0.1:8088`
- 单机 public gRPC 统一入口：`127.0.0.1:9091`

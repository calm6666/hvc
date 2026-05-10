Docker 部署说明

- `Dockerfile` 负责编译 `./cmd` 并生成运行镜像
- 容器内默认通过 `--config /etc/hvc/config.yaml` 加载 bootstrap 配置
- `docker-compose.yml` 会把仓库根目录下的 `sql/` 挂载到 MySQL 初始化目录，首次启动自动建库建表

常用命令：

- `docker compose -f deployments/docker/docker-compose.yml up -d`
- `docker compose -f deployments/docker/docker-compose.yml logs -f hvc-server`
- `docker compose -f deployments/docker/docker-compose.yml down`

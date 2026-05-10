发布脚本说明

- `release.sh` 会编译多平台二进制、构建 Docker 镜像、生成发布包
- Go 入口统一使用 `./cmd`
- 发布包中会携带 `configs/`、`deployments/`、`scripts/`

示例：

- `./scripts/release/release.sh v1.0.0`
- `./scripts/release/release.sh v1.0.0 --skip-push`

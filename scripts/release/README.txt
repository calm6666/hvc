发布脚本说明

- `release.sh` 会编译多平台二进制、构建 Docker 镜像、生成发布包
- Go 入口统一使用 `./cmd`
- 发布包中会携带 `configs/`、`deployments/`、`scripts/`、`sql/` 以及 `DEPLOYMENT_GUIDE.md`
- 只有在需要创建 Git Tag 时才要求工作目录干净；`--skip-tag` 时允许在本地修改状态下生成发布包

示例：

- `./scripts/release/release.sh v1.0.0`
- `./scripts/release/release.sh v1.0.0 --skip-push`

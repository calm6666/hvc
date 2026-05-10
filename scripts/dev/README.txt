开发脚本说明

- `dev.sh` 使用仓库根目录的 `.air.toml` 启动热重载开发环境
- 启动配置路径通过 `HVC_CONFIG_PATH` 传入主程序
- 如果本机没有 `air`，脚本会先执行 `go install github.com/air-verse/air@latest`

示例：

- `./scripts/dev/dev.sh`
- `./scripts/dev/dev.sh ./configs/config.yaml`

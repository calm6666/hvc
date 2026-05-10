systemd 部署说明

- `hvc-server.service` 使用 `ExecStart=/usr/local/bin/hvc-server --config /etc/hvc/config.yaml`
- 同时保留 `HVC_CONFIG_PATH=/etc/hvc/config.yaml` 作为环境变量入口，便于统一脚本习惯
- 服务日志进入 journald，应用自身仍会写入工作目录下的 `log/YYYY-MM-DD.log`

部署步骤：

- `sudo cp deployments/systemd/hvc-server.service /etc/systemd/system/`
- `sudo systemctl daemon-reload`
- `sudo systemctl enable --now hvc-server`

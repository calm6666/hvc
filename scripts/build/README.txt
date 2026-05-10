构建脚本说明

- `build.sh` 负责编译当前项目唯一 Go 入口 `./cmd`
- 默认输出目录为仓库根目录下的 `build/`
- 支持通过参数指定目标平台和版本号

示例：

- `./scripts/build/build.sh`
- `./scripts/build/build.sh linux amd64`
- `./scripts/build/build.sh windows amd64 v1.2.3`

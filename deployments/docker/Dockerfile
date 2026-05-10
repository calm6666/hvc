# ============================================================
# HVC 视频转码服务 - 多阶段构建 Dockerfile
# ============================================================
#
# 构建流程：
#   1. builder 阶段：编译 Go 二进制
#   2. runtime 阶段：最小化运行时镜像，包含 FFmpeg
#
# 使用方法：
#   docker build -t hvc-server:latest -f deployments/docker/Dockerfile .
#
# 运行：
#   docker run -p 8080:8080 -p 9090:9090 \
#     -v $(pwd)/configs/config.yaml:/etc/hvc/config.yaml:ro \
#     hvc-server:latest
# ============================================================

# ---- 构建阶段 ----
FROM golang:1.22-bookworm AS builder

# 设置工作目录
WORKDIR /app

# 先复制依赖文件，利用 Docker 缓存层加速构建
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码并编译
# CGO_ENABLED=0 禁用 CGO，生成纯静态二进制
# -s -w 去除调试信息，减小二进制体积
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-s -w" -o /hvc-server ./cmd

# ---- 运行阶段 ----
FROM debian:bookworm-slim

# 安装运行时依赖：
#   - ffmpeg：视频转码核心依赖
#   - ca-certificates：HTTPS 证书
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
    ffmpeg \
    curl \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# 从构建阶段复制二进制文件
COPY --from=builder /hvc-server /usr/local/bin/hvc-server

# 复制默认配置文件（生产环境应通过 Volume 挂载覆盖）
COPY configs/config.yaml /etc/hvc/config.yaml

# HTTP 服务端口（公共 API + 管理后台）
EXPOSE 8080
# 对外 public gRPC 端口（运行期可热启停）
EXPOSE 9090
# 集群内部 gRPC 端口（bootstrap 固定初始化）
EXPOSE 19090

# 启动命令
ENTRYPOINT ["hvc-server"]
CMD ["--config", "/etc/hvc/config.yaml"]

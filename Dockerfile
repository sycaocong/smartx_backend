# 构建阶段
FROM golang:1.21-alpine AS builder

# 安装构建依赖
RUN apk add --no-cache git ca-certificates tzdata

# 设置工作目录
WORKDIR /app

# 复制go mod文件
COPY go.mod go.sum ./
RUN go mod download

# 复制源代码
COPY . .

# 设置Go编译选项
ENV CGO_ENABLED=0
ENV GOOS=linux
ENV GOARCH=amd64

# 编译二进制文件
RUN go build -ldflags="-w -s" -o /server ./cmd/server

# 运行时阶段
FROM alpine:3.19

# 安装运行时依赖
RUN apk add --no-cache ca-certificates tzdata

# 创建非root用户
RUN addgroup -g 1000 appgroup && \
    adduser -u 1000 -G appgroup -s /bin/sh -D appuser

# 设置工作目录
WORKDIR /app

# 复制二进制文件
COPY --from=builder /server .

# 复制配置文件
COPY config.toml .

# 复制健康检查脚本
COPY docker-entrypoint.sh .

# 修改文件权限
RUN chmod +x docker-entrypoint.sh

# 切换到非root用户
USER appuser

# 暴露端口
EXPOSE 8080 9090

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# 启动命令
ENTRYPOINT ["/app/docker-entrypoint.sh"]

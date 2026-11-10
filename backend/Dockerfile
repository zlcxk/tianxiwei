# 1. 基础镜像：如拉取困难，可改用国内同步源（示例见下方说明）
FROM golang:1.26-alpine AS build
ARG APP_VERSION=dev
WORKDIR /src

# 2. 配置 Go 模块代理（国内加速核心）
ENV GOPROXY=https://goproxy.cn,direct
ENV GOSUMDB=sum.golang.google.cn

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath \
      -ldflags="-s -w -X xiweispace/backend/internal/collector.Version=${APP_VERSION}" -o /out/api ./cmd/api \
 && CGO_ENABLED=0 GOOS=linux go build -trimpath \
      -ldflags="-s -w -X xiweispace/backend/internal/collector.Version=${APP_VERSION}" -o /out/worker ./cmd/worker

FROM alpine:3.22

# 3. 替换 Alpine 软件源为国内镜像，加速 apk add
RUN sed -i 's/dl-cdn.alpinelinux.org/mirrors.ustc.edu.cn/g' /etc/apk/repositories \
 && apk add --no-cache ca-certificates tzdata \
 && addgroup -S xiwei \
 && adduser -S -G xiwei -h /app xiwei

WORKDIR /app
COPY --from=build /out/api /app/api
COPY --from=build /out/worker /app/worker

USER xiwei
EXPOSE 8080

# 必须使用 CMD 而不是 ENTRYPOINT：docker-compose 中 worker 服务通过
# command: ["/app/worker"] 覆盖默认命令。如果这里写 ENTRYPOINT，worker
# 容器实际会执行 "/app/api /app/worker"，导致 Worker 进程从未启动、
# 所有任务永远停留在 queued。
CMD ["/app/api"]
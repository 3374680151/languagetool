# FluentRead-Go 生产镜像（P7，NFR-4/12）。
#
# @file deploy/Dockerfile
# 文件职责：多阶段构建——① 前端（node → web/dist，vue-tsc + vite）；② 后端
#（CGO_ENABLED=0 静态二进制，web/dist 由 go:embed 打进程序）；③ 精简运行镜像。
# 应用单容器独立可跑（不依赖 LiteLLM，DR-9：LiteLLM 是可选外挂）。
#
# 主要内容：三个构建阶段与运行时配置。
#
# 模块边界：构建期需要网络拉依赖；运行期只需要一个可写的 SQLite 目录。

# 基础镜像可经构建参数替换（受限网络下用镜像源，如
# docker build --build-arg GO_IMAGE=docker.m.daocloud.io/library/golang:1.25-alpine ...）。
ARG NODE_IMAGE=node:22-alpine
ARG GO_IMAGE=golang:1.25-alpine

# ---------- 阶段 1：前端 ----------
FROM ${NODE_IMAGE} AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci
COPY web ./
RUN npm run build

# ---------- 阶段 2：后端 ----------
FROM ${GO_IMAGE} AS build
WORKDIR /src

# 先拷依赖清单，利用层缓存
COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
# web/embed.go 用 go:embed 引用 web/dist，缺了这行编译会直接失败。
COPY web ./web
# 用阶段 1 的产物覆盖 dist（确保二进制里是刚构建的前端）。
COPY --from=web /web/dist ./web/dist

# CGO_ENABLED=0 才能得到不依赖 libc 的静态二进制（NFR-5/DR-3）。
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/fluentread \
    ./cmd/server

# ---------- 阶段 3：运行 ----------
FROM alpine:3.20
RUN adduser -D -H -u 10001 app && mkdir -p /app/data && chown -R app:app /app
WORKDIR /app

COPY --from=build /out/fluentread /app/fluentread

USER app
EXPOSE 8787
VOLUME ["/app/data"]

# FR_LISTEN 默认 :8787（附录 A）。
ENTRYPOINT ["/app/fluentread"]

# 多阶段构建：Go 编译 → 精简运行镜像。
# P2 起前端由 go:embed 打进同一个二进制，所以这里只需要把 web/dist 拷进来，
# 不需要额外的 nginx 容器。前端构建（npm run build）留到 P7 加一个 node 阶段，
# 目前由 `make web` 在本地产出 web/dist 后再 build 镜像。
# 注意：modernc.org/sqlite 是纯 Go 实现，因此可以关掉 CGO，产物是静态二进制。

FROM golang:1.25-alpine AS build
WORKDIR /src

# 先拷依赖清单，利用层缓存
COPY go.mod go.sum ./
RUN go mod download

COPY cmd ./cmd
COPY internal ./internal
# web/embed.go 用 go:embed 引用 web/dist，缺了这行编译会直接失败。
COPY web ./web

# CGO_ENABLED=0 才能得到不依赖 libc 的静态二进制
RUN CGO_ENABLED=0 GOOS=linux go build \
    -trimpath \
    -ldflags="-s -w" \
    -o /out/fluentread \
    ./cmd/server

FROM alpine:3.20
RUN adduser -D -H -u 10001 app && mkdir -p /app/data && chown -R app:app /app
WORKDIR /app

COPY --from=build /out/fluentread /app/fluentread

USER app
EXPOSE 8080
VOLUME ["/app/data"]

ENTRYPOINT ["/app/fluentread"]

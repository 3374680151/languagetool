# 常用命令。默认目标是 help。
.PHONY: help run build test fmt vet verify verify-p1 verify-p1.5 verify-p2 up down logs health clean

help:
	@echo "make run        本地启动（go run）"
	@echo "make build      编译到 bin/fluentread"
	@echo "make test       跑全部测试"
	@echo "make fmt        格式化"
	@echo "make vet        静态检查"
	@echo "make verify     全量回归：P1 链路 + P1.5 护栏 + P2 写作台"
	@echo "make verify-p1  P1 翻译链路端到端"
	@echo "make verify-p1.5 P1.5 护栏（配额/限流/降级）"
	@echo "make verify-p2  P2 写作台（构建 + 出口标准）"
	@echo "make web        构建前端到 web/dist"
	@echo "make up         docker compose 起 web + litellm"
	@echo "make down       停止"
	@echo "make logs       跟踪 web 日志"
	@echo "make health     打健康检查"

run:
	go run ./cmd/server

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/fluentread ./cmd/server

web:
	cd web && npm run build

verify: verify-p1 verify-p1.5 verify-p2

verify-p1: build
	./scripts/verify-p1.sh

verify-p1.5: build
	./scripts/verify-p1.5.sh

verify-p2: build
	./scripts/verify-p2.sh

test:
	go test ./...

fmt:
	go fmt ./...

vet:
	go vet ./...

up:
	docker compose -f deploy/docker-compose.yml up -d --build

down:
	docker compose -f deploy/docker-compose.yml down

logs:
	docker compose -f deploy/docker-compose.yml logs -f web

health:
	@curl -s http://127.0.0.1:8080/api/health | python3 -m json.tool

clean:
	rm -rf bin data

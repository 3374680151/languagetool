# 常用命令。默认目标是 help。
#
# @file Makefile
# 文件职责：开发与验证的命令入口——构建、门禁、各期 verify、部署。
# 每期的 verify-*.sh 是该期出口标准的可复现门禁（证据落 docs/acceptance/）。
.PHONY: help run build web test vet verify verify-p0 verify-p1 verify-p1.5 verify-p2 verify-p3 verify-p3.5 verify-p4 verify-p5 verify-p5.5 verify-p6 verify-p7 verify-all up down logs health clean

help:
	@echo "make run           本地启动（go run，默认 :8787）"
	@echo "make build         CGO_ENABLED=0 编译到 bin/fluentread"
	@echo "make web           构建前端到 web/dist（vue-tsc + vite）"
	@echo "make test          go test ./..."
	@echo "make vet           go vet ./..."
	@echo "make verify-all    全量门禁：P0..P6 各期 verify 脚本依次执行"
	@echo "make verify-pN     单期出口标准门禁（证据落 docs/acceptance/PN/）"
	@echo "make verify-p7     部署门禁：docker compose 构建 + 单容器 health 三态"
	@echo "make up            docker compose 起 app（单容器）"
	@echo "make up-litellm    docker compose --profile litellm 起 app+网关"
	@echo "make down          停止"
	@echo "make logs          跟踪 app 日志"
	@echo "make health        打健康检查（:8787）"
	@echo "make clean         清理 bin 与 data"

run:
	go run ./cmd/server

build:
	CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/fluentread ./cmd/server
	go build -o bin/mockupstream ./cmd/mockupstream

web:
	cd web && npm run build

test:
	go test ./...

vet:
	go vet ./...

verify-p0: build
	./scripts/verify-p0.sh
verify-p1: build
	./scripts/verify-p1.sh
verify-p1.5: build
	./scripts/verify-p1.5.sh
verify-p2: build
	./scripts/verify-p2.sh
verify-p3: build
	./scripts/verify-p3.sh
verify-p3.5: build
	./scripts/verify-p3.5.sh
verify-p4: build
	./scripts/verify-p4.sh
verify-p5: build
	./scripts/verify-p5.sh
verify-p5.5: build
	./scripts/verify-p5.5.sh
verify-p6: build
	./scripts/verify-p6.sh
verify-p7: build
	./scripts/verify-p7.sh

verify-all: verify-p0 verify-p1 verify-p1.5 verify-p2 verify-p3 verify-p3.5 verify-p4 verify-p5 verify-p5.5 verify-p6 verify-p7

up:
	docker compose -f deploy/docker-compose.yml up -d --build

up-litellm:
	docker compose -f deploy/docker-compose.yml --profile litellm up -d --build

down:
	docker compose -f deploy/docker-compose.yml --profile litellm down

logs:
	docker compose -f deploy/docker-compose.yml logs -f app

health:
	@curl -s http://127.0.0.1:8787/api/health | python3 -m json.tool

clean:
	rm -rf bin data

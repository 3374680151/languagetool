#!/usr/bin/env bash
# @file scripts/r0-walkthrough.sh
# R0 体验走查的环境准备：构建 → 起假网关 + 一个正常配置的后端 → 跑 Playwright 走查。
#
# 与 verify-p2.sh 的区别：这里只起**一个**正常实例，因为走查的是"用户正常用"，
# 不是边界态。证据落到 docs/evidence/R0/。
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1

free_port() { python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()'; }

PIDS=()
cleanup() {
  for p in "${PIDS[@]:-}"; do [ -n "$p" ] && kill "$p" 2>/dev/null; done
  wait 2>/dev/null
}
trap cleanup EXIT

echo "=== 构建 ==="
go build -o bin/fake-gateway ./scripts/fake-gateway || { echo "!! 假网关编译失败"; exit 1; }
find web/dist -mindepth 1 -delete 2>/dev/null || true
(cd web && npm run build > /tmp/fr-web-build.log 2>&1) || { echo "!! 前端构建失败"; tail -20 /tmp/fr-web-build.log; exit 1; }
make build > /dev/null 2>&1 || { echo "!! 后端编译失败"; exit 1; }

echo "=== 起服务 ==="
GW_PORT=$(free_port)
./bin/fake-gateway -addr "127.0.0.1:$GW_PORT" -models deploy/models.json > /tmp/fr-r0-gw.log 2>&1 &
PIDS+=("$!")
for _ in $(seq 1 40); do curl -s --noproxy '*' -o /dev/null "http://127.0.0.1:$GW_PORT/v1/models" && break; sleep 0.2; done

PORT=$(free_port)
env FR_ADDR="127.0.0.1:$PORT" \
    FR_DB_PATH="$ROOT/data/r0.db" \
    FR_GATEWAY_BASE="http://127.0.0.1:$GW_PORT" \
    FR_GATEWAY_KEY=test-key \
    FR_DEFAULT_MODEL=deepseek-chat \
    FR_MYMEMORY_BASE="http://127.0.0.1:$GW_PORT/get" \
    FR_QUOTA_DAILY=1000 FR_RATE_LIMIT=1000 \
    ./bin/fluentread > /tmp/fr-r0-server.log 2>&1 &
PIDS+=("$!")
for _ in $(seq 1 80); do curl -s --noproxy '*' -o /dev/null "http://127.0.0.1:$PORT/api/health" && break; sleep 0.2; done
curl -s --noproxy '*' -o /dev/null "http://127.0.0.1:$PORT/api/health" || {
  echo "!! 服务未就绪"; cat /tmp/fr-r0-server.log; exit 1; }
echo "  写作台 → http://127.0.0.1:$PORT"

# 截图必须落在子目录里：EVIDENCE_DIR 会作为 Playwright 的 outputDir，
# 而 Playwright 每次开跑前**清空 outputDir**。早先把 report.md 和截图放同一层，
# 重跑一次就把上一轮的报告删了——证据目录里只剩截图，看起来像"没写过报告"。
EVIDENCE_DIR="$ROOT/docs/evidence/R0/shots"
find "$EVIDENCE_DIR" -mindepth 1 -delete 2>/dev/null || true
mkdir -p "$EVIDENCE_DIR"

echo "=== 走查 ==="
cd web || exit 1
if [ -z "${PW_CHROMIUM:-}" ]; then
  for cand in "$HOME"/Library/Caches/ms-playwright/chromium_headless_shell-*/chrome-headless-shell-mac-arm64/chrome-headless-shell; do
    [ -x "$cand" ] && PW_CHROMIUM="$cand"
  done
  for cand in "$HOME"/Library/Caches/ms-playwright/chromium-*/chrome-mac-arm64/Chromium.app/Contents/MacOS/Chromium; do
    [ -z "${PW_CHROMIUM:-}" ] && [ -x "$cand" ] && PW_CHROMIUM="$cand"
  done
fi

BASE_URL="http://127.0.0.1:$PORT" \
EVIDENCE_DIR="$EVIDENCE_DIR" \
PW_CHROMIUM="${PW_CHROMIUM:-}" \
npx playwright test e2e/r0-walkthrough.spec.ts 2>&1 | tail -80

echo
echo "截图：docs/evidence/R0/shots/（报告写在 docs/evidence/R0/report.md，不会被重跑清掉）"
ls "$EVIDENCE_DIR" 2>/dev/null | head -30

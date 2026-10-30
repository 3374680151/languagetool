#!/usr/bin/env bash
# @file scripts/verify-p0.sh
# P0 骨架出口标准验证（NFR-14 门禁 + e2e 冒烟）。
#
# 做法：起 mockupstream（假上游）+ 正常实例 + 未配置实例，
# 跑 Go 测试、CGO_ENABLED=0 构建、前端构建（含 vue-tsc）、
# health 三态断言与 Playwright e2e，证据写 docs/acceptance/P0/evidence/。
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1
EV="docs/acceptance/P0/evidence"
mkdir -p "$EV" /tmp/fr-p0
: > "$EV/gates.txt"  # 截断上次运行的门禁记录（每次运行独立留证）

FAIL=0
pass() { echo "  PASS  $1"; echo "PASS  $1" >> "$EV/gates.txt"; }
fail() { echo "  FAIL  $1"; echo "FAIL  $1" >> "$EV/gates.txt"; FAIL=1; }
free_port() { python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()'; }

PIDS=()
cleanup() {
  for p in "${PIDS[@]:-}"; do [ -n "$p" ] && kill "$p" 2>/dev/null; done
  wait 2>/dev/null
}
trap cleanup EXIT

start_server() {
  local name="$1"; shift
  local port
  port=$(free_port)
  env FR_LISTEN="127.0.0.1:$port" FR_DB_PATH="$ROOT/data/verify-$name.db" "$@" \
      ./bin/fluentread > "/tmp/fr-p0/$name.log" 2>&1 &
  PIDS+=("$!")
  for _ in $(seq 1 80); do
    curl -s --noproxy '*' -o /dev/null "http://127.0.0.1:$port/api/health" && break
    sleep 0.2
  done
  eval "PORT_$name=$port"
  echo "  实例 $name → http://127.0.0.1:$port"
}

echo "=== 1/4 门禁：go vet + go test ==="
if go vet ./... > "$EV/go-vet.txt" 2>&1; then pass "go vet ./..."; else fail "go vet"; cat "$EV/go-vet.txt"; fi
if go test ./... > "$EV/go-test.txt" 2>&1; then pass "go test ./..."; else fail "go test"; tail -20 "$EV/go-test.txt"; fi

echo "=== 2/4 构建：CGO_ENABLED=0 二进制 + 前端（vue-tsc + vite） ==="
if CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/fluentread ./cmd/server 2> "$EV/build-go.txt"; then pass "CGO_ENABLED=0 go build"; else fail "后端构建"; fi
go build -o bin/mockupstream ./cmd/mockupstream || fail "mockupstream 构建"
if (cd web && npm run build) > "$EV/build-web.txt" 2>&1; then pass "pnpm/npm build（含 vue-tsc）"; else fail "前端构建"; tail -20 "$EV/build-web.txt"; fi

echo "=== 3/4 health 三态 ==="
GW_PORT=$(free_port)
./bin/mockupstream --addr "127.0.0.1:$GW_PORT" --models deploy/models.json > /tmp/fr-p0/mock.log 2>&1 &
PIDS+=($!)
sleep 0.5

start_server ok FR_BASE_URL="http://127.0.0.1:$GW_PORT" FR_API_KEY="testkey" FR_MODEL="deepseek-chat"
start_server nogw   # 不配上游 = 未配置态

h1=$(curl -s --noproxy '*' "http://127.0.0.1:$PORT_ok/api/health")
h2=$(curl -s --noproxy '*' "http://127.0.0.1:$PORT_nogw/api/health")
echo "$h1" > "$EV/health-ok.json"; echo "$h2" > "$EV/health-gateway-fail.json"
echo "$h1"
echo "$h2"
if echo "$h1" | grep -q '"db":"ok"' && echo "$h1" | grep -q '"gateway":"ok"' && echo "$h1" | grep -q '"ok":true'; then
  pass "health 就绪态 ok/ok"
else
  fail "health 就绪态"
fi
if echo "$h2" | grep -q '"gateway":"fail"'; then
  pass "health 未配置态 gateway:fail"
else
  fail "health 未配置态"
fi
# db:fail 运行时第三态由单元测试覆盖（TestHealthDBFail）。
if grep -q "TestHealthDBFail" "$EV/go-test.txt" || go test ./internal/httpapi/handler/ -run TestHealthDBFail >> "$EV/go-test.txt" 2>&1; then
  pass "health db:fail（单测 TestHealthDBFail）"
else
  fail "health db:fail"
fi

echo "=== 4/4 Playwright e2e 冒烟 ==="
if (cd web && BASE_URL="http://127.0.0.1:$PORT_ok" EVIDENCE_DIR="$ROOT/$EV/e2e" npx playwright test e2e/p0.spec.ts --reporter=list) > "$EV/e2e.txt" 2>&1; then
  pass "Playwright P0 冒烟（web/e2e/p0.spec.ts）"
else
  fail "Playwright P0 冒烟"; tail -30 "$EV/e2e.txt"
fi

echo
if [ "$FAIL" = "0" ]; then
  echo "P0 出口门禁全部通过；证据在 $EV/"
else
  echo "P0 存在失败项，详见 $EV/"
fi
exit $FAIL

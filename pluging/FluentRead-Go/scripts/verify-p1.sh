#!/usr/bin/env bash
# @file scripts/verify-p1.sh
# P1 翻译链路出口标准验证（3 段 1 失败另 2 段成功；NFR-1 计时；FR-1xx 冒烟）。
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1
EV="docs/acceptance/P1/evidence"
mkdir -p "$EV" /tmp/fr-p1

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

echo "=== 1/3 门禁：go vet + go test + 构建 ==="
if go vet ./... > "$EV/go-vet.txt" 2>&1; then pass "go vet ./..."; else fail "go vet"; cat "$EV/go-vet.txt"; fi
if go test ./... > "$EV/go-test.txt" 2>&1; then pass "go test ./..."; else fail "go test"; tail -20 "$EV/go-test.txt"; fi
if CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/fluentread ./cmd/server 2>> "$EV/build-go.txt"; then pass "CGO_ENABLED=0 go build"; else fail "后端构建"; fi
go build -o bin/mockupstream ./cmd/mockupstream || fail "mockupstream 构建"
if (cd web && npm run build) > "$EV/build-web.txt" 2>&1; then pass "前端构建（含 vue-tsc）"; else fail "前端构建"; tail -20 "$EV/build-web.txt"; fi

echo "=== 2/3 起服务 ==="
GW_PORT=$(free_port)
./bin/mockupstream --addr "127.0.0.1:$GW_PORT" --models deploy/models.json > /tmp/fr-p1/mock.log 2>&1 &
PIDS+=($!)
sleep 0.5

PORT=$(free_port)
FR_LISTEN="127.0.0.1:$PORT" FR_DB_PATH="$ROOT/data/verify-p1.db" \
  FR_BASE_URL="http://127.0.0.1:$GW_PORT" FR_API_KEY="testkey" FR_MODEL="deepseek-chat" \
  ./bin/fluentread > /tmp/fr-p1/server.log 2>&1 &
PIDS+=($!)
for _ in $(seq 1 80); do
  curl -s --noproxy '*' -o /dev/null "http://127.0.0.1:$PORT/api/health" && break
  sleep 0.2
done
echo "  被测服务 → http://127.0.0.1:${PORT} （假上游 :${GW_PORT}）"

echo "=== 3/3 接口级出口标准（3 段 1 失败 2 成功 + NFR-1 计时） ==="
curl -s --noproxy '*' -c /tmp/fr-p1/c.txt "http://127.0.0.1:$PORT/api/health" > /dev/null
T0=$(python3 -c 'import time; print(time.time())')
curl -s --noproxy '*' -b /tmp/fr-p1/c.txt -H 'content-type: application/json' \
  -X POST "http://127.0.0.1:$PORT/api/translate" \
  -d '{"segments":[{"text":"First ok."},{"text":"[[fail]] broken"},{"text":"Third ok."}],"from":"en","to":"zh-Hans"}' \
  > "$EV/translate-3seg.json"
T1=$(python3 -c 'import time; print(time.time())')
python3 - "$EV/translate-3seg.json" << 'PYEOF' && pass "3 段中 1 段失败另 2 段成功（FR-303）" || fail "失败隔离"
import json, sys
d = json.load(open(sys.argv[1]))
segs = d["data"]["segments"]
assert d["ok"] is True, "整体应 ok"
assert [s["status"] for s in segs] == ["ok", "failed", "ok"], segs
assert segs[1]["code"] == "UPSTREAM_UNAVAILABLE", segs[1]
assert "线路" not in segs[1]["message"] and "上游" not in segs[1]["message"], "NFR-11"
PYEOF
ELAPSED=$(python3 -c "print(f'{($T1-$T0)*1000:.0f}')")
echo "  3 段总耗时（含失败段）: ${ELAPSED}ms"
if python3 -c "exit(0 if $ELAPSED < 3000 else 1)"; then
  pass "NFR-1 计时（本机假网关 <1s/段；3 段含失败共 ${ELAPSED}ms）"
else
  fail "NFR-1 计时: ${ELAPSED}ms"
fi

echo "=== e2e 冒烟（Playwright） ==="
if (cd web && BASE_URL="http://127.0.0.1:$PORT" EVIDENCE_DIR="$ROOT/$EV/e2e" npx playwright test e2e/p1.spec.ts --reporter=list) > "$EV/e2e.txt" 2>&1; then
  pass "Playwright P1（分段/全译/失败隔离/对齐/草稿/单段译）"
else
  fail "Playwright P1"; tail -40 "$EV/e2e.txt"
fi

echo
if [ "$FAIL" = "0" ]; then
  echo "P1 出口门禁全部通过；证据在 $EV/"
else
  echo "P1 存在失败项，详见 $EV/"
fi
exit $FAIL

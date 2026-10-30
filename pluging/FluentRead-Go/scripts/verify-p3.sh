#!/usr/bin/env bash
# @file scripts/verify-p3.sh
# P3 纠错出口标准验证：3 处错误出 3 条建议，逐条接受后正确；FR-201~209 全过。
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1
EV="docs/acceptance/P3/evidence"
mkdir -p "$EV" /tmp/fr-p3

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

echo "=== 1/3 门禁 ==="
if go vet ./... > "$EV/go-vet.txt" 2>&1; then pass "go vet ./..."; else fail "go vet"; fi
if go test ./... > "$EV/go-test.txt" 2>&1; then pass "go test ./..."; else fail "go test"; tail -8 "$EV/go-test.txt"; fi
if CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/fluentread ./cmd/server 2>> "$EV/build-go.txt"; then pass "CGO_ENABLED=0 go build"; else fail "后端构建"; fi
go build -o bin/mockupstream ./cmd/mockupstream || fail "mockupstream 构建"
if (cd web && npm run build) > "$EV/build-web.txt" 2>&1; then pass "前端构建（含 vue-tsc）"; else fail "前端构建"; tail -8 "$EV/build-web.txt"; fi

echo "=== 2/3 起服务 + 接口级验证 ==="
GW_PORT=$(free_port)
./bin/mockupstream --addr "127.0.0.1:$GW_PORT" --models deploy/models.json > /tmp/fr-p3/mock.log 2>&1 &
PIDS+=($!)
sleep 0.5

PORT=$(free_port)
FR_LISTEN="127.0.0.1:$PORT" FR_DB_PATH="$ROOT/data/verify-p3.db" \
  FR_BASE_URL="http://127.0.0.1:$GW_PORT" FR_API_KEY="testkey" FR_MODEL="deepseek-chat" \
  ./bin/fluentread > /tmp/fr-p3/server.log 2>&1 &
PIDS+=($!)
for _ in $(seq 1 80); do
  curl -s --noproxy '*' -o /dev/null "http://127.0.0.1:$PORT/api/health" && break
  sleep 0.2
done
echo "  被测服务 → http://127.0.0.1:$PORT"

# FR-204：模型返回不存在的片段 → 服务端丢弃（用夹具同时含合法与非法片段）
BODY='{"text":"The system recieves data from a RAG pipeline. Totally absent marker xyzzy not here.","lang":"en"}'
curl -s --noproxy '*' -c /tmp/fr-p3/c.txt "http://127.0.0.1:$PORT/api/health" > /dev/null
curl -s --noproxy '*' -b /tmp/fr-p3/c.txt -H 'content-type: application/json' \
  -X POST "http://127.0.0.1:$PORT/api/proofread" -d "$BODY" > "$EV/proofread-api.json"
python3 - "$EV/proofread-api.json" << 'PYEOF' && pass "FR-204 非子串建议被服务端丢弃" || fail "FR-204"
import json, sys
d = json.load(open(sys.argv[1]))
items = d["data"]["suggestions"]
assert d["ok"], d
assert all(i["original"] in "The system recieves data from a RAG pipeline. Totally absent marker xyzzy not here." for i in items), items
assert any(i["original"] == "recieves" for i in items), items
assert any(i["original"] == "a RAG" for i in items), items
assert all(i["category"] in {"spelling","grammar","punctuation","style","word_choice"} for i in items), items
assert all(i["reason"].strip() for i in items), items
PYEOF

# FR-206：中文段
curl -s --noproxy '*' -b /tmp/fr-p3/c.txt -H 'content-type: application/json' \
  -X POST "http://127.0.0.1:$PORT/api/proofread" -d '{"text":"这是一段纯中文。","lang":"zh-Hans"}' > "$EV/proofread-cjk.json"
if python3 -c 'import json,sys; d=json.load(open("docs/acceptance/P3/evidence/proofread-cjk.json")); assert d["ok"] and d["data"]["suggestions"]==[]'; then
  pass "FR-206 中文段返回空建议（跳过）"
else
  fail "FR-206"
fi

echo "=== 3/3 e2e ==="
if (cd web && BASE_URL="http://127.0.0.1:$PORT" EVIDENCE_DIR="$ROOT/$EV/e2e" npx playwright test e2e/p3.spec.ts --reporter=list) > "$EV/e2e.txt" 2>&1; then
  pass "Playwright P3（3 建议/逐条接受/忽略/无错反馈/中文无入口/不自动触发）"
else
  fail "Playwright P3"; tail -40 "$EV/e2e.txt"
fi

echo
if [ "$FAIL" = "0" ]; then
  echo "P3 出口门禁全部通过；证据在 $EV/"
else
  echo "P3 存在失败项，详见 $EV/"
fi
exit $FAIL

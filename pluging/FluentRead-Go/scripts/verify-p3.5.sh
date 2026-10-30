#!/usr/bin/env bash
# @file scripts/verify-p3.5.sh
# P3.5 模型选择出口标准验证：三层模型都能用；FR-401~409 全过。
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1
EV="docs/acceptance/P3.5/evidence"
mkdir -p "$EV" /tmp/fr-p35
rm -f "$ROOT/data/verify-p35-"*.db

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

echo "=== 2/3 起服务（巡检间隔 2s，便于验证剔除与恢复） ==="
GW_PORT=$(free_port)
./bin/mockupstream --addr "127.0.0.1:$GW_PORT" --models deploy/models.json > /tmp/fr-p35/mock.log 2>&1 &
PIDS+=($!)
sleep 0.5

PORT=$(free_port)
FR_LISTEN="127.0.0.1:$PORT" FR_DB_PATH="$ROOT/data/verify-p35.db" \
  FR_BASE_URL="http://127.0.0.1:$GW_PORT" FR_API_KEY="testkey" FR_MODEL="deepseek-chat" \
  FR_INSPECT_INTERVAL_SECONDS=2 \
  ./bin/fluentread > /tmp/fr-p35/server.log 2>&1 &
PIDS+=($!)
for _ in $(seq 1 80); do
  curl -s --noproxy '*' -o /dev/null "http://127.0.0.1:$PORT/api/health" && break
  sleep 0.2
done
echo "  被测服务 → http://127.0.0.1:${PORT} （假上游 :${GW_PORT}）"

echo "=== FR-409 启动校验：占位模板启用会被拒绝 ==="
cat > /tmp/fr-p35/bad-models.json << 'EOF'
[{"id":"my-custom","label":"自定义","enabled":true}]
EOF
# macOS 无 GNU timeout：后台起、等 2 秒、杀掉，再看日志。
FR_LISTEN="127.0.0.1:1" FR_DB_PATH=/tmp/fr-p35/bad.db FR_MODELS_FILE=/tmp/fr-p35/bad-models.json \
  ./bin/fluentread > "$EV/fr409-reject.log" 2>&1 &
FR409_PID=$!
sleep 2
if kill -0 "$FR409_PID" 2>/dev/null; then
  kill "$FR409_PID" 2>/dev/null
  fail "FR-409 占位模型启用应启动失败（进程还活着）"
else
  wait "$FR409_PID" 2>/dev/null
  if grep -q "占位模板" "$EV/fr409-reject.log"; then
    pass "FR-409 占位模型启用被启动校验拒绝"
  else
    fail "FR-409 拒绝原因不符"; cat "$EV/fr409-reject.log"
  fi
fi

echo "=== FR-408/1103 巡检：拔掉模型 → 剔除；恢复 → 回来 ==="
# 初始下拉：deepseek-chat + qwen-plus
curl -s --noproxy '*' -c /tmp/fr-p35/c.txt "http://127.0.0.1:$PORT/api/models" > "$EV/models-initial.json"
python3 -c '
import json
d = json.load(open("docs/acceptance/P3.5/evidence/models-initial.json"))["data"]["models"]
ids = [m["id"] for m in d]
assert "deepseek-chat" in ids and "qwen-plus" in ids, ids
assert "my-custom" not in ids, ids  # FR-409：禁用的占位模板不下发
'
pass "初始下拉含 enabled 模型，禁用占位模板不下发（FR-402/409）"

# 拔掉 qwen-plus（mock 从 /v1/models 隐藏）
curl -s --noproxy '*' "http://127.0.0.1:$GW_PORT/__control?models=qwen-plus" > /dev/null
sleep 6  # 2 轮巡检（间隔 2s）
curl -s --noproxy '*' -b /tmp/fr-p35/c.txt "http://127.0.0.1:$PORT/api/models" > "$EV/models-removed.json"
if python3 -c '
import json
d = json.load(open("docs/acceptance/P3.5/evidence/models-removed.json"))
ids = [m["id"] for m in d["data"]["models"]]
assert "qwen-plus" not in ids, ids
assert d["data"]["degraded"] is True, d
'; then
  pass "FR-1103 连续失败模型已从下拉剔除，health 置 degraded"
else
  fail "巡检剔除"; cat "$EV/models-removed.json"
fi
if grep -q "已从下拉剔除" /tmp/fr-p35/server.log; then
  pass "FR-1103 剔除动作记 WARN 日志"
else
  fail "剔除 WARN 日志缺失"
fi

# 恢复
curl -s --noproxy '*' "http://127.0.0.1:$GW_PORT/__control?models=" > /dev/null
sleep 4
curl -s --noproxy '*' -b /tmp/fr-p35/c.txt "http://127.0.0.1:$PORT/api/models" > "$EV/models-recovered.json"
if python3 -c '
import json
d = json.load(open("docs/acceptance/P3.5/evidence/models-recovered.json"))
ids = [m["id"] for m in d["data"]["models"]]
assert "qwen-plus" in ids, ids
assert d["data"]["degraded"] is False, d
'; then
  pass "FR-1103 上游恢复后模型自动回到下拉"
else
  fail "巡检恢复"; cat "$EV/models-recovered.json"
fi

echo "=== FR-404 自定义密钥不落盘、不写日志、响应 no-store ==="
curl -s --noproxy '*' -D "$EV/model-test-headers.txt" -b /tmp/fr-p35/c.txt -H 'content-type: application/json' \
  -X POST "http://127.0.0.1:$PORT/api/models/test" \
  -d "{\"baseURL\":\"http://127.0.0.1:$GW_PORT\",\"apiKey\":\"VISITOR-SECRET-XYZ\",\"model\":\"deepseek-chat\"}" \
  > "$EV/model-test.json"
if python3 -c '
import json
d = json.load(open("docs/acceptance/P3.5/evidence/model-test.json"))
assert d["ok"] and d["data"]["ok"] is True, d
raw = json.dumps(d)
assert "VISITOR-SECRET-XYZ" not in raw, "响应回显了密钥"
'; then
  pass "自定义模型测试连接成功且响应不含密钥"
else
  fail "FR-404 测试连接"; cat "$EV/model-test.json"
fi
if grep -qi 'no-store' "$EV/model-test-headers.txt"; then
  pass "FR-404 响应 no-store"
else
  fail "缺 no-store 头"; cat "$EV/model-test-headers.txt"
fi
if grep -q "VISITOR-SECRET-XYZ" /tmp/fr-p35/server.log; then
  fail "FR-404 密钥写进了服务端日志"
else
  pass "FR-404 服务端日志无密钥"
fi

echo "=== 3/3 e2e（FR-401/402/403/405/409 浏览器路径） ==="
if (cd web && BASE_URL="http://127.0.0.1:$PORT" CUSTOM_MOCK_URL="http://127.0.0.1:$GW_PORT" \
    EVIDENCE_DIR="$ROOT/$EV/e2e" npx playwright test e2e/p3.5.spec.ts --reporter=list) > "$EV/e2e.txt" 2>&1; then
  pass "Playwright P3.5（下拉/自定义测试连接与翻译/失败回落提示）"
else
  fail "Playwright P3.5"; tail -40 "$EV/e2e.txt"
fi

echo
if [ "$FAIL" = "0" ]; then
  echo "P3.5 出口门禁全部通过；证据在 $EV/"
else
  echo "P3.5 存在失败项，详见 $EV/"
fi
exit $FAIL

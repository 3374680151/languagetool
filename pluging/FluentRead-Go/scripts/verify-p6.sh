#!/usr/bin/env bash
# @file scripts/verify-p6.sh
# P6 SDK 出口标准验证：FR-901~906 全过 + 体积断言 ≤30KB gzip（FR-905）。
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1
EV="docs/acceptance/P6/evidence"
mkdir -p "$EV" /tmp/fr-p6
rm -f "$ROOT/data/verify-p6"*.db

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

echo "=== 1/3 门禁 + 体积断言 ==="
if go vet ./... > "$EV/go-vet.txt" 2>&1; then pass "go vet ./..."; else fail "go vet"; fi
if go test ./... > "$EV/go-test.txt" 2>&1; then pass "go test ./..."; else fail "go test"; tail -8 "$EV/go-test.txt"; fi
if CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/fluentread ./cmd/server 2>> "$EV/build-go.txt"; then pass "CGO_ENABLED=0 go build"; else fail "后端构建"; fi
go build -o bin/mockupstream ./cmd/mockupstream || fail "mockupstream 构建"
if (cd web && npm run build) > "$EV/build-web.txt" 2>&1; then pass "前端构建（含 vue-tsc）"; else fail "前端构建"; tail -8 "$EV/build-web.txt"; fi

# FR-905：SDK 体积 ≤30KB gzip
GZ=$(gzip -c web/sdk/fluentread-sdk.js | wc -c | tr -d ' ')
RAW=$(wc -c < web/sdk/fluentread-sdk.js | tr -d ' ')
echo "  SDK 体积：原始 ${RAW}B，gzip ${GZ}B（预算 30720B）"
echo "raw=$RAW gzip=$GZ" > "$EV/sdk-size.txt"
if [ "$GZ" -le 30720 ]; then
  pass "FR-905 SDK ≤30KB gzip（实测 ${GZ}B）"
else
  fail "FR-905 SDK 超预算: ${GZ}B"
fi

echo "=== 2/3 起服务 ==="
GW_PORT=$(free_port)
./bin/mockupstream --addr "127.0.0.1:$GW_PORT" --models deploy/models.json > /tmp/fr-p6/mock.log 2>&1 &
PIDS+=($!)
sleep 0.5

PORT=$(free_port)
ORIGIN="http://127.0.0.1:$PORT"
FR_LISTEN="127.0.0.1:$PORT" FR_DB_PATH="$ROOT/data/verify-p6.db" \
  FR_BASE_URL="http://127.0.0.1:$GW_PORT" FR_API_KEY="testkey" FR_MODEL="deepseek-chat" \
  FR_SDK_ORIGINS="$ORIGIN" \
  ./bin/fluentread > /tmp/fr-p6/server.log 2>&1 &
PIDS+=($!)
for _ in $(seq 1 80); do
  curl -s --noproxy '*' -o /dev/null "http://127.0.0.1:$PORT/api/health" && break
  sleep 0.2
done
echo "  被测服务 → ${ORIGIN} （白名单含自身，演示页可用）"

# SDK 关闭实例（白名单为空）
PORT2=$(free_port)
FR_LISTEN="127.0.0.1:$PORT2" FR_DB_PATH="$ROOT/data/verify-p6-off.db" \
  FR_BASE_URL="http://127.0.0.1:$GW_PORT" FR_API_KEY="testkey" FR_MODEL="deepseek-chat" \
  ./bin/fluentread > /tmp/fr-p6/server-off.log 2>&1 &
PIDS+=($!)
for _ in $(seq 1 80); do
  curl -s --noproxy '*' -o /dev/null "http://127.0.0.1:$PORT2/api/health" && break
  sleep 0.2
done

echo "--- 接口级验证 ---"
# FR-906 白名单内可用
curl -s --noproxy '*' -c /tmp/fr-p6/c.txt -H "Origin: $ORIGIN" "http://127.0.0.1:$PORT/api/config" > "$EV/config-sdk-on.json"
CODE=$(curl -s --noproxy '*' -o /dev/null -w '%{http_code}' -b /tmp/fr-p6/c.txt -H "Origin: $ORIGIN" -H 'content-type: application/json' \
  -X POST "http://127.0.0.1:$PORT/api/sdk/translate" -d '{"segments":[{"text":"SDK works fine."}],"from":"en","to":"zh-Hans"}')
echo "  白名单内 SDK translate: $CODE"
if [ "$CODE" = "200" ]; then pass "FR-906 白名单内来源可用"; else fail "白名单内被拒: $CODE"; fi

# FR-906 白名单外 403
CODE2=$(curl -s --noproxy '*' -o /dev/null -w '%{http_code}' -H "Origin: https://evil.example.com" -H 'content-type: application/json' \
  -X POST "http://127.0.0.1:$PORT/api/sdk/translate" -d '{"segments":[{"text":"x"}],"from":"en","to":"zh-Hans"}')
BODY2=$(curl -s --noproxy '*' -H "Origin: https://evil.example.com" -H 'content-type: application/json' \
  -X POST "http://127.0.0.1:$PORT/api/sdk/translate" -d '{"segments":[{"text":"x"}],"from":"en","to":"zh-Hans"}')
echo "$BODY2" > "$EV/sdk-forbidden.json"
if [ "$CODE2" = "403" ] && echo "$BODY2" | grep -q "ORIGIN_FORBIDDEN" && echo "$BODY2" | grep -q "未开通翻译服务"; then
  pass "FR-906 白名单外 403 + 可读提示"
else
  fail "FR-906: $CODE2 $BODY2"
fi

# FR-906 SDK 关闭（白名单空）→ 403
CODE3=$(curl -s --noproxy '*' -o /dev/null -w '%{http_code}' -H "Origin: http://127.0.0.1:$PORT2" -H 'content-type: application/json' \
  -X POST "http://127.0.0.1:$PORT2/api/sdk/translate" -d '{"segments":[{"text":"x"}],"from":"en","to":"zh-Hans"}')
if [ "$CODE3" = "403" ]; then
  pass "FR-906 白名单为空 = SDK 关闭"
else
  fail "SDK 关闭未生效: $CODE3"
fi

# FR-903 与主站同一配额：SDK 翻译计入 usage_daily
USED=$(curl -s --noproxy '*' -b /tmp/fr-p6/c.txt "http://127.0.0.1:$PORT/api/quota" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["used"])')
echo "  SDK 翻译后 quota.used = $USED"
if [ "$USED" -ge 1 ]; then
  pass "FR-903 SDK 与主站同一配额（SDK 调用计入了访客用量）"
else
  fail "FR-903 配额未共用"
fi

# SDK 脚本与演示页可下发
curl -s --noproxy '*' -D "$EV/sdk-js-headers.txt" "http://127.0.0.1:$PORT/sdk/fluentread-sdk.js" -o "$EV/fluentread-sdk.js"
if grep -qi 'cache-control' "$EV/sdk-js-headers.txt"; then pass "SDK 脚本下发（带缓存头）"; else fail "SDK 下发失败"; fi

echo "=== 3/3 e2e（演示页真实浏览器） ==="
if (cd web && BASE_URL="$ORIGIN" SDK_OFF_URL="http://127.0.0.1:$PORT2" EVIDENCE_DIR="$ROOT/$EV/e2e" npx playwright test e2e/p6.spec.ts --reporter=list) > "$EV/e2e.txt" 2>&1; then
  pass "Playwright P6（悬浮球/面板/划词气泡/词卡收藏/样式隔离/白名单）"
else
  fail "Playwright P6"; tail -40 "$EV/e2e.txt"
fi

echo
if [ "$FAIL" = "0" ]; then
  echo "P6 出口门禁全部通过；证据在 $EV/"
else
  echo "P6 存在失败项，详见 $EV/"
fi
exit $FAIL

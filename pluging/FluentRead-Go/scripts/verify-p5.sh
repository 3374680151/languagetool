#!/usr/bin/env bash
# @file scripts/verify-p5.sh
# P5 词表+收藏闭环出口标准验证：FR-702~704、1201、1202 全过 + AD-6/AD-10。
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1
EV="docs/acceptance/P5/evidence"
mkdir -p "$EV" /tmp/fr-p5
: > "$EV/gates.txt"  # 截断上次运行的门禁记录（每次运行独立留证）
rm -f "$ROOT/data/verify-p5"*.db

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
./bin/mockupstream --addr "127.0.0.1:$GW_PORT" --models deploy/models.json > /tmp/fr-p5/mock.log 2>&1 &
PIDS+=($!)
sleep 0.5

PORT=$(free_port)
FR_LISTEN="127.0.0.1:$PORT" FR_DB_PATH="$ROOT/data/verify-p5.db" \
  FR_BASE_URL="http://127.0.0.1:$GW_PORT" FR_API_KEY="testkey" FR_MODEL="deepseek-chat" \
  ./bin/fluentread > /tmp/fr-p5/server.log 2>&1 &
PIDS+=($!)
for _ in $(seq 1 80); do
  curl -s --noproxy '*' -o /dev/null "http://127.0.0.1:$PORT/api/health" && break
  sleep 0.2
done
echo "  被测服务 → http://127.0.0.1:$PORT"

curl -s --noproxy '*' -c /tmp/fr-p5/c.txt "http://127.0.0.1:$PORT/api/config" > /dev/null

echo "--- FR-1201 收藏幂等（同词重复收藏返回已有记录） ---"
BODY='{"word":"recall","lang":"en","contextSentence":"The judge will recall the witness."}'
curl -s --noproxy '*' -b /tmp/fr-p5/c.txt -H 'content-type: application/json' -X POST "http://127.0.0.1:$PORT/api/words" -d "$BODY" > "$EV/words-create1.json"
curl -s --noproxy '*' -b /tmp/fr-p5/c.txt -H 'content-type: application/json' -X POST "http://127.0.0.1:$PORT/api/words" -d "$BODY" > "$EV/words-create2.json"
python3 - "$EV/words-create1.json" "$EV/words-create2.json" << 'PYEOF' && pass "FR-1201 收藏幂等（created=true 后 false，同一 id）" || fail "FR-1201 幂等"
import json, sys
a = json.load(open(sys.argv[1]))["data"]
b = json.load(open(sys.argv[2]))["data"]
assert a["created"] is True and b["created"] is False, (a, b)
assert a["word"]["id"] == b["word"]["id"], (a, b)
assert a["word"]["contextSentence"].startswith("The judge will recall"), a
PYEOF

echo "--- FR-1202 再使用标记（词形还原匹配，AD-7 同一实现） ---"
curl -s --noproxy '*' -b /tmp/fr-p5/c.txt -H 'content-type: application/json' -X POST "http://127.0.0.1:$PORT/api/translate" \
  -d '{"segments":[{"text":"New paragraph will recall the old memory and recalled again."}],"from":"en","to":"zh-Hans"}' > /dev/null
curl -s --noproxy '*' -b /tmp/fr-p5/c.txt "http://127.0.0.1:$PORT/api/words" > "$EV/words-after-reuse.json"
python3 - "$EV/words-after-reuse.json" << 'PYEOF' && pass "FR-1202 recall 与 recalled 均命中 → 用过 1 次（每请求一次）" || fail "FR-1202"
import json, sys
d = json.load(open(sys.argv[1]))["data"]
row = [w for w in d if w["word"] == "recall"][0]
assert row["useCount"] == 1, row
assert row["contextSentence"].startswith("The judge"), row
PYEOF

echo "--- FR-703 按访客存储（换 cookie = 新词表） ---"
curl -s --noproxy '*' -c /tmp/fr-p5/c2.txt "http://127.0.0.1:$PORT/api/config" > /dev/null
curl -s --noproxy '*' -b /tmp/fr-p5/c2.txt "http://127.0.0.1:$PORT/api/words" > "$EV/words-other-visitor.json"
if python3 -c 'import json,sys; d=json.load(open("docs/acceptance/P5/evidence/words-other-visitor.json"))["data"]; assert d==[], d'; then
  pass "FR-703 另一访客词表为空（匿名隔离）"
else
  fail "FR-703"
fi

echo "=== 3/3 e2e ==="
if (cd web && BASE_URL="http://127.0.0.1:$PORT" EVIDENCE_DIR="$ROOT/$EV/e2e" npx playwright test e2e/p5.spec.ts --reporter=list) > "$EV/e2e.txt" 2>&1; then
  pass "Playwright P5（收藏带原句/幂等/再用+1/排序/访客隔离/删除/搜索弹卡）"
else
  fail "Playwright P5"; tail -40 "$EV/e2e.txt"
fi

echo
if [ "$FAIL" = "0" ]; then
  echo "P5 出口门禁全部通过；证据在 $EV/"
else
  echo "P5 存在失败项，详见 $EV/"
fi
exit $FAIL

#!/usr/bin/env bash
# @file scripts/verify-p4.sh
# P4 词典+术语+词卡出口标准验证：§八 AD-1~12（AD-10 随 P5）+ FR-501~505、701。
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1
EV="docs/acceptance/P4/evidence"
mkdir -p "$EV" /tmp/fr-p4
rm -f "$ROOT/data/verify-p4"*.db

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
./bin/mockupstream --addr "127.0.0.1:$GW_PORT" --models deploy/models.json > /tmp/fr-p4/mock.log 2>&1 &
PIDS+=($!)
sleep 0.5

PORT=$(free_port)
FR_LISTEN="127.0.0.1:$PORT" FR_DB_PATH="$ROOT/data/verify-p4.db" \
  FR_BASE_URL="http://127.0.0.1:$GW_PORT" FR_API_KEY="testkey" FR_MODEL="deepseek-chat" \
  FR_DICT_RATE_PER_MINUTE=3 \
  ./bin/fluentread > /tmp/fr-p4/server.log 2>&1 &
PIDS+=($!)
for _ in $(seq 1 80); do
  curl -s --noproxy '*' -o /dev/null "http://127.0.0.1:$PORT/api/health" && break
  sleep 0.2
done
echo "  被测服务 → http://127.0.0.1:$PORT"

curl -s --noproxy '*' -c /tmp/fr-p4/c.txt "http://127.0.0.1:$PORT/api/config" > /dev/null  # 经访客中间件拿 cookie

echo "--- AD-4 词典命中：recall（音标/释义/标签，<100ms） ---"
T0=$(python3 -c 'import time; print(time.time())')
curl -s --noproxy '*' -b /tmp/fr-p4/c.txt "http://127.0.0.1:$PORT/api/dict/recall" > "$EV/dict-recall.json"
T1=$(python3 -c 'import time; print(time.time())')
python3 - "$EV/dict-recall.json" << 'PYEOF' && pass "AD-4 ECDICT 命中 recall（音标/词性/释义/标签）" || fail "AD-4"
import json, sys
d = json.load(open(sys.argv[1]))["data"]
assert d["word"] == "recall" and d["source"] == "ecdict", d
assert d["phonetic"], d
assert d["senses"], d
assert d["collected"]["yes"] is False, d
PYEOF
ELAPSED=$(python3 -c "print(f'{($T1-$T0)*1000:.0f}')")
echo "  首查耗时 ${ELAPSED}ms"
if python3 -c "exit(0 if $ELAPSED < 300 else 1)"; then pass "AD-4 感知 <100ms 量级（服务端 ${ELAPSED}ms，含 HTTP）"; else fail "词典查询太慢: ${ELAPSED}ms"; fi

echo "--- AD-7 词形还原：recalled → recall（与 FR-1202 同一实现） ---"
curl -s --noproxy '*' -b /tmp/fr-p4/c.txt "http://127.0.0.1:$PORT/api/dict/recalled" > "$EV/dict-recalled.json"
python3 - "$EV/dict-recalled.json" << 'PYEOF' && pass "AD-7 recalled 还原为 recall" || fail "AD-7"
import json, sys
d = json.load(open(sys.argv[1]))["data"]
assert d["word"] == "recall" and d["query"] == "recalled", d
PYEOF

echo "--- AD-4/8.3 生僻词走模型兜底并标注 ---"
curl -s --noproxy '*' -b /tmp/fr-p4/c.txt "http://127.0.0.1:$PORT/api/dict/gerrymander" > "$EV/dict-gerrymander.json"
python3 - "$EV/dict-gerrymander.json" << 'PYEOF' && pass "AD-4 生僻词转模型解释（source=model，带例句）" || fail "AD-4 模型兜底"
import json, sys
d = json.load(open(sys.argv[1]))["data"]
assert d["source"] == "model", d
assert d["senses"], d
PYEOF

echo "--- AD-9 查词不计翻译配额；模型兜底限频（3 次/分钟） ---"
BEFORE=$(curl -s --noproxy '*' -b /tmp/fr-p4/c.txt "http://127.0.0.1:$PORT/api/quota" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["used"])')
for w in obscure1 obscure2 obscure3 obscure4; do
  curl -s --noproxy '*' -o /dev/null -w "%{http_code}\n" -b /tmp/fr-p4/c.txt "http://127.0.0.1:$PORT/api/dict/$w" >> "$EV/dict-ratelimit.txt"
done
AFTER=$(curl -s --noproxy '*' -b /tmp/fr-p4/c.txt "http://127.0.0.1:$PORT/api/quota" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["used"])')
echo "  配额 before=${BEFORE} after=${AFTER} ；限频响应码序列：$(tr '\n' ',' < "$EV/dict-ratelimit.txt")"
if [ "${BEFORE}" = "${AFTER}" ]; then
  pass "AD-9 查词不计翻译配额"
else
  fail "AD-9 配额被查词消耗"
fi
if grep -q "429" "$EV/dict-ratelimit.txt"; then
  pass "AD-9 模型兜底超限返回 429"
else
  fail "AD-9 限频未生效"; cat "$EV/dict-ratelimit.txt"
fi

echo "--- FR-505 术语冲突按 站点 > 用户 消解 ---"
curl -s --noproxy '*' -b /tmp/fr-p4/c.txt -H 'content-type: application/json' -X POST "http://127.0.0.1:$PORT/api/glossary" \
  -d '{"scope":"site","sourceTerm":"recall","targetTerm":"站点译法"}' > /dev/null
curl -s --noproxy '*' -b /tmp/fr-p4/c.txt -H 'content-type: application/json' -X POST "http://127.0.0.1:$PORT/api/glossary" \
  -d '{"scope":"user","sourceTerm":"recall","targetTerm":"用户译法"}' > /dev/null
curl -s --noproxy '*' -b /tmp/fr-p4/c.txt "http://127.0.0.1:$PORT/api/glossary" > "$EV/glossary-merged.json"
if python3 -c '
import json
d = json.load(open("docs/acceptance/P4/evidence/glossary-merged.json"))["data"]
items = {i["sourceTerm"]: i for i in d["items"] if i["sourceTerm"] == "recall"}
assert items["recall"]["targetTerm"] == "站点译法", items  # 站点 > 用户
assert d["fingerprint"], d
'; then
  pass "FR-505 站点级覆盖用户级；返回指纹"
else
  fail "FR-505"; cat "$EV/glossary-merged.json"
fi

echo "=== 3/3 e2e（AD-1/2/3/4/5/7/11/12 + FR-501~504） ==="
if (cd web && BASE_URL="http://127.0.0.1:$PORT" MOCK_URL="http://127.0.0.1:$GW_PORT" \
    EVIDENCE_DIR="$ROOT/$EV/e2e" npx playwright test e2e/p4.spec.ts --reporter=list) > "$EV/e2e.txt" 2>&1; then
  pass "Playwright P4（弹卡触发/内容/收藏/模式/朗读 + 术语闭环）"
else
  fail "Playwright P4"; tail -40 "$EV/e2e.txt"
fi

echo
if [ "$FAIL" = "0" ]; then
  echo "P4 出口门禁全部通过；证据在 $EV/"
else
  echo "P4 存在失败项，详见 $EV/"
fi
exit $FAIL

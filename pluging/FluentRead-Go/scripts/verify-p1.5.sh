#!/usr/bin/env bash
# @file scripts/verify-p1.5.sh
# P1.5 护栏出口标准验证：FR-601~606 逐条过。
#
# 四个实例（各自独立数据库，避免配额互相污染）：
#   A normal  默认配额/限流/站点上限 + 免费后备
#   B tight   访客日配额=3（FR-601 重启不归零、FR-605 Retry-After）
#   C cap     站点付费日字符=30 + 免费后备（FR-603/306 触顶降级、FR-604 免费不计字符）
#   D rate    访客每分钟桶=5（FR-602 被拒请求不打上游）
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1
EV="docs/acceptance/P1.5/evidence"
mkdir -p "$EV" /tmp/fr-p15
: > "$EV/gates.txt"  # 截断上次运行的门禁记录（每次运行独立留证）
rm -f "$ROOT/data/verify-p15-"*.db

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
  env FR_LISTEN="127.0.0.1:$port" FR_DB_PATH="$ROOT/data/verify-p15-$name.db" "$@" \
      ./bin/fluentread > "/tmp/fr-p15/$name.log" 2>&1 &
  PIDS+=("$!")
  for _ in $(seq 1 80); do
    curl -s --noproxy '*' -o /dev/null "http://127.0.0.1:$port/api/health" && break
    sleep 0.2
  done
  eval "PORT_$name=$port"
  echo "  实例 $name → http://127.0.0.1:$port"
}

translate() { # translate <port> <cookiejar> <body>
  curl -s --noproxy '*' -b "$2" -c "$2" -H 'content-type: application/json' \
    -X POST "http://127.0.0.1:$1/api/translate" -d "$3"
}

echo "=== 1/3 门禁 ==="
if go vet ./... > "$EV/go-vet.txt" 2>&1; then pass "go vet ./..."; else fail "go vet"; fi
if go test ./... > "$EV/go-test.txt" 2>&1; then pass "go test ./..."; else fail "go test"; tail -10 "$EV/go-test.txt"; fi
if CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/fluentread ./cmd/server 2>> "$EV/build-go.txt"; then pass "CGO_ENABLED=0 go build"; else fail "后端构建"; fi
go build -o bin/mockupstream ./cmd/mockupstream || fail "mockupstream 构建"
if (cd web && npm run build) > "$EV/build-web.txt" 2>&1; then pass "前端构建（含 vue-tsc）"; else fail "前端构建"; tail -10 "$EV/build-web.txt"; fi

echo "=== 2/3 起服务 ==="
GW_PORT=$(free_port)
./bin/mockupstream --addr "127.0.0.1:$GW_PORT" --models deploy/models.json > /tmp/fr-p15/mock.log 2>&1 &
PIDS+=($!)
sleep 0.5

start_server A FR_BASE_URL="http://127.0.0.1:$GW_PORT" FR_API_KEY="testkey" FR_MODEL="deepseek-chat" \
  FR_FALLBACK_TYPE="mymemory" FR_FALLBACK_BASE_URL="http://127.0.0.1:$GW_PORT/get"
start_server B FR_BASE_URL="http://127.0.0.1:$GW_PORT" FR_API_KEY="testkey" FR_MODEL="deepseek-chat" \
  FR_VISITOR_DAILY_SEGMENTS=3 FR_RATE_PER_MINUTE=100
BPID="${PIDS[$(( ${#PIDS[@]} - 1 ))]}"
start_server C FR_BASE_URL="http://127.0.0.1:$GW_PORT" FR_API_KEY="testkey" FR_MODEL="deepseek-chat" \
  FR_SITE_DAILY_PAID_CHARS=30 FR_RATE_PER_MINUTE=100 \
  FR_FALLBACK_TYPE="mymemory" FR_FALLBACK_BASE_URL="http://127.0.0.1:$GW_PORT/get"
start_server D FR_BASE_URL="http://127.0.0.1:$GW_PORT" FR_API_KEY="testkey" FR_MODEL="deepseek-chat" \
  FR_RATE_PER_MINUTE=5 FR_VISITOR_DAILY_SEGMENTS=1000
# E low：额度 5 段，给 FR-606 低额态 e2e 用（不能与 B 共用——B 已被 601 用满）。
start_server E FR_BASE_URL="http://127.0.0.1:$GW_PORT" FR_API_KEY="testkey" FR_MODEL="deepseek-chat" \
  FR_VISITOR_DAILY_SEGMENTS=5 FR_RATE_PER_MINUTE=100

curl -s --noproxy '*' "http://127.0.0.1:$GW_PORT/__reset" > /dev/null

echo "=== FR-604 计费口径：5 段含 3 空段只扣 2 ==="
BODY_A='{"segments":[{"text":"Alpha segment."},{"text":"  "},{"text":""},{"text":"Beta segment."},{"text":"   "}],"from":"en","to":"zh-Hans"}'
translate "$PORT_A" /tmp/fr-p15/a.txt "$BODY_A" > "$EV/fr604-translate.json"
USED=$(curl -s --noproxy '*' -b /tmp/fr-p15/a.txt "http://127.0.0.1:$PORT_A/api/quota" | python3 -c 'import json,sys; print(json.load(sys.stdin)["data"]["used"])')
echo "  quota.used = ${USED} （期望 2）"
if [ "$USED" = "2" ]; then pass "FR-604 空白段不计费（used=2）"; else fail "FR-604: used=$USED"; fi

echo "=== FR-603/306 站点触顶降级（不拒绝） ==="
curl -s --noproxy '*' "http://127.0.0.1:$GW_PORT/__reset" > /dev/null
LONG='{"segments":[{"text":"This single segment carries more than thirty characters for sure."}],"from":"en","to":"zh-Hans"}'
translate "$PORT_C" /tmp/fr-p15/c.txt "$LONG" > "$EV/fr603-degraded.json"
python3 - "$EV/fr603-degraded.json" << 'PYEOF' && pass "FR-603/306 触顶降级 degraded:true，译文可用" || fail "FR-603/306"
import json, sys
d = json.load(open(sys.argv[1]))
segs = d["data"]["segments"]
assert d["ok"] and segs[0]["status"] == "ok", segs
assert segs[0].get("degraded") is True, segs[0]
PYEOF
# 上游只打了免费线路，没打付费线路
STATS=$(curl -s --noproxy '*' "http://127.0.0.1:$GW_PORT/__stats")
echo "  mock stats: $STATS"
if echo "$STATS" | python3 -c 'import json,sys; s=json.load(sys.stdin); exit(0 if s["free"]>=1 and s["chat"]==0 else 1)'; then
  pass "FR-603/604 触顶后付费线路 0 次调用，免费线路接管"
else
  fail "触顶降级上游计数"
fi

echo "=== FR-601 配额落库：用满 3 段 → 429 → 重启 → 仍 429 ==="
ONE='{"segments":[{"text":"One segment."}],"from":"en","to":"zh-Hans"}'
for i in 1 2 3; do translate "$PORT_B" /tmp/fr-p15/b.txt "$ONE" > /dev/null; done
CODE=$(curl -s --noproxy '*' -o /dev/null -w '%{http_code}' -b /tmp/fr-p15/b.txt -H 'content-type: application/json' -X POST "http://127.0.0.1:$PORT_B/api/translate" -d "$ONE")
echo "  用满后状态码 = $CODE"
if [ "$CODE" = "429" ]; then pass "FR-601 配额用尽返回 429"; else fail "FR-601: $CODE"; fi

# 重启实例 B（同库）：杀掉记录的 BPID，不能用 PIDS 下标——后面还追加了 D/E。
kill "$BPID" 2>/dev/null || true
sleep 0.5
start_server B FR_BASE_URL="http://127.0.0.1:$GW_PORT" FR_API_KEY="testkey" FR_MODEL="deepseek-chat" \
  FR_VISITOR_DAILY_SEGMENTS=3 FR_RATE_PER_MINUTE=100
CODE2=$(curl -s --noproxy '*' -o /dev/null -w '%{http_code}' -b /tmp/fr-p15/b.txt -H 'content-type: application/json' -X POST "http://127.0.0.1:$PORT_B/api/translate" -d "$ONE")
echo "  重启后状态码 = $CODE2"
if [ "$CODE2" = "429" ]; then pass "FR-601 重启后配额不归零"; else fail "FR-601 重启: $CODE2"; fi

echo "=== FR-605 429 必须同时给 Retry-After 头与可读等待时间 ==="
curl -s --noproxy '*' -D "$EV/fr605-headers.txt" -b /tmp/fr-p15/b.txt -H 'content-type: application/json' \
  -X POST "http://127.0.0.1:$PORT_B/api/translate" -d "$ONE" > "$EV/fr605-body.json"
if grep -qi '^retry-after: [0-9]' "$EV/fr605-headers.txt"; then
  pass "FR-605 Retry-After 头存在"
else
  fail "FR-605 缺 Retry-After 头"; cat "$EV/fr605-headers.txt"
fi
if python3 -c '
import json
d = json.load(open("docs/acceptance/P1.5/evidence/fr605-body.json"))
msg = d["error"]["message"]
assert d["ok"] is False and d["error"]["code"] == "QUOTA_EXCEEDED", d
assert "小时" in msg or "分钟" in msg, msg
assert "3 段" in msg, msg
assert not any(w in msg for w in ["上游","线路","配额","跳","回退","provider"])
' 2>/dev/null; then
  pass "FR-605 文案含恢复时间且无内部词"
else
  fail "FR-605 文案"; cat "$EV/fr605-body.json"
fi

echo "=== FR-602 双桶限流：被拒请求一次都不打上游 ==="
curl -s --noproxy '*' "http://127.0.0.1:$GW_PORT/__reset" > /dev/null
ALLOWED=0; REJECTED=0
for i in $(seq 1 12); do
  # 每次文本不同：排除缓存命中干扰（缓存命中不调上游是 FR-1101 的正确行为）。
  BODY_D="{\"segments\":[{\"text\":\"Unique request number ${i} for rate limiting.\"}],\"from\":\"en\",\"to\":\"zh-Hans\"}"
  CODE=$(curl -s --noproxy '*' -o /dev/null -w '%{http_code}' -b /tmp/fr-p15/d.txt -c /tmp/fr-p15/d.txt \
    -H 'content-type: application/json' -X POST "http://127.0.0.1:$PORT_D/api/translate" -d "$BODY_D")
  if [ "$CODE" = "200" ]; then ALLOWED=$((ALLOWED+1)); else REJECTED=$((REJECTED+1)); fi
done
CHAT=$(curl -s --noproxy '*' "http://127.0.0.1:$GW_PORT/__stats" | python3 -c 'import json,sys; print(json.load(sys.stdin)["chat"])')
echo "  放行 ${ALLOWED} / 拒绝 ${REJECTED} / 上游 chat 调用 ${CHAT}"
if [ "$ALLOWED" -ge 1 ] && [ "$REJECTED" -ge 1 ] && [ "$CHAT" = "$ALLOWED" ]; then
  pass "FR-602 放行数=${ALLOWED} ，被拒的 ${REJECTED} 次全部没打上游"
else
  fail "FR-602: allowed=$ALLOWED rejected=$REJECTED chat=$CHAT"
fi

echo "=== 3/3 e2e：额度提示只在该出现时出现（FR-606） ==="
if (cd web && BASE_URL="http://127.0.0.1:$PORT_A" EXPECT_QUOTA=ok EVIDENCE_DIR="$ROOT/$EV/e2e-ok" npx playwright test e2e/p1.5.spec.ts --reporter=list) > "$EV/e2e-ok.txt" 2>&1; then
  pass "FR-606 额度充足时无提示"
else
  fail "FR-606 充足态"; tail -15 "$EV/e2e-ok.txt"
fi
if (cd web && BASE_URL="http://127.0.0.1:$PORT_E" EXPECT_QUOTA=low EVIDENCE_DIR="$ROOT/$EV/e2e-low" npx playwright test e2e/p1.5.spec.ts --reporter=list) > "$EV/e2e-low.txt" 2>&1; then
  pass "FR-606 额度快用完时出现提示"
else
  fail "FR-606 低额态"; tail -15 "$EV/e2e-low.txt"
fi

echo
if [ "$FAIL" = "0" ]; then
  echo "P1.5 出口门禁全部通过；证据在 $EV/"
else
  echo "P1.5 存在失败项，详见 $EV/"
fi
exit $FAIL

#!/usr/bin/env bash
# @file scripts/verify-p1.5.sh
# P1.5 护栏退出标准验证。对应 docs/06-plan.md 的 P1.5 出口，逐条编号：
#   ① 同一访客第 N+1 次请求返回 429 QUOTA_EXCEEDED，hint 含剩余与恢复时间
#   ② 1 秒内连发 50 次，超出 FR_RATE_LIMIT 的请求被拒，且不打上游
#   ③ FR_UPSTREAM_DAILY_CAP 调到极小值，翻译仍成功但响应 degraded=true
#   ④ 空白段不计入配额（3 空段 + 2 实段，只扣 2）
#   ⑤ 重启服务后，当天已用配额不归零
#   ⑥ 全部由脚本断言，不依赖人工点击
#
# 关键点：每条用独立数据库，避免互相污染；"不打上游"用假网关的调用计数证明，
# 只看响应码是证明不了的。
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1

FAIL=0
pass() { echo "  PASS  $1"; }
fail() { echo "  FAIL  $1"; FAIL=1; }
free_port() { python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()'; }

GW_PORT=$(free_port)
GW="http://127.0.0.1:$GW_PORT"
SRV_PID=""

cleanup() {
  [ -n "$SRV_PID" ] && kill "$SRV_PID" 2>/dev/null
  [ -n "${GW_PID:-}" ] && kill "$GW_PID" 2>/dev/null
  wait 2>/dev/null
}
trap cleanup EXIT

# ---------- 假网关 ----------
echo "=== 启动假网关 $GW ==="
go build -o bin/fake-gateway ./scripts/fake-gateway || { echo "!! 假网关编译失败"; exit 1; }
./bin/fake-gateway -addr "127.0.0.1:$GW_PORT" -models deploy/models.json > /tmp/fr-gw.log 2>&1 &
GW_PID=$!
for _ in $(seq 1 40); do
  curl -s --noproxy '*' -o /dev/null "$GW/v1/models" && break
  sleep 0.2
done
curl -s --noproxy '*' -o /dev/null "$GW/v1/models" || { echo "!! 假网关未就绪"; cat /tmp/fr-gw.log; exit 1; }
echo "就绪"

gw_stat() { curl -s --noproxy '*' "$GW/__stats" | python3 -c "import json,sys;print(json.load(sys.stdin)['$1'])"; }
gw_reset() { curl -s --noproxy '*' -X POST "$GW/__reset" > /dev/null; }

# start_server <db-name> <extra env...>
# 起一个独立数据库的服务实例，端口随机。
start_server() {
  local db="$1"; shift
  [ -n "$SRV_PID" ] && { kill "$SRV_PID" 2>/dev/null; wait "$SRV_PID" 2>/dev/null; SRV_PID=""; }
  SRV_PORT=$(free_port)
  env FR_ADDR="127.0.0.1:$SRV_PORT" \
      FR_DB_PATH="$ROOT/data/$db" \
      FR_GATEWAY_BASE="$GW" \
      FR_GATEWAY_KEY="test-key" \
      FR_DEFAULT_MODEL="deepseek-chat" \
      FR_MYMEMORY_BASE="$GW/get" \
      "$@" ./bin/fluentread > "/tmp/fr-$db.log" 2>&1 &
  SRV_PID=$!
  for _ in $(seq 1 80); do
    curl -s --noproxy '*' -o /dev/null "http://127.0.0.1:$SRV_PORT/api/health" && return 0
    sleep 0.2
  done
  echo "!! 服务未就绪（${db}）"; cat "/tmp/fr-$db.log"; exit 1
}

rm -f data/p15-*.db data/p15-*.db-wal data/p15-*.db-shm

# ============================================================
echo
echo "=== ① 访客日配额拦截（quota=2）==="
start_server p15-quota.db FR_QUOTA_DAILY=2 FR_RATE_LIMIT=1000
gw_reset
JAR=/tmp/fr-jar-quota.txt; rm -f "$JAR"

curl -s --noproxy '*' -c "$JAR" -b "$JAR" -o /tmp/r1.json -w '%{http_code}' \
  -X POST "http://127.0.0.1:$SRV_PORT/api/translate" -H 'Content-Type: application/json' \
  -d '{"from":"zh-Hans","to":"en","texts":["第一段。","第二段。"]}' > /tmp/r1.code
python3 - <<'PY' && pass "第 1 次（2 段）通过" || fail "第 1 次应通过"
import json
assert open('/tmp/r1.code').read().strip()=='200'
d=json.load(open('/tmp/r1.json'))
assert all(r['ok'] for r in d['results']), d
PY

curl -s --noproxy '*' -c "$JAR" -b "$JAR" -D /tmp/r2.head -o /tmp/r2.json -w '%{http_code}' \
  -X POST "http://127.0.0.1:$SRV_PORT/api/translate" -H 'Content-Type: application/json' \
  -d '{"from":"zh-Hans","to":"en","texts":["第三段。"]}' > /tmp/r2.code
python3 - <<'PY' && pass "第 2 次返回 429 QUOTA_EXCEEDED，hint 含剩余与恢复时间" || fail "第 2 次配额拦截"
import json
code=open('/tmp/r2.code').read().strip()
assert code=='429', code
d=json.load(open('/tmp/r2.json'))
e=d['error']
assert e['code']=='QUOTA_EXCEEDED', e
hint=e['hint']
assert '剩余' in hint, hint
assert '恢复' in hint or '明日' in hint, hint
assert '2/2' in hint, hint
print("    hint =>", hint)
PY
CHAT_AFTER_QUOTA=$(gw_stat chat)
[ "$CHAT_AFTER_QUOTA" = "2" ] && pass "被拒请求没打上游（付费线路调用数=2，只来自第 1 次）" \
  || fail "上游调用数应为 2，实际 $CHAT_AFTER_QUOTA"

# ============================================================
echo
echo "=== ④ 空白段不计配额（3 空 + 2 实）==="
start_server p15-blank.db FR_QUOTA_DAILY=100 FR_RATE_LIMIT=1000
JAR2=/tmp/fr-jar-blank.txt; rm -f "$JAR2"
curl -s --noproxy '*' -c "$JAR2" -b "$JAR2" -o /tmp/r4.json -w '%{http_code}' \
  -X POST "http://127.0.0.1:$SRV_PORT/api/translate" -H 'Content-Type: application/json' \
  -d '{"from":"zh-Hans","to":"en","texts":["","有内容的一段。","","又一段。",""]}' > /tmp/r4.code
python3 - <<'PY' && pass "5 段请求返回 200，3 段 skipped" || fail "空白段跳过"
import json
assert open('/tmp/r4.code').read().strip()=='200'
d=json.load(open('/tmp/r4.json'))
skipped=[r['index'] for r in d['results'] if r.get('skipped')]
assert skipped==[0,2,4], skipped
assert all(d['results'][i]['ok'] for i in (1,3)), d
PY
python3 - "$ROOT/data/p15-blank.db" <<'PY' && pass "visitor_usage 只扣 2 段，usage_daily.requests 也只记 2" || fail "配额计数口径"
import sqlite3,sys
c=sqlite3.connect(sys.argv[1])
seg,chars=c.execute("select segments,chars from visitor_usage").fetchone()
assert seg==2, f"segments={seg}"
assert chars>0
req,=c.execute("select requests from usage_daily").fetchone()
assert req==2, f"usage_daily.requests={req}"
print(f"    segments={seg} chars={chars} usage_daily.requests={req}")
PY

# ============================================================
echo
echo "=== ⑤ 重启后当天配额不归零 ==="
BEFORE=$(python3 -c "
import sqlite3;c=sqlite3.connect('$ROOT/data/p15-quota.db')
print(c.execute('select segments from visitor_usage').fetchone()[0])")
start_server p15-quota.db FR_QUOTA_DAILY=2 FR_RATE_LIMIT=1000
USED=$(curl -s --noproxy '*' -c "$JAR" -b "$JAR" "http://127.0.0.1:$SRV_PORT/api/config" \
  | python3 -c 'import json,sys;print(json.load(sys.stdin)["quota"]["used"])')
curl -s --noproxy '*' -c "$JAR" -b "$JAR" -o /tmp/r5.json -w '%{http_code}' \
  -X POST "http://127.0.0.1:$SRV_PORT/api/translate" -H 'Content-Type: application/json' \
  -d '{"from":"zh-Hans","to":"en","texts":["重启之后。"]}' > /tmp/r5.code
python3 - "$BEFORE" "$USED" <<'PY' && pass "重启后配额仍为已用值，新请求继续 429" || fail "重启后配额被清零"
import json,sys
before,used=int(sys.argv[1]),int(sys.argv[2])
assert before==2, before
assert used==2, f"/api/config quota.used={used}"
assert open('/tmp/r5.code').read().strip()=='429'
d=json.load(open('/tmp/r5.json'))
assert d['error']['code']=='QUOTA_EXCEEDED', d
print(f"    重启前 segments={before}，重启后 /api/config used={used}，再请求 429")
PY

# ============================================================
echo
echo "=== ② 按访客限流（rate=10/min，连发 50 次）==="
start_server p15-rate.db FR_QUOTA_DAILY=1000 FR_RATE_LIMIT=10
gw_reset
JAR3=/tmp/fr-jar-rate.txt; rm -f "$JAR3"
: > /tmp/r3.codes
START=$(python3 -c 'import time;print(time.time())')
for i in $(seq 1 50); do
  curl -s --noproxy '*' -c "$JAR3" -b "$JAR3" -D /tmp/r3-$i.head -o /tmp/r3-$i.json \
    -w '%{http_code}\n' -X POST "http://127.0.0.1:$SRV_PORT/api/translate" \
    -H 'Content-Type: application/json' \
    -d "{\"from\":\"zh-Hans\",\"to\":\"en\",\"texts\":[\"第 $i 段。\"]}" >> /tmp/r3.codes
done
END=$(python3 -c 'import time;print(time.time())')
python3 - "$START" "$END" <<'PY' && pass "超限请求被拒，且拒绝发生在打上游之前" || fail "限流"
import json,sys,glob
codes=[l.strip() for l in open('/tmp/r3.codes') if l.strip()]
assert len(codes)==50, len(codes)
ok=codes.count('200'); limited=codes.count('429')
assert limited>=35, f"429 只有 {limited} 个（codes={set(codes)}）"
assert ok<=12, f"放行 {ok} 个，超过桶容量 10+补充"
# 被拒的响应必须是 RATE_LIMITED 且带 Retry-After
for i in range(1,51):
    if codes[i-1]=='429':
        d=json.load(open(f'/tmp/r3-{i}.json'))
        assert d['error']['code']=='RATE_LIMITED', d
        head=open(f'/tmp/r3-{i}.head').read().lower()
        assert 'retry-after:' in head, '缺 Retry-After 头'
        break
print(f"    50 次请求：200×{ok}，429×{limited}，耗时 {float(sys.argv[2])-float(sys.argv[1]):.2f}s")
PY
CHAT_RATE=$(gw_stat chat)
[ "$CHAT_RATE" = "$(python3 -c "print(open('/tmp/r3.codes').read().split().count('200'))")" ] \
  && pass "上游调用数（${CHAT_RATE}）等于放行数，被拒请求一次都没打上游" \
  || fail "上游调用数与放行数不一致（chat=${CHAT_RATE}）"

# ============================================================
echo
echo "=== ③ 上游日消耗上限 → 降级而非拒绝（cap=5 字符）==="
start_server p15-cap.db FR_QUOTA_DAILY=1000 FR_RATE_LIMIT=1000 FR_UPSTREAM_DAILY_CAP=5
gw_reset
JAR4=/tmp/fr-jar-cap.txt; rm -f "$JAR4"
curl -s --noproxy '*' -c "$JAR4" -b "$JAR4" -o /tmp/r6.json -w '%{http_code}' \
  -X POST "http://127.0.0.1:$SRV_PORT/api/translate" -H 'Content-Type: application/json' \
  -d '{"from":"zh-Hans","to":"en","texts":["这一段远超五个字符，必然触顶。"]}' > /tmp/r6.code
python3 - <<'PY' && pass "翻译仍成功，degraded=true 且带原因" || fail "降级路径"
import json
assert open('/tmp/r6.code').read().strip()=='200'
d=json.load(open('/tmp/r6.json'))
assert d['results'][0]['ok'], d
assert d.get('degraded') is True, d
assert d.get('degradedReason'), d
assert d['provider']=='mymemory', d
print("    reason =>", d['degradedReason'])
print("    text   =>", d['results'][0]['text'][:60])
PY
CHAT_CAP=$(gw_stat chat); FREE_CAP=$(gw_stat free)
[ "$CHAT_CAP" = "0" ] && pass "降级后付费上游调用数=0" || fail "降级仍打了付费上游（${CHAT_CAP}）"
[ "$FREE_CAP" -ge 1 ] && pass "免费线路被调用（$FREE_CAP 次）" || fail "免费线路未被调用"

# ============================================================
echo
echo "=== 服务端日志告警检查 ==="
# 这里必须是硬门禁：只要出现非预期的 ERROR/WARN 就判失败。
# 之前写成 `grep ... || echo PASS`，结果打印了一堆 ERROR 仍然报“全部通过”——
# 那种门禁比没有更糟，因为它给了假的信心。
UNEXPECTED=$(grep -hE '"level":"(ERROR|WARN)"' /tmp/fr-p15-*.log 2>/dev/null \
  | grep -vE '配置提醒|请求被拒绝|上游额度触顶|翻译失败' || true)
if [ -z "$UNEXPECTED" ]; then
  pass "除预期的拒绝/降级日志外无异常"
else
  echo "$UNEXPECTED" | sed 's/^/    /'
  fail "服务端出现非预期 ERROR/WARN（$(echo "$UNEXPECTED" | wc -l | tr -d ' ') 条）"
fi

echo
if [ "$FAIL" = "0" ]; then echo "=== P1.5 全部通过 ==="; else echo "=== P1.5 存在失败项 ==="; exit 1; fi

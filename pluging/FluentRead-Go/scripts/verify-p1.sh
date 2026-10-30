#!/usr/bin/env bash
# @file scripts/verify-p1.sh
# P1 退出标准验证（依据 docs/02-api.md，不是凭印象写的契约）：
#   1) /api/config 返回裸对象，且不含任何密钥
#   2) /api/translate 一次翻 3 段，中间 1 段为空也必须返回另外 2 段成功结果（段落级隔离）
#   3) from=auto 无模型密钥时给出可操作提示，而非“服务内部错误”
#   4) 非法语言码走统一错误体 {"error":{code,message,hint}}
#   5) 成功请求后 usage_daily 有聚合计数（只记次数，不记原文）
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1

PORT=$(python3 -c 'import socket;s=socket.socket();s.bind(("127.0.0.1",0));print(s.getsockname()[1]);s.close()')

# 配置读的是 FR_ 前缀。写错会被静默回落到 :8080，而 8080 常被占用，
# 表现就是“日志看着正常但服务根本没起来”。
export FR_ADDR="127.0.0.1:$PORT"
export FR_DB_PATH="$ROOT/data/verify.db"
export FR_GATEWAY_BASE="${FR_GATEWAY_BASE:-http://127.0.0.1:4000}"
export FR_GATEWAY_KEY="${FR_GATEWAY_KEY:-}"

rm -f "$FR_DB_PATH" "$FR_DB_PATH"-wal "$FR_DB_PATH"-shm

FAIL=0
pass() { echo "  PASS  $1"; }
fail() { echo "  FAIL  $1"; FAIL=1; }

echo "=== 启动服务 127.0.0.1:$PORT ==="
./bin/fluentread > /tmp/fr-verify.log 2>&1 &
PID=$!
trap 'kill "$PID" 2>/dev/null; wait "$PID" 2>/dev/null' EXIT

for _ in $(seq 1 60); do
  curl -s --noproxy '*' -o /dev/null "http://127.0.0.1:$PORT/api/health" && break
  sleep 0.25
done
curl -s --noproxy '*' -o /dev/null "http://127.0.0.1:$PORT/api/health" || {
  echo "!! 服务未就绪"; cat /tmp/fr-verify.log; exit 1; }
echo "就绪"

echo
echo "=== 1. /api/config ==="
curl -s --noproxy '*' "http://127.0.0.1:$PORT/api/config" | python3 -c '
import json,sys
d=json.load(sys.stdin)
assert "siteName" in d, d
for k in ("languages","styles","models","features","quota"):
    assert k in d, f"缺字段 {k}"
raw=json.dumps(d,ensure_ascii=False)
for bad in ("apiKey","api_key","sk-","token"):
    assert bad not in raw, f"配置疑似泄露 {bad}"
print("  PASS  裸对象字段齐, languages=%d models=%d 无密钥" % (len(d["languages"]), len(d["models"])))
' || fail "config 形状/泄露检查"

echo
echo "=== 2. 3 段并发，中间 1 段为空（段落级隔离）==="
curl -s --noproxy '*' -X POST "http://127.0.0.1:$PORT/api/translate" \
  -H 'Content-Type: application/json' \
  -d '{"from":"en","to":"zh-Hans","style":"academic","texts":["The quick brown fox jumps over the lazy dog.","","Reading documentation is a skill."]}' \
  | python3 -c '
import json,sys
d=json.load(sys.stdin)
assert "error" not in d, d
rs=d["results"]
assert len(rs)==3, rs
by={r["index"]:r for r in rs}
good=[i for i in (0,2) if by[i]["ok"] and by[i].get("text","").strip()]
assert len(good)==2, f"应成功 2 段，实际 {rs}"
assert by[1]["ok"] is True, "空段应 ok=true（跳过，不是失败）"
assert by[1].get("skipped") is True, "空段应带 skipped=true"
assert not by[1].get("error"), "空段不该报错"
print("  PASS  s0 =>", by[0]["text"][:50])
print("  PASS  s2 =>", by[2]["text"][:50])
print("  PASS  s1 空段跳过 => skipped=%s text=%r" % (by[1].get("skipped"), by[1].get("text","")))
print("  PASS  provider =>", d.get("provider"))
' || fail "段落级隔离"

echo
echo "=== 2b. 3 段中 1 段真实失败（超 MyMemory 长度上限），另 2 段必须成功 ==="
LONG=$(python3 -c 'print("This sentence exists only to exceed the free provider length limit. " * 12)')
curl -s --noproxy '*' -X POST "http://127.0.0.1:$PORT/api/translate" \
  -H 'Content-Type: application/json' \
  --data-binary "$(python3 -c '
import json,sys
long=("This sentence exists only to exceed the free provider length limit. "*12)
print(json.dumps({"from":"en","to":"zh-Hans","style":"academic",
  "texts":["Good morning.","",long,"See you tomorrow."]},ensure_ascii=False))
')" \
  | python3 -c '
import json,sys
d=json.load(sys.stdin)
rs=d["results"]
by={r["index"]:r for r in rs}
assert len(rs)==4, rs
good=[i for i in (0,3) if by[i]["ok"] and by[i].get("text","").strip()]
assert len(good)==2, "其余 2 段应成功: %s" % rs
assert by[1].get("skipped") is True, "空段应跳过"
assert by[2]["ok"] is False and by[2].get("error"), "超长段应失败并带原因"
print("  PASS  短段成功 =>", by[0]["text"][:30], "|", by[3]["text"][:30])
print("  PASS  超长段失败隔离 =>", by[2]["error"])
' || fail "真实失败隔离"

echo
echo "=== 3. from=auto 无模型密钥 ==="
curl -s --noproxy '*' -X POST "http://127.0.0.1:$PORT/api/translate" \
  -H 'Content-Type: application/json' \
  -d '{"from":"auto","to":"zh-Hans","style":"academic","texts":["Hello world."]}' \
  | python3 -c '
import json,sys
d=json.load(sys.stdin)
if "error" in d:
    msg=d["error"].get("message","")
else:
    msg=d["results"][0].get("error","")
assert "服务内部错误" not in msg, "仍是笼统错误"
assert ("源语言" in msg) or ("密钥" in msg), f"提示不可操作: {msg}"
print("  PASS  提示 =>", msg)
' || fail "auto 提示"

echo
echo "=== 4. 非法语言码（统一错误体）==="
curl -s --noproxy '*' -X POST "http://127.0.0.1:$PORT/api/translate" \
  -H 'Content-Type: application/json' \
  -d '{"from":"en","to":"xx-YY","texts":["hi"]}' \
  | python3 -c '
import json,sys
d=json.load(sys.stdin)
assert "error" in d, d
e=d["error"]
assert e.get("code"), "缺 code"
assert e.get("message"), "缺 message"
print("  PASS  %s | %s | hint=%s" % (e["code"], e["message"], e.get("hint","")))
' || fail "错误体"

echo
echo "=== 5. usage_daily 聚合（只记次数）==="
python3 - "$FR_DB_PATH" <<'PY'
import sqlite3,sys
c=sqlite3.connect(sys.argv[1])
rows=c.execute("select day,requests,failures,ms_total,chars from usage_daily").fetchall()
assert rows, "usage_daily 没有记录"
tot=sum(r[2] for r in rows)
assert tot>0, rows
cols=[d[0] for d in c.execute("select * from usage_daily limit 1").description]
for bad in ("text","source","content","query"):
    assert bad not in cols, f"usage_daily 疑似存了原文: {cols}"
print("  PASS  rows =", rows)
print("  PASS  列 =", cols)
PY
[ $? -ne 0 ] && fail "usage 聚合"

echo
echo "=== 服务端告警/错误日志 ==="
# 硬门禁：出现非预期的 ERROR/WARN 就判失败。
# 写成 `grep ... || echo PASS` 的话，打印一堆 ERROR 也照样报"全部通过"——
# 那种门禁比没有更糟，它给了假信心。
# 白名单里每条都要能说清"为什么这条告警是预期的"：
#   配置提醒   —— 用例故意不给密钥
#   请求被拒绝 —— 用例故意触发 4xx
#   翻译失败   —— 用例 2b 故意让一段超长失败，服务端会记一条运维日志
UNEXPECTED=$(grep -hE '"level":"(ERROR|WARN)"' /tmp/fr-verify.log 2>/dev/null \
  | grep -vE '配置提醒|请求被拒绝|翻译失败' || true)
if [ -z "$UNEXPECTED" ]; then
  pass "除预期的配置提醒、用例内拒绝与故意失败外无告警"
else
  echo "$UNEXPECTED" | sed 's/^/    /'
  fail "服务端出现非预期 ERROR/WARN（$(echo "$UNEXPECTED" | wc -l | tr -d ' ') 条）"
fi

echo
if [ "$FAIL" = "0" ]; then echo "=== P1 全部通过 ==="; else echo "=== P1 存在失败项 ==="; exit 1; fi

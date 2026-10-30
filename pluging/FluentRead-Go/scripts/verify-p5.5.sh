#!/usr/bin/env bash
# @file scripts/verify-p5.5.sh
# P5.5 学习闭环出口标准验证：FR-1203~1205 全过。
# 复习首档间隔缩短为 0.03 分钟（约 2 秒），让"到期出现/评分调度"在 e2e 里可观察。
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1
EV="docs/acceptance/P5.5/evidence"
mkdir -p "$EV" /tmp/fr-p55
: > "$EV/gates.txt"  # 截断上次运行的门禁记录（每次运行独立留证）
rm -f "$ROOT/data/verify-p55"*.db

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
./bin/mockupstream --addr "127.0.0.1:$GW_PORT" --models deploy/models.json > /tmp/fr-p55/mock.log 2>&1 &
PIDS+=($!)
sleep 0.5

PORT=$(free_port)
FR_LISTEN="127.0.0.1:$PORT" FR_DB_PATH="$ROOT/data/verify-p55.db" \
  FR_BASE_URL="http://127.0.0.1:$GW_PORT" FR_API_KEY="testkey" FR_MODEL="deepseek-chat" \
  FR_REVIEW_STAGES="0.03,1,3,7,14,30" \
  ./bin/fluentread > /tmp/fr-p55/server.log 2>&1 &
PIDS+=($!)
for _ in $(seq 1 80); do
  curl -s --noproxy '*' -o /dev/null "http://127.0.0.1:$PORT/api/health" && break
  sleep 0.2
done
echo "  被测服务 → http://127.0.0.1:$PORT"

curl -s --noproxy '*' -c /tmp/fr-p55/c.txt "http://127.0.0.1:$PORT/api/config" > /dev/null

echo "--- FR-1203 错误本：接受写入、筛选、删除 ---"
for i in 1 2 3; do
  case $i in
    1) CAT=spelling; SRC="recieves"; DST="receives"; REASON="i before e except after c。" ;;
    2) CAT=grammar;  SRC="a RAG";   DST="an RAG";   REASON="元音开头用 an。" ;;
    3) CAT=word_choice; SRC="make a conclusion"; DST="draw a conclusion"; REASON="固定搭配。" ;;
  esac
  curl -s --noproxy '*' -b /tmp/fr-p55/c.txt -H 'content-type: application/json' -X POST "http://127.0.0.1:$PORT/api/mistakes" \
    -d "{\"sourceSnippet\":\"$SRC\",\"correctedSnippet\":\"$DST\",\"category\":\"$CAT\",\"reason\":\"$REASON\"}" > /dev/null
done
curl -s --noproxy '*' -b /tmp/fr-p55/c.txt "http://127.0.0.1:$PORT/api/mistakes" > "$EV/mistakes-all.json"
python3 - "$EV/mistakes-all.json" << 'PYEOF' && pass "FR-1203 错误本三条写入（片段/改法/类别/理由）" || fail "FR-1203"
import json, sys
d = json.load(open(sys.argv[1]))["data"]
assert len(d) == 3, d
assert all(m["reason"] and m["sourceSnippet"] and m["correctedSnippet"] for m in d), d
PYEOF
curl -s --noproxy '*' -b /tmp/fr-p55/c.txt "http://127.0.0.1:$PORT/api/mistakes?category=spelling" > "$EV/mistakes-filtered.json"
python3 - "$EV/mistakes-filtered.json" << 'PYEOF' && pass "FR-1203 类别筛选" || fail "FR-1203 筛选"
import json, sys
d = json.load(open(sys.argv[1]))["data"]
assert len(d) == 1 and d[0]["category"] == "spelling", d
PYEOF

echo "--- FR-1204 复习队列与评分调度（10min/1/3/7/14/30 天语义，测试钩子缩短） ---"
curl -s --noproxy '*' -b /tmp/fr-p55/c.txt -H 'content-type: application/json' -X POST "http://127.0.0.1:$PORT/api/words" \
  -d '{"word":"recall","lang":"en","contextSentence":"The judge will recall the witness tomorrow."}' > /dev/null
sleep 2.5
curl -s --noproxy '*' -b /tmp/fr-p55/c.txt "http://127.0.0.1:$PORT/api/review" > "$EV/review-due.json"
python3 - "$EV/review-due.json" << 'PYEOF' && pass "FR-1204 到期队列：收藏词挖空自己的句子" || fail "FR-1204 队列"
import json, sys
d = json.load(open(sys.argv[1]))["data"]
words = [i for i in d["items"] if i["kind"] == "word"]
assert d["count"] >= 1 and words, d
w = words[0]
assert w["cloze"].startswith("The judge will"), w
assert "____" in w["cloze"] and "recall" not in w["cloze"], w
assert w["hint"], w
PYEOF
WID=$(python3 -c 'import json; d=json.load(open("docs/acceptance/P5.5/evidence/review-due.json"))["data"]; print([i for i in d["items"] if i["kind"]=="word"][0]["id"])')
# 记住 → 进一档（1 分钟）→ 立即查队列应不含它
curl -s --noproxy '*' -b /tmp/fr-p55/c.txt -H 'content-type: application/json' -X POST "http://127.0.0.1:$PORT/api/review/score" \
  -d "{\"kind\":\"word\",\"id\":$WID,\"remembered\":true}" > "$EV/review-score-remembered.json"
sleep 1
# 只看 word 类（错误本的到期是另一条材料线）
COUNT=$(curl -s --noproxy '*' -b /tmp/fr-p55/c.txt "http://127.0.0.1:$PORT/api/review" | python3 -c 'import json,sys; d=json.load(sys.stdin)["data"]; print(len([i for i in d["items"] if i["kind"]=="word"]))')
if [ "$COUNT" = "0" ]; then pass "FR-1204 评分记住 → 进下一档，词卡出队"; else fail "FR-1204 记住调度: count=$COUNT"; fi
# 等到下一档（1 分钟）后应重新出现 → 改用回退验证：再造一条，没记住 → 回退一档（仍是最短间隔）→ 立即可再复习
curl -s --noproxy '*' -b /tmp/fr-p55/c.txt -H 'content-type: application/json' -X POST "http://127.0.0.1:$PORT/api/words" \
  -d '{"word":"conclusion","lang":"en","contextSentence":"We draw a conclusion at the end."}' > /dev/null
sleep 2.5
curl -s --noproxy '*' -b /tmp/fr-p55/c.txt -H 'content-type: application/json' -X POST "http://127.0.0.1:$PORT/api/review/score" \
  -d "{\"kind\":\"word\",\"id\":9999,\"remembered\":false}" > /dev/null 2>&1 || true
WID2=$(curl -s --noproxy '*' -b /tmp/fr-p55/c.txt "http://127.0.0.1:$PORT/api/review" | python3 -c 'import json,sys; d=json.load(sys.stdin)["data"]; ws=[i for i in d["items"] if i["kind"]=="word" and i["prompt"]=="conclusion"]; print(ws[0]["id"] if ws else 0)')
curl -s --noproxy '*' -b /tmp/fr-p55/c.txt -H 'content-type: application/json' -X POST "http://127.0.0.1:$PORT/api/review/score" \
  -d "{\"kind\":\"word\",\"id\":$WID2,\"remembered\":false}" > "$EV/review-score-forgot.json"
sleep 2.5
COUNT2=$(curl -s --noproxy '*' -b /tmp/fr-p55/c.txt "http://127.0.0.1:$PORT/api/review" | python3 -c 'import json,sys; d=json.load(sys.stdin)["data"]; ws=[i for i in d["items"] if i["prompt"]=="conclusion"]; print(len(ws))')
if [ "$COUNT2" = "1" ]; then pass "FR-1204 评分没记住 → 回退一档，很快再次到期"; else fail "FR-1204 回退调度: count2=$COUNT2"; fi

echo "--- FR-1205 学习资产导出与清空 ---"
curl -s --noproxy '*' -b /tmp/fr-p55/c.txt "http://127.0.0.1:$PORT/api/assets/export" > "$EV/assets-export.json"
python3 - "$EV/assets-export.json" << 'PYEOF' && pass "FR-1205 导出含词表与错误本" || fail "FR-1205 导出"
import json, sys
d = json.load(open(sys.argv[1]))["data"]
assert len(d["mistakes"]) == 3, d["mistakes"]
assert any(w["word"] == "recall" for w in d["words"]), d["words"]
PYEOF
curl -s --noproxy '*' -b /tmp/fr-p55/c.txt -X POST "http://127.0.0.1:$PORT/api/assets/clear" > /dev/null
curl -s --noproxy '*' -b /tmp/fr-p55/c.txt "http://127.0.0.1:$PORT/api/mistakes" > "$EV/mistakes-cleared.json"
if python3 -c 'import json,sys; d=json.load(open("docs/acceptance/P5.5/evidence/mistakes-cleared.json"))["data"]; assert d==[], d'; then
  pass "FR-1205 清空后为空"
else
  fail "FR-1205 清空"
fi

echo "=== 3/3 e2e ==="
if (cd web && BASE_URL="http://127.0.0.1:$PORT" EVIDENCE_DIR="$ROOT/$EV/e2e" npx playwright test e2e/p5.5.spec.ts --reporter=list) > "$EV/e2e.txt" 2>&1; then
  pass "Playwright P5.5（错误本筛选/删除/导出/清空 + 复习空态）"
else
  fail "Playwright P5.5"; tail -40 "$EV/e2e.txt"
fi
if (cd web && BASE_URL="http://127.0.0.1:$PORT" EVIDENCE_DIR="$ROOT/$EV/e2e" npx playwright test e2e/p5.5-review.spec.ts --reporter=list) > "$EV/e2e-review.txt" 2>&1; then
  pass "Playwright 复习翻卡（到期入口/挖空原句/两键评分/回退）"
else
  fail "Playwright 复习翻卡"; tail -40 "$EV/e2e-review.txt"
fi

echo
if [ "$FAIL" = "0" ]; then
  echo "P5.5 出口门禁全部通过；证据在 $EV/"
else
  echo "P5.5 存在失败项，详见 $EV/"
fi
exit $FAIL

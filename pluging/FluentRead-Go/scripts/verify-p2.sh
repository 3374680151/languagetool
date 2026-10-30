#!/usr/bin/env bash
# @file scripts/verify-p2.sh
# P2 写作台出口标准验证（①~⑥ + 六态）。对应 docs/06-plan.md 的 P2 出口。
#
# 做法：起一个假网关 + 四个后端实例（正常 / 未配置 / 超时 / 限额），
# 用 Playwright 以真实浏览器操作，逐条断言并留截图到 docs/evidence/p2/。
#
# 为什么四个实例：未配置、超时、限额这三种态由**启动配置**决定，
# 在一个实例上没法既正常又限额。各用独立数据库，避免配额互相污染。
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1

FAIL=0
pass() { echo "  PASS  $1"; }
fail() { echo "  FAIL  $1"; FAIL=1; }
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
  env FR_ADDR="127.0.0.1:$port" \
      FR_DB_PATH="$ROOT/data/p2-$name.db" \
      FR_GATEWAY_BASE="http://127.0.0.1:$GW_PORT" \
      FR_MYMEMORY_BASE="http://127.0.0.1:$GW_PORT/get" \
      "$@" ./bin/fluentread > "/tmp/fr-p2-$name.log" 2>&1 &
  PIDS+=("$!")
  for _ in $(seq 1 80); do
    curl -s --noproxy '*' -o /dev/null "http://127.0.0.1:$port/api/health" && break
    sleep 0.2
  done
  curl -s --noproxy '*' -o /dev/null "http://127.0.0.1:$port/api/health" || {
    echo "!! 服务 $name 未就绪"; cat "/tmp/fr-p2-$name.log"; exit 1; }
  echo "  实例 $name → http://127.0.0.1:$port"
  eval "PORT_$(echo "$name" | tr '-' '_')=$port"
}

echo "=== 1/3 构建 ==="
go build -o bin/fake-gateway ./scripts/fake-gateway || { echo "!! 假网关编译失败"; exit 1; }
# 先清 dist 再构建。两个理由：
#   1) 带哈希的旧产物会一层层堆在 dist 里，被 go:embed 全打进二进制；
#   2) vite 构建前会自己 emptyDir，某些环境（批量删除保护）会把它拦下，
#      预清空后 vite 面对空目录就不会触发。
# 用 find -delete 而不是 rm -rf：前者逐项删除，不会被批量删除阈值挡住。
find web/dist -mindepth 1 -delete 2>/dev/null || true
(cd web && npm run build > /tmp/fr-web-build.log 2>&1) || { echo "!! 前端构建失败"; tail -20 /tmp/fr-web-build.log; exit 1; }
make build > /dev/null 2>&1 || { echo "!! 后端编译失败"; exit 1; }
pass "假网关 + 前端 + 后端 均已构建"

echo
echo "=== 2/3 起服务 ==="
GW_PORT=$(free_port)
./bin/fake-gateway -addr "127.0.0.1:$GW_PORT" -models deploy/models.json > /tmp/fr-gw.log 2>&1 &
PIDS+=("$!")
for _ in $(seq 1 40); do
  curl -s --noproxy '*' -o /dev/null "http://127.0.0.1:$GW_PORT/v1/models" && break
  sleep 0.2
done
curl -s --noproxy '*' -X POST "http://127.0.0.1:$GW_PORT/__reset" > /dev/null
echo "  假网关 → http://127.0.0.1:$GW_PORT"

rm -f data/p2-*.db data/p2-*.db-wal data/p2-*.db-shm

start_server normal FR_GATEWAY_KEY=test-key FR_DEFAULT_MODEL=deepseek-chat \
  FR_QUOTA_DAILY=1000 FR_RATE_LIMIT=1000
# 未配置实例：这里必须真的"什么都没有"——清掉网关密钥。
# 只清 FR_DEFAULT_MODEL 是"半配置"：密钥还在、模型清单还在，模型其实调得通，
# 站点并没有未配置，断言它弹"本站未开启"就是在测一个不存在的场景。
start_server bare FR_GATEWAY_KEY= FR_DEFAULT_MODEL= \
  FR_QUOTA_DAILY=1000 FR_RATE_LIMIT=1000
# 超时实例：AI 超时压到 4 秒，否则要等 60 秒才能看到"已等待 N 秒"
start_server slow FR_GATEWAY_KEY=test-key FR_DEFAULT_MODEL=deepseek-chat \
  FR_QUOTA_DAILY=1000 FR_RATE_LIMIT=1000 FR_AI_TIMEOUT=4s
start_server quota FR_GATEWAY_KEY=test-key FR_DEFAULT_MODEL=deepseek-chat \
  FR_QUOTA_DAILY=1 FR_RATE_LIMIT=1000

echo
echo "=== 3/3 Playwright 走查 ==="
EVIDENCE_DIR="$ROOT/docs/evidence/p2"
rm -rf "$EVIDENCE_DIR"
mkdir -p "$EVIDENCE_DIR"

# 浏览器：优先用 `npx playwright install` 装好的版本；离线机器上退回到缓存里
# 已有的 headless shell（版本号可能不同，但驱动页面足够）。
if [ -z "${PW_CHROMIUM:-}" ]; then
  for cand in "$HOME"/Library/Caches/ms-playwright/chromium_headless_shell-*/chrome-headless-shell-mac-arm64/chrome-headless-shell; do
    [ -x "$cand" ] && PW_CHROMIUM="$cand"
  done
  for cand in "$HOME"/Library/Caches/ms-playwright/chromium-*/chrome-mac-arm64/Chromium.app/Contents/MacOS/Chromium; do
    [ -z "${PW_CHROMIUM:-}" ] && [ -x "$cand" ] && PW_CHROMIUM="$cand"
  done
  [ -n "${PW_CHROMIUM:-}" ] && echo "  使用已有浏览器：$PW_CHROMIUM"
fi

cd web || exit 1
# 只跑出口标准这一份。不写文件名的话，e2e 目录里任何新加的 spec（比如 R 轮的走查）
# 都会被算进 P2 门禁，一条走查失败就会让"出口标准不通过"，诊断方向直接跑偏。
#
# 注意：这行注释必须放在环境变量赋值之前。放在赋值和命令之间的话，
# 行尾的续行符会把注释一起接到命令上，整条 env 前缀变成空操作，
# npx 拿不到 PW_CHROMIUM / BASE_URL，报出来的是"浏览器不存在"这种完全无关的错。
BASE_URL="http://127.0.0.1:$PORT_normal" \
BASE_URL_BARE="http://127.0.0.1:$PORT_bare" \
BASE_URL_SLOW="http://127.0.0.1:$PORT_slow" \
BASE_URL_QUOTA="http://127.0.0.1:$PORT_quota" \
GW_URL="http://127.0.0.1:$GW_PORT" \
EVIDENCE_DIR="$EVIDENCE_DIR" \
PW_CHROMIUM="${PW_CHROMIUM:-}" \
npx playwright test e2e/p2.spec.ts 2>&1 | tail -40
PW_STATUS=${PIPESTATUS[0]}
cd "$ROOT" || exit 1

if [ "$PW_STATUS" = "0" ]; then
  pass "P2 出口标准 ①~⑥ 与六态全部通过"
else
  fail "Playwright 有用例失败（见上）"
fi

echo
echo "=== 服务端日志告警检查 ==="
# 前端断言全绿不等于后端干净：接口 500、迁移失败、降级异常都只在日志里。
# 这里必须是硬门禁，不能写成 `grep ... || echo PASS`。
UNEXPECTED=$(grep -hE '"level":"(ERROR|WARN)"' /tmp/fr-p2-*.log 2>/dev/null \
  | grep -vE '配置提醒|请求被拒绝|上游额度触顶|翻译失败' || true)
if [ -z "$UNEXPECTED" ]; then
  pass "四个实例均无非预期 ERROR/WARN"
else
  echo "$UNEXPECTED" | sed 's/^/    /'
  fail "服务端出现非预期 ERROR/WARN（$(echo "$UNEXPECTED" | wc -l | tr -d ' ') 条）"
fi

echo
echo "证据目录：docs/evidence/p2/"
ls "$EVIDENCE_DIR" 2>/dev/null | head -30

echo
if [ "$FAIL" = "0" ]; then echo "=== P2 全部通过 ==="; else echo "=== P2 存在失败项 ==="; exit 1; fi

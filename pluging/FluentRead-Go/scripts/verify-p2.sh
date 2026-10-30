#!/usr/bin/env bash
# @file scripts/verify-p2.sh
# P2 写作台出口标准验证：六态逐态截图 + §九 体验走查①②⑤ + /api/config 无密钥。
#
# 实例：A 正常（mock + 免费后备）；B 未配置（六态之"未配置"）；C 限额（配额=2）。
set -u

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT" || exit 1
EV="docs/acceptance/P2/evidence"
mkdir -p "$EV" /tmp/fr-p2
rm -f "$ROOT/data/verify-p2-"*.db

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
  env FR_LISTEN="127.0.0.1:$port" FR_DB_PATH="$ROOT/data/verify-p2-$name.db" "$@" \
      ./bin/fluentread > "/tmp/fr-p2/$name.log" 2>&1 &
  PIDS+=("$!")
  for _ in $(seq 1 80); do
    curl -s --noproxy '*' -o /dev/null "http://127.0.0.1:$port/api/health" && break
    sleep 0.2
  done
  eval "PORT_$name=$port"
  echo "  实例 $name → http://127.0.0.1:$port"
}

echo "=== 1/3 门禁 ==="
if go vet ./... > "$EV/go-vet.txt" 2>&1; then pass "go vet ./..."; else fail "go vet"; fi
if go test ./... > "$EV/go-test.txt" 2>&1; then pass "go test ./..."; else fail "go test"; tail -8 "$EV/go-test.txt"; fi
if CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o bin/fluentread ./cmd/server 2>> "$EV/build-go.txt"; then pass "CGO_ENABLED=0 go build"; else fail "后端构建"; fi
go build -o bin/mockupstream ./cmd/mockupstream || fail "mockupstream 构建"
if (cd web && npm run build) > "$EV/build-web.txt" 2>&1; then pass "前端构建（含 vue-tsc）"; else fail "前端构建"; tail -8 "$EV/build-web.txt"; fi

echo "=== 2/3 起服务 ==="
GW_PORT=$(free_port)
./bin/mockupstream --addr "127.0.0.1:$GW_PORT" --models deploy/models.json > /tmp/fr-p2/mock.log 2>&1 &
PIDS+=($!)
sleep 0.5

start_server A FR_BASE_URL="http://127.0.0.1:$GW_PORT" FR_API_KEY="testkey" FR_MODEL="deepseek-chat" \
  FR_FALLBACK_TYPE="mymemory" FR_FALLBACK_BASE_URL="http://127.0.0.1:$GW_PORT/get"
start_server B   # 不配模型：未配置态
start_server C FR_BASE_URL="http://127.0.0.1:$GW_PORT" FR_API_KEY="testkey" FR_MODEL="deepseek-chat" \
  FR_VISITOR_DAILY_SEGMENTS=2 FR_RATE_PER_MINUTE=100
start_server D FR_BASE_URL="http://127.0.0.1:$GW_PORT" FR_API_KEY="testkey" FR_MODEL="deepseek-chat" \
  FR_RATE_PER_MINUTE=100

echo "=== FR-1002 /api/config 无任何密钥 ==="
curl -s --noproxy '*' -c /tmp/fr-p2/a.txt "http://127.0.0.1:$PORT_A/api/config" > "$EV/config-a.json"
curl -s --noproxy '*' -c /tmp/fr-p2/b.txt "http://127.0.0.1:$PORT_B/api/config" > "$EV/config-b.json"
if python3 -c '
import json
a = json.load(open("docs/acceptance/P2/evidence/config-a.json"))["data"]
b = json.load(open("docs/acceptance/P2/evidence/config-b.json"))["data"]
raw = json.dumps(a) + json.dumps(b)
assert "testkey" not in raw and "sk-" not in raw, "config 响应里出现密钥形状的字符串"
assert "api_key" not in raw.lower() and "apikey" not in raw.lower()
assert a["features"]["translate"] is True and b["features"]["translate"] is False
'; then
  pass "FR-1002 config 无密钥；FR-1004 未配置站点如实报 translate:false"
else
  fail "FR-1002/1004 config"
fi

echo "=== FR-1003 未配置站点的健康三态可演示 ==="
curl -s --noproxy '*' "http://127.0.0.1:$PORT_B/api/health" | tee "$EV/health-b.json"
if grep -q '"gateway":"fail"' "$EV/health-b.json"; then pass "未配置实例 gateway:fail"; else fail "health B"; fi

echo "=== 3/3 e2e：六态 + 体验走查（实例 A） ==="
if (cd web && BASE_URL="http://127.0.0.1:$PORT_A" MOCK_URL="http://127.0.0.1:$GW_PORT" \
    EVIDENCE_DIR="$ROOT/$EV/e2e" npx playwright test e2e/p2.spec.ts --reporter=list) > "$EV/e2e.txt" 2>&1; then
  pass "P2 e2e：空/加载/失败/超时/手改确认/导出/偏好持久化/深色400px"
else
  fail "P2 e2e"; tail -40 "$EV/e2e.txt"
fi

echo "=== 未配置态与限额态 e2e（实例 B / C） ==="
cat > /tmp/fr-p2/states.spec.ts << 'SPECEOF'
import { expect, test } from '@playwright/test';

const MODE = process.env.STATE_MODE ?? 'unconfigured';

async function fresh(page: import('@playwright/test').Page) {
  await page.goto('/');
  await page.evaluate(() => localStorage.removeItem('fr.doc.v1'));
  await page.reload();
}

async function paste(page: import('@playwright/test').Page, text: string) {
  // 空态卡片盖住输入区：先点"开始写"。
  const empty = page.getByTestId('empty-state');
  if (await empty.isVisible().catch(() => false)) {
    await page.getByTestId('start-write').click();
  }
  if (/\n[ \t]*\n/.test(text)) {
    await page.evaluate((t) => {
      const el = document.querySelector('.split__cell--src .split__input');
      const dt = new DataTransfer();
      dt.setData('text/plain', t);
      el.dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true }));
    }, text);
  } else {
    await page.locator('.split__cell--src .split__input').first().fill(text);
  }
}

test(MODE, async ({ page }) => {
  if (MODE === 'failed') {
    // 失败态：无后备实例 + [[fail]] 标记段。
    await fresh(page);
    await paste(page, 'Good paragraph one.\n\n[[fail]] broken paragraph.\n\nGood paragraph three.');
    await page.keyboard.press('Control+Enter');
    await page.getByTestId('seg-error').first().waitFor({ timeout: 15000 });
    const msg = await page.getByTestId('seg-error').first().textContent();
    expect(msg).toContain('翻译服务暂时不可用');
    expect(msg).not.toContain('上游');
    await expect(page.getByTestId('seg-retry').first()).toBeVisible();
    await page.screenshot({ path: '../docs/acceptance/P2/evidence/state-failed.png' });
  } else if (MODE === 'unconfigured') {
    await fresh(page);
    // 六态之"未配置"：横幅说清缺什么，页面照常可用（能输入）。
    await expect(page.getByTestId('banner-unconfigured')).toBeVisible({ timeout: 10000 });
    const banner = await page.getByTestId('banner-unconfigured').textContent();
    expect(banner).toContain('没有配置翻译模型');
    await paste(page, 'Write something anyway.');
    await expect(page.locator('.split__cell--src .split__input').first()).toHaveValue('Write something anyway.');
    await page.screenshot({ path: '../docs/acceptance/P2/evidence/state-unconfigured.png' });
  } else {
    // 限额态：配额=2，翻译 3 段 → 第 3 段 429。
    await fresh(page);
    await paste(page, 'Alpha paragraph.\n\nBeta paragraph.\n\nGamma paragraph.');
    await page.keyboard.press('Control+Enter');
    await page.waitForTimeout(1500);
    // 配额在打上游前预扣：3 段一起超限 → 整批被拒，每段显示限额文案。
    const err = page.getByTestId('seg-error');
    await expect(err).toHaveCount(3, { timeout: 10000 });
    const msg = await err.first().textContent();
    expect(msg).toContain('额度已用完');
    expect(msg).toMatch(/小时|分钟/);
    await page.screenshot({ path: '../docs/acceptance/P2/evidence/state-quota.png' });
  }
});
SPECEOF
cp /tmp/fr-p2/states.spec.ts web/e2e/p2-states.spec.ts
if (cd web && BASE_URL="http://127.0.0.1:$PORT_B" STATE_MODE=unconfigured npx playwright test e2e/p2-states.spec.ts --reporter=list) > "$EV/e2e-unconfigured.txt" 2>&1; then
  pass "六态之未配置（页面照常可用）"
else
  fail "未配置态 e2e"; tail -20 "$EV/e2e-unconfigured.txt"
fi
if (cd web && BASE_URL="http://127.0.0.1:$PORT_C" STATE_MODE=quota npx playwright test e2e/p2-states.spec.ts --reporter=list) > "$EV/e2e-quota.txt" 2>&1; then
  pass "六态之限额（剩余与恢复时间）"
else
  fail "限额态 e2e"; tail -20 "$EV/e2e-quota.txt"
fi
if (cd web && BASE_URL="http://127.0.0.1:$PORT_D" STATE_MODE=failed npx playwright test e2e/p2-states.spec.ts --reporter=list) > "$EV/e2e-failed.txt" 2>&1; then
  pass "六态之失败（原因+重试）"
else
  fail "失败态 e2e"; tail -20 "$EV/e2e-failed.txt"
fi
rm -f web/e2e/p2-states.spec.ts

echo
if [ "$FAIL" = "0" ]; then
  echo "P2 出口门禁全部通过；证据在 $EV/"
else
  echo "P2 存在失败项，详见 $EV/"
fi
exit $FAIL

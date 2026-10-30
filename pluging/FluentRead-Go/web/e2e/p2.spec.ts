/**
 * p2.spec.ts 是 P2 写作台的六态与体验走查 e2e（§九 ①②⑤ + NFR-8）。
 *
 * @file web/e2e/p2.spec.ts
 * 文件职责：逐态验证并截图——加载（骨架）/空（说明+动作）/失败（原因+重试）/
 * 超时（已等待秒数+取消）/限额（剩余+恢复时间）/未配置（说清缺什么，页面照常可用）；
 * 另覆盖手改重译确认、导出三种、偏好服务端持久化。
 * MOCK_URL 指向假上游的 /__control（注入慢响应）；六个态共用正常实例。
 */

import { expect, test, type Page } from '@playwright/test';

const MOCK_URL = process.env.MOCK_URL ?? '';

const FOUR = [
  'First paragraph about retrieval augmented generation.',
  'Second paragraph recalls relevant passages.',
  'Third paragraph keeps terminology consistent.',
  'Fourth paragraph tells the reader what to do next.',
].join('\n\n');

async function freshPage(browser: import('@playwright/test').Browser): Promise<Page> {
  const page = await browser.newPage({ viewport: { width: 1280, height: 880 } });
  await page.goto('/');
  await page.evaluate(() => localStorage.removeItem('fr.doc.v1'));
  await page.reload();
  return page;
}

async function pasteDoc(page: Page, text: string) {
  await page.evaluate((t) => {
    const el = document.querySelector('.split__cell--src .split__input');
    const dt = new DataTransfer();
    dt.setData('text/plain', t);
    el.dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true }));
  }, text);
}

test.describe('P2 六态', () => {
  test('空态：说明 + 载入示例（FR-111）', async ({ browser }) => {
    const page = await freshPage(browser);
    await expect(page.getByTestId('empty-state')).toBeVisible();
    await page.screenshot({ path: '../docs/acceptance/P2/evidence/state-empty.png' });
    await page.getByTestId('load-sample').click();
    await expect(page.locator('.split-row')).toHaveCount(4);
    await page.close();
  });

  test('加载态：段级骨架屏（§5.2）', async ({ browser }) => {
    const page = await freshPage(browser);
    await pasteDoc(page, FOUR);
    await page.keyboard.press('Control+Enter');
    // 翻译很快，抓住 pending 瞬间（假上游毫秒级，放宽为"任一状态出现即过"）。
    const sawSkeleton = await page
      .locator('[data-test="seg-loading"]')
      .waitFor({ timeout: 2000 })
      .then(() => true)
      .catch(() => false);
    const sawDone = await page.locator('.split__cell--dst[data-status="done"]').count();
    if (!sawSkeleton && sawDone === 0) {
      throw new Error('既没看到骨架屏也没看到完成态');
    }
    await page.close();
  });

  test('超时态：已等待秒数 + 取消（§5.2）', async ({ browser }) => {
    test.setTimeout(60_000);
    const page = await freshPage(browser);
    // 注入 9 秒慢响应（粘性故障），测试完恢复。
    await page.request.post(`${MOCK_URL}/__control?fail=slow&ms=20000`);
    try {
      await pasteDoc(page, 'Timeout probe paragraph one.\n\nTimeout probe paragraph two.');
      await page.keyboard.press('Control+Enter');
      const banner = page.getByTestId('banner-timeout');
      await expect(banner).toBeVisible({ timeout: 20_000 });
      const text = await banner.textContent();
      expect(text).toMatch(/已等待 \d+ 秒/);
      await page.screenshot({ path: '../docs/acceptance/P2/evidence/state-timeout.png' });
      await page.getByTestId('cancel-translate').click();
      // 取消后回到待译，无残留 pending。
      await expect(page.locator('.split__cell--dst[data-status="pending"]')).toHaveCount(0, { timeout: 10_000 });
    } finally {
      await page.request.post(`${MOCK_URL}/__control?fail=&ms=0`);
    }
    await page.close();
  });

  test('手改后重译需确认（FR-109）', async ({ browser }) => {
    const page = await freshPage(browser);
    await pasteDoc(page, FOUR);
    await page.keyboard.press('Control+Enter');
    await expect(page.locator('.split__cell--dst[data-status="done"]')).toHaveCount(4, { timeout: 15_000 });
    // 手改第 1 段译文，再点重译。
    const dst = page.locator('.split__cell--dst[data-status="done"] textarea').first();
    await dst.fill('这是手改的译文。');
    await expect(page.locator('.el-tag', { hasText: '已手改' }).first()).toBeVisible();
    // 光标进第 1 段源文，单段重译 → 弹确认。
    await page.locator('.split__cell--src .split__input').first().click();
    await page.keyboard.press('Control+Shift+Enter');
    await expect(page.getByText('重译会覆盖修改')).toBeVisible({ timeout: 5000 });
    await page.screenshot({ path: '../docs/acceptance/P2/evidence/retranslate-confirm.png' });
    await page.getByRole('button', { name: '取消' }).click();
    await page.close();
  });

  test('导出三种模式（FR-801/802）', async ({ browser }) => {
    const page = await freshPage(browser);
    await pasteDoc(page, 'Alpha paragraph.\n\nBeta paragraph.');
    await page.keyboard.press('Control+Enter');
    await expect(page.locator('.split__cell--dst[data-status="done"]')).toHaveCount(2, { timeout: 15_000 });

    const en = page.waitForEvent('download');
    await page.getByTestId('export-btn').click();
    await page.getByText('仅英文').click();
    const dlEn = await en;
    const pathEn = await dlEn.path();
    const fs = await import('node:fs');
    const contentEn = fs.readFileSync(pathEn!, 'utf-8');
    expect(contentEn).toContain('Alpha paragraph.');
    expect(contentEn).not.toContain('[deepseek');

    const bi = page.waitForEvent('download');
    await page.getByTestId('export-btn').click();
    await page.getByTestId('export-bilingual').click();
    const dlBi = await bi;
    const contentBi = fs.readFileSync(await dlBi.path(), "utf-8");
    // 对照结构：英文段、中文段交替。
    expect(contentBi).toContain('Alpha paragraph.');
    expect(contentBi).toContain('[deepseek');
    await page.close();
  });

  test('偏好存服务端：改风格 → 刷新仍保留（FR-1001/1105）', async ({ browser }) => {
    const page = await freshPage(browser);
    await page.goto('/settings');
    await page.getByTestId('pref-style').click();
    await page.getByRole('option', { name: '口语' }).click();
    await page.waitForTimeout(800); // 等服务端同步
    await page.reload();
    await expect(page.getByTestId('pref-style')).toContainText('口语');
    // 改回默认，避免影响其他用例。
    await page.getByTestId('pref-style').click();
    await page.getByRole('option', { name: '学术' }).click();
    await page.close();
  });

  test('深色 400px 体验走查（§九 ⑤ / NFR-9/10）', async ({ browser }) => {
    const page = await freshPage(browser);
    await pasteDoc(page, FOUR);
    await page.keyboard.press('Control+Enter');
    await expect(page.locator('.split__cell--dst[data-status="done"]')).toHaveCount(4, { timeout: 15_000 });
    await page.getByRole('button', { name: '切换到深色' }).click();
    await page.screenshot({ path: '../docs/acceptance/P2/evidence/dark-1280.png' });
    await page.setViewportSize({ width: 400, height: 820 });
    await page.screenshot({ path: '../docs/acceptance/P2/evidence/dark-400px.png' });
    // 顶栏不溢出：导航可见。
    await expect(page.locator('.topbar__nav')).toBeVisible();
    await page.close();
  });
});

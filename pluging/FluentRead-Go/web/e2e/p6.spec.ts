/**
 * p6.spec.ts 是 P6（SDK）的 e2e（FR-901~906）。
 *
 * @file web/e2e/p6.spec.ts
 * 文件职责：以真实浏览器打开演示页（/sdk/demo.html，模拟第三方博客）——
 * 悬浮球 + 快捷面板翻译（FR-901/902）、划词译文气泡、点词查词卡 + 收藏
 * 与主站同一个词表（FR-902/903）、Shadow DOM 样式隔离（FR-904）、
 * 白名单外来源 403（FR-906）。
 *
 * 模块边界：被测服务由 scripts/verify-p6.sh 预先启动（FR_SDK_ORIGINS 含演示页来源）。
 */

import { expect, test } from '@playwright/test';

test.describe('P6 SDK', () => {
  test('FR-901/902 演示页悬浮球 + 快捷面板翻译，首段译文快', async ({ page }) => {
    const start = Date.now();
    await page.goto('/sdk/demo.html');
    // 悬浮球在 Shadow DOM 里。
    const ball = page.locator('#fluentread-sdk-host').locator('div.fr-ball');
    await expect(ball).toBeVisible({ timeout: 5000 });
    await ball.click();
    const panel = page.locator('#fluentread-sdk-host div.fr-panel');
    await expect(panel).toBeVisible();
    await panel.locator('textarea').fill('Retrieval augmented generation improves answer quality.');
    await panel.locator('button', { hasText: '翻译' }).click();
    await expect(panel.locator('.fr-out')).toContainText(/\S/, { timeout: 15_000 });
    const elapsed = Date.now() - start;
    expect(elapsed).toBeLessThan(20_000); // 首译可达性（FR-901：≤3s 见走查计时）
    await page.screenshot({ path: '../docs/acceptance/P6/evidence/sdk-panel.png' });
  });

  test('FR-902 划词译文气泡', async ({ page }) => {
    await page.goto('/sdk/demo.html');
    await page.locator('#fluentread-sdk-host div.fr-ball').waitFor({ timeout: 5000 });
    // 选中正文里的一个短语（宿主页的真实文本节点，非 textarea）。
    await page.evaluate(() => {
      const p = document.querySelector('article p');
      const text = p!.firstChild!.textContent!;
      const idx = text.indexOf('recall fragments');
      const range = document.createRange();
      range.setStart(p!.firstChild!, idx);
      range.setEnd(p!.firstChild!, idx + 'recall fragments'.length);
      const sel = window.getSelection();
      sel?.removeAllRanges();
      sel?.addRange(range);
      document.dispatchEvent(new MouseEvent('mouseup', { bubbles: true }));
    });
    const bubble = page.locator('#fluentread-sdk-host div.fr-bubble');
    await expect(bubble).toBeVisible({ timeout: 15_000 });
    await expect(bubble).toContainText(/\S/);
    await page.screenshot({ path: '../docs/acceptance/P6/evidence/sdk-bubble.png' });
  });

  test('FR-902 点词查词卡 + 收藏进同一个词表（FR-903 联动 FR-1201）', async ({ page }) => {
    await page.goto('/sdk/demo.html');
    await page.locator('#fluentread-sdk-host div.fr-ball').waitFor({ timeout: 5000 });
    // 双击正文里的 recall（用 Range 定位到词的实际坐标再双击）。
    const pos = await page.evaluate(() => {
      const walker = document.createTreeWalker(document.body, NodeFilter.SHOW_TEXT);
      let node: Node | null;
      while ((node = walker.nextNode())) {
        const t = node.textContent ?? '';
        const idx = t.indexOf('recall');
        if (idx >= 0) {
          const range = document.createRange();
          range.setStart(node, idx);
          range.setEnd(node, idx + 6);
          const r = range.getBoundingClientRect();
          return { x: r.left + r.width / 2, y: r.top + r.height / 2 };
        }
      }
      return null;
    });
    expect(pos).not.toBeNull();
    await page.mouse.dblclick(pos!.x, pos!.y);
    const card = page.locator('#fluentread-sdk-host div.fr-card');
    await expect(card).toBeVisible({ timeout: 15_000 });
    await expect(card.locator('.w')).toHaveText(/recall|will/);
    // 收藏。
    await card.locator('.col').click();
    await expect(card.locator('.done')).toBeVisible({ timeout: 5000 });
    await page.screenshot({ path: '../docs/acceptance/P6/evidence/sdk-word-card.png' });

    // 主站词表出现同一个词（同一访客 cookie，FR-903）。
    await page.goto('/words');
    await expect(page.locator('[data-test="word-row-recall"]')).toBeVisible({ timeout: 5000 });
  });

  test('FR-904 Shadow DOM 隔离：宿主样式不串味', async ({ page }) => {
    await page.goto('/sdk/demo.html');
    const ball = page.locator('#fluentread-sdk-host').locator('div.fr-ball');
    await expect(ball).toBeVisible({ timeout: 5000 });
    // 宿主页 Georgia 字体不会进入 shadow 内的卡片；SDK 的圆形悬浮球样式生效。
    const styles = await page.evaluate(() => {
      const host = document.getElementById('fluentread-sdk-host')!;
      const shadow = host.shadowRoot!;
      const el = shadow.querySelector('.fr-ball') as HTMLElement;
      const cs = getComputedStyle(el);
      return { radius: cs.borderRadius, fontFamily: cs.fontFamily };
    });
    expect(styles.radius).toBe('50%');
    // 宿主 Georgia 排在栈里，但 SDK CSS 里 * 规则覆盖为 PingFang 栈。
    expect(styles.fontFamily).not.toContain('Georgia');
  });

  test('FR-906 白名单外来源 403（可读提示）', async ({ request }) => {
    const res = await request.post('/api/sdk/translate', {
      data: { segments: [{ text: 'hello' }], from: 'en', to: 'zh-Hans' },
      headers: { origin: 'https://evil.example.com' },
    });
    expect(res.status()).toBe(403);
    const body = await res.json();
    expect(body.error.code).toBe('ORIGIN_FORBIDDEN');
    expect(body.error.message).toContain('未开通翻译服务');
  });

  test('FR-906 白名单为空 = SDK 关闭', async ({ request }) => {
    // SDK 关闭的实例由 verify 脚本以 SDK_OFF_URL 提供（白名单为空）。
    const base = process.env.SDK_OFF_URL ?? '';
    expect(base).toBeTruthy();
    const res = await request.get(`${base}/api/sdk/word?word=recall`);
    expect(res.status()).toBe(403);
    const body = await res.json();
    expect(body.error.code).toBe('ORIGIN_FORBIDDEN');
  });
});

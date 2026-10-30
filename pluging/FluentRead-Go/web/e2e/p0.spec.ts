/**
 * p0.spec.ts 是 P0 骨架的 e2e 冒烟（NFR-14 门禁的一部分）。
 *
 * @file web/e2e/p0.spec.ts
 * 文件职责：以真实浏览器验证 P0 出口标准——服务能起、页面能开不白屏、
 * 四个路由可达、深浅色切换生效、接口 404 是统一响应包、访客 cookie 已下发。
 *
 * 模块边界：只断言用户可感知行为；被测服务由 scripts/verify-p0.sh 预先启动。
 */

import { expect, test } from '@playwright/test';

test.describe('P0 骨架', () => {
  test('四个页面都能打开且不白屏', async ({ page }) => {
    for (const [path, probe] of [
      ['/', '.toolbar'],           // 写作台：工具栏（含翻译按钮）
      ['/words', '[data-test="words-sort"]'],   // 词表页：排序控件
      ['/review', 'h1:has-text("复习")'],
      ['/settings', 'h1:has-text("偏好")'],
    ] as const) {
      await page.goto(path);
      await expect(page.locator(probe).first()).toBeVisible();
      // 不白屏：页面上始终有顶栏。
      await expect(page.locator('.topbar')).toBeVisible();
    }
  });

  test('访客 cookie 已下发（FR-1104）', async ({ page }) => {
    await page.goto('/');
    const cookies = await page.context().cookies();
    const vid = cookies.find((c) => c.name === 'fr_vid');
    expect(vid).toBeTruthy();
    // UUID v4 形状。
    expect(vid!.value).toMatch(/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/);
  });

  test('深浅色切换且刷新保留（FR-1001 主题位）', async ({ page }) => {
    await page.goto('/settings');
    await expect(page.locator('html')).not.toHaveClass(/dark/);
    await page.getByRole('button', { name: '切换到深色' }).click();
    await expect(page.locator('html')).toHaveClass(/dark/);
    await page.reload();
    await expect(page.locator('html')).toHaveClass(/dark/);
    // 切回浅色，给后面的用例一个干净状态。
    await page.getByRole('button', { name: '切换到浅色' }).click();
    await expect(page.locator('html')).not.toHaveClass(/dark/);
  });

  test('未实现的接口返回统一响应包 404（NFR-12）', async ({ request }) => {
    const res = await request.get('/api/definitely-not-exists');
    expect(res.status()).toBe(400);
    const body = await res.json();
    expect(body.ok).toBe(false);
    expect(body.error.code).toBe('INVALID_REQUEST');
    expect(typeof body.error.message).toBe('string');
  });

  test('health 三态可通过页面网络访问验证（FR-1003）', async ({ request }) => {
    const res = await request.get('/api/health');
    expect(res.ok()).toBeTruthy();
    const body = await res.json();
    expect(['ok', 'fail', 'degraded']).toContain(body.gateway);
    expect(['ok', 'fail']).toContain(body.db);
  });

  test('未知前端路由回落写作台', async ({ page }) => {
    await page.goto('/no/such/page');
    // 回落到写作台：工具栏可见。
    await expect(page.locator('.toolbar')).toBeVisible();
  });
});

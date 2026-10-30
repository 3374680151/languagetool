/**
 * p3.5.spec.ts 是 P3.5 模型选择的 e2e（FR-401~406/409）。
 *
 * @file web/e2e/p3.5.spec.ts
 * 文件职责：顶栏下拉展示站点池（带标签）与自定义入口（FR-402/409）；
 * 自定义模型三要素 → 测试连接 → 翻译走自定义端点（FR-403）；
 * 自定义失败回落默认模型并提示（FR-405）；密钥不回显（FR-404 浏览器侧）。
 * CUSTOM_MOCK_URL 指向假上游（自定义端点的目标）。
 */

import { expect, test, type Page } from '@playwright/test';

const MOCK_URL = process.env.CUSTOM_MOCK_URL ?? '';

async function pasteDoc(page: Page, text: string) {
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

async function gotoFresh(page: Page, path = '/') {
  await page.goto(path);
  await page.evaluate(() => localStorage.removeItem('fr.doc.v1'));
  await page.reload();
}

test.describe('P3.5 模型选择', () => {
  test('下拉含默认模型、站点池模型与自定义入口（FR-401/402/409）', async ({ page }) => {
    await gotoFresh(page);
    await page.getByTestId('model-select').click();
    const options = await page.locator('.el-select-dropdown__item').allTextContents();
    expect(options.some((o) => o.includes('默认'))).toBeTruthy();
    expect(options.some((o) => o.includes('DeepSeek'))).toBeTruthy();
    expect(options.some((o) => o.includes('通义千问'))).toBeTruthy();
    expect(options.some((o) => o.includes('自定义模型'))).toBeTruthy();
    // FR-409：enabled:false 的占位模板不出现在下拉。
    expect(options.some((o) => o.includes('my-custom'))).toBeFalsy();
    await page.keyboard.press('Escape');
  });

  test('自定义模型：测试连接成功 → 翻译走自定义端点（FR-403）', async ({ page }) => {
    await gotoFresh(page, '/settings');
    await page.getByTestId('custom-base').fill(MOCK_URL);
    await page.getByTestId('custom-key').fill('visitor-own-key');
    await page.getByTestId('custom-model').fill('my-own-model');
    await page.getByTestId('custom-test').click();
    await expect(page.getByTestId('custom-result')).toContainText('连接成功', { timeout: 10000 });
    await page.getByTestId('custom-save').click();

    // 密钥不回显到页面文本（FR-404 浏览器侧检查）。
    const body = await page.locator('body').textContent();
    expect(body).not.toContain('visitor-own-key');

    // 用顶栏链接切回写作台（SPA 内导航，内存里的 apiKey 保留；整页刷新会按
    // FR-404 约定清空密钥）。
    await page.locator('.topbar__nav').getByText('写作台').click();
    await page.getByTestId('model-select').click();
    await page.getByRole('option', { name: '自定义模型' }).click();
    await pasteDoc(page, 'A paragraph for the custom model.');
    await page.keyboard.press('Control+Enter');
    const dst = page.locator('.split__cell--dst[data-status="done"] textarea');
    await expect(dst).toHaveCount(1, { timeout: 15000 });
    await expect(dst).toHaveValue(/\[my-own-model\]/);
  });

  test('自定义模型失败回落默认并提示（FR-405）', async ({ page }) => {
    await gotoFresh(page, '/settings');
    // 测试连接：指向死端口。
    await page.getByTestId('custom-base').fill('http://127.0.0.1:1');
    await page.getByTestId('custom-key').fill('k');
    await page.getByTestId('custom-model').fill('m');
    await page.getByTestId('custom-test').click();
    await expect(page.getByTestId('custom-result')).toContainText('连接失败', { timeout: 15000 });
    await page.getByTestId('custom-save').click();

    await page.locator('.topbar__nav').getByText('写作台').click();
    await page.getByTestId('model-select').click();
    await page.getByRole('option', { name: '自定义模型' }).click();
    // 注意：不能整页刷新——apiKey 只存内存（FR-404），刷新会清空，自定义就不会携带。
    await pasteDoc(page, 'Fallback paragraph.');
    await page.keyboard.press('Control+Enter');
    // 译文来自默认模型（mock 给默认线路输出），且有回落提示。
    const dst = page.locator('.split__cell--dst[data-status="done"] textarea');
    await expect(dst).toHaveCount(1, { timeout: 20000 });
    await expect(dst).toHaveValue(/\[deepseek-chat\]/);
    // 持久回落标记（比 toast 更稳定，也符合"提示要说清"的要求）。
    await expect(page.getByTestId('custom-fallback-tag')).toBeVisible({ timeout: 5000 });
  });
});

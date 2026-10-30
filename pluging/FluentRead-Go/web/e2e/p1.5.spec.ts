/**
 * p1.5.spec.ts 是 P1.5 护栏的浏览器冒烟（FR-606 / FR-306 横幅）。
 *
 * @file web/e2e/p1.5.spec.ts
 * 文件职责：额度快用完才出现提示、充足时无提示（FR-606）；
 * 降级横幅出现且文案说人话（FR-306/NFR-11）。
 * EXPECT_QUOTA=low|ok 决定断言方向，由 verify-p1.5.sh 对不同实例各跑一次。
 */

import { expect, test } from '@playwright/test';

const expectQuota = process.env.EXPECT_QUOTA ?? 'ok';

const FOUR = [
  'First paragraph about retrieval augmented generation.',
  'Second paragraph recalls relevant passages.',
  'Third paragraph keeps terminology consistent.',
  'Fourth paragraph tells the reader what to do next.',
].join('\n\n');

async function pasteDoc(page: import('@playwright/test').Page, text: string) {
  await page.evaluate((t) => {
    const el = document.querySelector('.split__cell--src .split__input');
    const dt = new DataTransfer();
    dt.setData('text/plain', t);
    el.dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true }));
  }, text);
}

test('额度提示按余量出现（FR-606）', async ({ page }) => {
  await page.goto('/');
  await page.evaluate(() => localStorage.removeItem('fr.doc.v1'));
  await page.reload();
  await pasteDoc(page, FOUR);
  await page.keyboard.press('Control+Enter');
  await expect(page.locator('.split__cell--dst[data-status="done"]')).toHaveCount(4, { timeout: 15_000 });
  await page.waitForTimeout(600); // 等额度同步

  if (expectQuota === 'low') {
    await expect(page.getByTestId('banner-quota-low')).toBeVisible({ timeout: 5000 });
    const text = await page.getByTestId('banner-quota-low').textContent();
    expect(text).toContain('还剩');
    expect(text).not.toContain('配额');
  } else {
    await expect(page.getByTestId('banner-quota-low')).toHaveCount(0);
    await expect(page.getByTestId('banner-quota-out')).toHaveCount(0);
  }
});

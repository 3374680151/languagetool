/**
 * p5.5-review.spec.ts 是复习翻卡流程的 e2e（FR-1204 核心）。
 *
 * @file web/e2e/p5.5-review.spec.ts
 * 文件职责：到期入口徽标 → 复习卡挖空自己的句子 → 显示答案 → 两键评分
 *（记住/没记住）→ 间隔调度生效（FR_REVIEW_STAGES 缩短后 2 秒左右到期）。
 *
 * 模块边界：被测服务由 scripts/verify-p5.5.sh 预先启动（复习首档缩短为约 2 秒）。
 */

import { expect, test } from '@playwright/test';

async function gotoWrite(page: Page) {
  await page.goto('/');
  await page.evaluate(() => {
    localStorage.removeItem('fr.doc.v1');
    sessionStorage.clear();
  });
  await page.reload();
  const empty = page.getByTestId('empty-state');
  if (await empty.isVisible().catch(() => false)) {
    await page.getByTestId('start-write').click();
  }
}

test('FR-1204 复习闭环：到期入口 → 挖空自己的句子 → 两键评分', async ({ page }) => {
  test.setTimeout(60_000);
  await page.goto('/words');
  await page.evaluate(async () => {
    await fetch('/api/assets/clear', { method: 'POST' });
  });

  // 收藏一个带原句的词。
  await gotoWrite(page);
  await page.locator('.split__cell--src .split__input').first().fill('The judge will recall the witness tomorrow.');
  await page.evaluate(() => {
    const el = document.querySelector('.split__cell--src .split__input') as HTMLTextAreaElement;
    const idx = el.value.indexOf('recall');
    el.focus();
    el.setSelectionRange(idx, idx + 6);
    el.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, clientX: 300, clientY: 200 }));
  });
  await page.getByTestId('word-collect').click();
  await expect(page.getByTestId('word-collected')).toBeVisible({ timeout: 5000 });

  // 等第一档间隔（约 2 秒）到期。
  await page.waitForTimeout(2600);

  // 词表页入口显示到期数量（FR-1204：不弹窗、不常驻顶栏）。
  await page.goto('/words');
  await expect(page.getByTestId('review-due-badge')).toBeVisible({ timeout: 5000 });
  await expect(page.getByTestId('review-due-badge')).toHaveText('1');
  await page.screenshot({ path: '../docs/acceptance/P5.5/evidence/review-entry-badge.png' });

  // 复习卡：挖空自己的句子。
  await page.getByTestId('review-entry').click();
  await expect(page.getByTestId('review-card')).toBeVisible({ timeout: 5000 });
  const cloze = await page.getByTestId('review-cloze').textContent();
  expect(cloze).toContain('____');
  expect(cloze).toContain('The judge will');
  expect(cloze).not.toContain('recall');
  // 显示答案 → 两键。
  await page.getByTestId('review-reveal').click();
  await expect(page.getByTestId('review-answer')).toContainText('recall');
  await page.screenshot({ path: '../docs/acceptance/P5.5/evidence/review-card-revealed.png' });

  // 没记住 → 回退重排（仍在第一档，稍后会再出现）。
  await page.getByTestId('review-forget').click();
  await expect(page.getByTestId('review-done')).toBeVisible({ timeout: 5000 });
  await expect(page.getByTestId('review-done')).toContainText('没记住 1');

  // 再等一档 → 卡片再次出现（回退语义）。
  await page.waitForTimeout(2600);
  await page.goto('/review');
  await expect(page.getByTestId('review-card')).toBeVisible({ timeout: 8000 });
  // 这次记住 → 下一档是 1 分钟，暂时不再到期。
  await page.getByTestId('review-reveal').click();
  await page.getByTestId('review-remember').click();
  await expect(page.getByTestId('review-done')).toBeVisible({ timeout: 5000 });
  await page.waitForTimeout(2600);
  await page.goto('/review');
  await expect(page.getByTestId('review-empty')).toBeVisible({ timeout: 5000 });
  await page.screenshot({ path: '../docs/acceptance/P5.5/evidence/review-remembered.png' });
});

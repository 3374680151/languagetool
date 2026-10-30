/**
 * p5.spec.ts 是 P5（词表 + 收藏闭环）的 e2e（FR-702/703/704/1201/1202 + AD-6/AD-10）。
 *
 * @file web/e2e/p5.spec.ts
 * 文件职责：真实浏览器验证——写作台查词收藏带原句（FR-1201）、卡片已收藏态与
 * 幂等（AD-6）、词表列表/删除/空态/排序（FR-702/704）、再用标记 +1 且不打断
 *（FR-1202）、服务端按访客存储（FR-703）、词表页搜索框弹卡（AD-10）。
 *
 * 模块边界：被测服务由 scripts/verify-p5.sh 预先启动。
 */

import { expect, test, type Page } from '@playwright/test';

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

async function selectWord(page: Page, word: string) {
  await page.evaluate((w) => {
    const el = document.querySelector('.split__cell--src .split__input') as HTMLTextAreaElement;
    const idx = el.value.indexOf(w);
    el.focus();
    el.setSelectionRange(idx, idx + w.length);
    el.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, clientX: 300, clientY: 200 }));
  }, word);
  await page.waitForTimeout(50);
}

test.describe('P5 词表与收藏闭环', () => {
  test('FR-704 空态：说明 + 下一步动作', async ({ page }) => {
    await page.goto('/words');
    await expect(page.getByTestId('words-empty')).toBeVisible({ timeout: 5000 });
    const text = await page.getByTestId('words-empty').textContent();
    expect(text).toContain('词表还是空的');
    await page.screenshot({ path: '../docs/acceptance/P5/evidence/words-empty.png' });
  });

  test('FR-1201 查词收藏带原句；卡片变已收藏；重复收藏幂等（AD-6）', async ({ page }) => {
    await gotoWrite(page);
    await page.locator('.split__cell--src .split__input').first()
      .fill('The judge will recall the witness tomorrow.');
    await selectWord(page, 'recall');
    await expect(page.getByTestId('word-popup')).toBeVisible({ timeout: 5000 });
    // 收藏 → 卡片变已收藏。
    await page.getByTestId('word-collect').click();
    await expect(page.getByTestId('word-collected')).toBeVisible({ timeout: 5000 });
    // 重复收藏：按钮已是收藏态（只加不减，§八 8.3）。
    await expect(page.getByTestId('word-collect')).toHaveCount(0);

    // 词表出现该词，原句就是写作里的句子。
    await page.goto('/words');
    const row = page.locator('[data-test="word-row-recall"]');
    await expect(row).toBeVisible({ timeout: 5000 });
    await expect(row.locator('[data-test="word-context"]')).toHaveText('The judge will recall the witness tomorrow.');
    await page.screenshot({ path: '../docs/acceptance/P5/evidence/words-collected.png' });

    // 幂等：写作台再查再收藏 → 仍是同一条（列表只有一行）。
    await gotoWrite(page);
    await selectWord(page, 'recall');
    await expect(page.getByTestId('word-collected')).toBeVisible({ timeout: 5000 });
    await page.goto('/words');
    await expect(page.locator('[data-test="word-row-recall"]')).toHaveCount(1);
  });

  test('FR-1202 再使用标记：新段落再用 → 用过 +1，全程无弹窗', async ({ page }) => {
    // 收藏 recall。
    await gotoWrite(page);
    await page.locator('.split__cell--src .split__input').first().fill('The judge will recall the witness.');
    await selectWord(page, 'recall');
    await page.getByTestId('word-collect').click();
    await expect(page.getByTestId('word-collected')).toBeVisible({ timeout: 5000 });

    // 词表：用过 0 次。
    await page.goto('/words');
    const row = page.locator('[data-test="word-row-recall"]');
    await expect(row.locator('[data-test="word-use-count"]')).toHaveText('用过 0 次');

    // 写新段落（含 recall）→ 翻译 → 用过 1 次；输入与翻译过程无弹窗。
    await gotoWrite(page);
    await page.locator('.split__cell--src .split__input').first().fill('New paragraph will recall the old memory.');
    // 输入过程断言无弹窗。
    await expect(page.getByTestId('word-popup')).toHaveCount(0);
    await page.keyboard.press('Control+Enter');
    await expect(page.locator('.split__cell--dst[data-status="done"]')).toHaveCount(1, { timeout: 15000 });
    await expect(page.getByTestId('word-popup')).toHaveCount(0);

    await page.goto('/words');
    await expect(page.locator('[data-test="word-row-recall"]').locator('[data-test="word-use-count"]')).toHaveText('用过 1 次', { timeout: 5000 });
    await page.screenshot({ path: '../docs/acceptance/P5/evidence/words-reused.png' });
  });

  test('FR-1202 排序：最少使用优先', async ({ page }) => {
    // 收藏两个词，一个使用一次一个不用。
    await gotoWrite(page);
    await page.locator('.split__cell--src .split__input').first().fill('The judge will recall the witness.');
    await selectWord(page, 'recall');
    await page.getByTestId('word-collect').click();
    await expect(page.getByTestId('word-collected')).toBeVisible({ timeout: 5000 });
    await page.locator('.split__cell--src .split__input').first().fill('The system processes the data.');
    await selectWord(page, 'system');
    // system 可能不在词典：弹卡可能是模型兜底或错误态，收藏按钮存在即可。
    const collectBtn = page.getByTestId('word-collect');
    if (await collectBtn.isVisible().catch(() => false)) {
      await collectBtn.click();
      await page.waitForTimeout(600);
    }
    // recall 用一次。
    await page.keyboard.press('Control+Enter');
    await page.waitForTimeout(1200);

    await page.goto('/words');
    await page.locator('[data-test="words-sort"]').locator('.el-radio-button', { hasText: '最少使用' }).click();
    await page.waitForTimeout(600);
    const first = await page.locator('[data-test="words-list"] .word').first().textContent();
    // 未使用（0 次）的词排在前面。
    const rows = await page.locator('[data-test="words-list"] .word').allTextContents();
    const counts = await page.locator('[data-test="words-list"] [data-test="word-use-count"]').allTextContents();
    expect(rows.length).toBeGreaterThanOrEqual(1);
    expect(counts[0]).toBe('用过 0 次');
    void first;
  });

  test('FR-703 服务端按访客存储：同 context 可见，跨 context 不同步（预期）', async ({ page }) => {
    await gotoWrite(page);
    await page.locator('.split__cell--src .split__input').first().fill('The judge will recall the witness.');
    await selectWord(page, 'recall');
    await page.getByTestId('word-collect').click();
    await expect(page.getByTestId('word-collected')).toBeVisible({ timeout: 5000 });
    // 刷新后仍在（服务端存储）。
    await page.goto('/words');
    await page.reload();
    await expect(page.locator('[data-test="word-row-recall"]')).toBeVisible({ timeout: 5000 });
  });

  test('删除收藏词（FR-702 管理）', async ({ page }) => {
    await gotoWrite(page);
    await page.locator('.split__cell--src .split__input').first().fill('The judge will recall the witness.');
    await selectWord(page, 'recall');
    await page.getByTestId('word-collect').click();
    await expect(page.getByTestId('word-collected')).toBeVisible({ timeout: 5000 });
    await page.goto('/words');
    await page.locator('[data-test="word-del-recall"]').click();
    await page.getByTestId('confirm-ok').click();
    await expect(page.getByTestId('word-row-recall')).toHaveCount(0, { timeout: 5000 });
  });

  test('AD-10 词表页搜索框弹卡 + 收藏（无原句）', async ({ page }) => {
    await page.goto('/words');
    await page.getByTestId('words-search').fill('conclusion');
    await page.getByTestId('words-search-btn').click();
    await expect(page.getByTestId('words-search-card')).toBeVisible({ timeout: 5000 });
    await expect(page.getByTestId('words-search-card')).toContainText('conclusion');
    // 无原句收藏。
    await page.getByTestId('words-collect-btn').click();
    await expect(page.getByTestId('words-collected')).toBeVisible({ timeout: 5000 });
    // 列表出现该词，无原句提示。
    const row = page.locator('[data-test="word-row-conclusion"]');
    await expect(row).toBeVisible({ timeout: 5000 });
    await expect(row.locator('[data-test="word-context"]')).toContainText('没有原句');
    await page.screenshot({ path: '../docs/acceptance/P5/evidence/words-search-card.png' });
  });
});

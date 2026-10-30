/**
 * p3.spec.ts 是 P3 纠错的 e2e（出口标准 + FR-201~209）。
 *
 * @file web/e2e/p3.spec.ts
 * 文件职责：真实浏览器验证——3 处错误出 3 条建议（四字段）、逐条接受后原文正确、
 * 接受后剩余建议重定位、忽略移除、无错段落明确反馈、中文段无入口、
 * 输入过程不弹建议、界面无"一键改写全文"。
 *
 * 模块边界：被测服务由 scripts/verify-p3.sh 预先启动（mockupstream 提供确定性建议）。
 */

import { expect, test, type Page } from '@playwright/test';

// 覆盖 mock 的三个纠错标记：recieves（拼写）/ a RAG（语法）/ make a conclusion（用词）。
const BUGGY = 'The system recieves data from a RAG pipeline to make a conclusion.';
const CLEAN = 'The system receives data from the pipeline and draws the right conclusion.';

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

async function openProofread(page: Page, nth = 0) {
  await page.getByTestId(`proofread-btn-${nth}`).click();
  await page.getByTestId('proofread-panel').waitFor({ timeout: 5000 });
}

test.describe('P3 纠错', () => {
  test.beforeEach(async ({ page }) => {
    await page.goto('/');
    await page.evaluate(() => localStorage.removeItem('fr.doc.v1'));
    await page.reload();
  });

  test('3 处错误出 3 条建议，四字段齐全（FR-201/202/203）', async ({ page }) => {
    await pasteDoc(page, BUGGY);
    await openProofread(page);
    await expect(page.locator('.proof-item')).toHaveCount(3, { timeout: 15000 });
    for (let i = 0; i < 3; i++) {
      const item = page.locator(`[data-test="suggestion-${i}"]`);
      await expect(item.locator('.diff-original')).not.toBeEmpty();
      await expect(item.locator('.diff-suggestion')).not.toBeEmpty();
      await expect(item.locator('.el-tag')).not.toBeEmpty();
      await expect(item.locator('.proof-item__reason')).not.toBeEmpty();
    }
    // 类别覆盖：拼写 / 语法 / 用词。
    const tags = await page.locator('.proof-item .el-tag').allTextContents();
    expect(tags).toContain('拼写');
    expect(tags).toContain('语法');
    expect(tags).toContain('用词');
    await page.screenshot({ path: '../docs/acceptance/P3/evidence/proofread-3-suggestions.png' });
  });

  test('逐条接受：原文被改对，剩余建议重定位（FR-205，出口标准）', async ({ page }) => {
    await pasteDoc(page, BUGGY);
    await openProofread(page);
    await expect(page.locator('.proof-item')).toHaveCount(3, { timeout: 15000 });

    // 接受第 1 条 → 原文 recieves → receives。
    await page.locator('[data-test="suggestion-0"] button', { hasText: '接受' }).click();
    await expect(page.locator('.split__cell--src .split__input').first()).toHaveValue(/receives data from a RAG/);
    // 第 2/3 条仍能正确定位（重定位后仍是 2 条）。
    await expect(page.locator('.proof-item')).toHaveCount(2);

    await page.locator('[data-test="suggestion-0"] button', { hasText: '接受' }).click();
    await expect(page.locator('.split__cell--src .split__input').first()).toHaveValue(/from an RAG/);

    await page.locator('[data-test="suggestion-0"] button', { hasText: '接受' }).click();
    const final = await page.locator('.split__cell--src .split__input').first().inputValue();
    expect(final).toBe('The system receives data from an RAG pipeline to draw a conclusion.');
    // 面板里没有遗留建议。
    await expect(page.locator('.proof-item')).toHaveCount(0);
    await page.screenshot({ path: '../docs/acceptance/P3/evidence/proofread-all-accepted.png' });
  });

  test('忽略：建议移除，原文不动（FR-205）', async ({ page }) => {
    await pasteDoc(page, BUGGY);
    await openProofread(page);
    await expect(page.locator('.proof-item')).toHaveCount(3, { timeout: 15000 });
    await page.locator('[data-test="suggestion-0"] button', { hasText: '忽略' }).click();
    await expect(page.locator('.proof-item')).toHaveCount(2);
    await expect(page.locator('.split__cell--src .split__input').first()).toHaveValue(BUGGY);
  });

  test('没有错误的段落明确反馈（FR-209）', async ({ page }) => {
    await pasteDoc(page, CLEAN);
    await openProofread(page);
    await expect(page.getByTestId('proofread-empty')).toBeVisible({ timeout: 15000 });
    const text = await page.getByTestId('proofread-empty').textContent();
    expect(text).toContain('没有发现问题');
    await page.screenshot({ path: '../docs/acceptance/P3/evidence/proofread-clean.png' });
  });

  test('中文段没有纠错入口（FR-206）', async ({ page }) => {
    await pasteDoc(page, '这是一段纯中文的段落，不应该出现纠错按钮。');
    await expect(page.locator('.split__src-meta')).not.toContainText('纠错');
  });

  test('输入过程不弹建议、界面无整段重写（FR-207/208）', async ({ page }) => {
    await pasteDoc(page, CLEAN);
    // 逐字输入。
    await page.locator('.split__cell--src .split__input').first().click();
    await page.keyboard.press('End');
    await page.keyboard.type(' More words here.', { delay: 20 });
    await expect(page.getByTestId('proofread-panel')).not.toBeVisible();
    // 界面不出现"一键改写全文"。
    const body = await page.locator('.write').textContent();
    expect(body).not.toContain('一键改写');
    expect(body).not.toContain('改写全文');
  });

  test('纠错不自动触发：打开面板前无网络调用之外的弹层', async ({ page }) => {
    await pasteDoc(page, BUGGY);
    await page.waitForTimeout(1000);
    await expect(page.getByTestId('proofread-panel')).not.toBeVisible();
  });
});

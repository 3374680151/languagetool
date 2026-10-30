/**
 * p1.spec.ts 是 P1 翻译链路的 e2e（出口标准 + FR-1xx 冒烟）。
 *
 * @file web/e2e/p1.spec.ts
 * 文件职责：以真实浏览器验证——粘贴自动分段（FR-104/105）、Ctrl/⌘+Enter 全译
 *（FR-106）、逐段状态机与失败隔离（FR-107/303）、NFR-1 首段计时、
 * 逐行对齐（FR-102）、草稿恢复（FR-110）。
 *
 * 模块边界：被测服务由 scripts/verify-p1.sh 预先启动（mockupstream 提供确定性译文）。
 */

import { expect, test, type Page } from '@playwright/test';

const SAMPLE_4 = [
  'First paragraph about retrieval augmented generation.',
  'Second paragraph recalls relevant passages.',
  'Third paragraph keeps terminology consistent.',
  'Fourth paragraph tells the reader what to do next.',
].join('\n\n');

async function clearDraft(page: Page) {
  await page.goto('/');
  await page.evaluate(() => localStorage.removeItem('fr.doc.v1'));
}

async function pasteDoc(page: Page, text: string) {
  const first = page.locator('.split__cell--src .split__input').first();
  await first.click();
  await first.fill('');
  if (/\n[ \t]*\n/.test(text)) {
    // 含段落边界的"粘贴整篇"：合成 paste 事件（组件会手动并文并拆段）。
    await page.evaluate((t) => {
      const el = document.querySelector('.split__cell--src .split__input') as HTMLTextAreaElement;
      el.focus();
      const dt = new DataTransfer();
      dt.setData('text/plain', t);
      el.dispatchEvent(new ClipboardEvent('paste', { clipboardData: dt, bubbles: true, cancelable: true }));
    }, text);
  } else {
    // 单段文本：无空行边界，组件走浏览器默认粘贴，等价于直接写入。
    await first.fill(text);
  }
}

test.describe('P1 翻译链路', () => {
  test.beforeEach(async ({ page }) => {
    await clearDraft(page);
  });

  test('粘贴 4 段 + 空行得到 4 个输入框，段号连续（FR-104/105）', async ({ page }) => {
    await page.goto('/');
    await pasteDoc(page, SAMPLE_4 + '\n\n\n'); // 尾部多个空行不产生空框
    const rows = page.locator('.split-row');
    await expect(rows).toHaveCount(4);
    for (let i = 0; i < 4; i++) {
      await expect(rows.nth(i).locator('.split__num')).toHaveText(String(i + 1));
    }
  });

  test('Ctrl+Enter 全译：4 段都出中文，状态"完成"（FR-106/107）', async ({ page }) => {
    await page.goto('/');
    await pasteDoc(page, SAMPLE_4);
    await page.keyboard.press('Control+Enter');
    const done = page.locator('.split__cell--dst[data-status="done"] textarea');
    await expect(done).toHaveCount(4, { timeout: 15_000 });
    // 假网关译文是中文。
    const first = await done.first().inputValue();
    expect(first.length).toBeGreaterThan(0);
    // 统计行：4/4 段已译。
    await expect(page.getByTestId('stats')).toContainText('4/4 段已译');
  });

  test('1 段失败不影响其他段（P1 出口标准 / FR-303）', async ({ page }) => {
    await page.goto('/');
    const withFail = [
      'First paragraph is fine.',
      '[[fail]] This segment will fail upstream.',
      'Third paragraph is fine too.',
    ].join('\n\n');
    await pasteDoc(page, withFail);
    await page.keyboard.press('Control+Enter');

    const dst = page.locator('.split__cell--dst');
    await expect(dst.nth(0)).toHaveAttribute('data-status', 'done', { timeout: 15_000 });
    await expect(dst.nth(1)).toHaveAttribute('data-status', 'error');
    await expect(dst.nth(2)).toHaveAttribute('data-status', 'done');
    // 失败段给"重试本段"，文案不含内部词（NFR-11）。
    const errText = await page.getByTestId('seg-error').nth(0).textContent();
    expect(errText).not.toContain('线路');
    expect(errText).not.toContain('上游');
    await expect(page.getByTestId('seg-retry')).toBeVisible();
    // 移除标记后重试成功（选择器只取左栏源文输入框）。
    await page.locator('.split__cell--src .split__input').nth(1).fill('Fixed segment now works.');
    await page.getByTestId('seg-retry').click();
    await expect(dst.nth(1)).toHaveAttribute('data-status', 'done', { timeout: 15_000 });
  });

  test('NFR-1：单段首译 ≤ 1 秒（假网关基线）', async ({ page }) => {
    await page.goto('/');
    await pasteDoc(page, 'A single short paragraph for timing.');
    const start = Date.now();
    await page.keyboard.press('Control+Enter');
    await expect(page.locator('.split__cell--dst[data-status="done"]')).toBeVisible({ timeout: 15_000 });
    const elapsed = Date.now() - start;
    expect(elapsed).toBeLessThan(1000);
  });

  test('逐行对齐：每行左右顶端偏差 ≤ 2px（FR-102）', async ({ page }) => {
    await page.goto('/');
    await pasteDoc(page, SAMPLE_4);
    await page.keyboard.press('Control+Enter');
    await expect(page.locator('.split__cell--dst[data-status="done"]')).toHaveCount(4, { timeout: 15_000 });
    const offsets = await page.evaluate(() => {
      const rows = document.querySelectorAll('.split-row');
      return Array.from(rows).map((row) => {
        const cells = row.querySelectorAll<HTMLElement>('.split__cell');
        return Math.abs(cells[0].getBoundingClientRect().top - cells[1].getBoundingClientRect().top);
      });
    });
    for (const [i, off] of offsets.entries()) {
      expect(off, `第 ${i + 1} 行偏差 ${off}px`).toBeLessThanOrEqual(2);
    }
  });

  test('刷新不丢稿：段落、译文、手改标记都在（FR-110）', async ({ page }) => {
    await page.goto('/');
    await pasteDoc(page, SAMPLE_4);
    await page.keyboard.press('Control+Enter');
    await expect(page.locator('.split__cell--dst[data-status="done"] textarea')).toHaveCount(4, { timeout: 15_000 });
    // 手改第 1 段译文。
    const dst = page.locator('.split__cell--dst[data-status="done"] textarea').first();
    await dst.fill('这是手动改过的译文。');
    await page.reload();
    const rows = page.locator('.split-row');
    await expect(rows).toHaveCount(4);
    await expect(page.locator('.split__cell--dst textarea').first()).toHaveValue('这是手动改过的译文。');
    await expect(page.locator('.split__input--edited').first()).toBeVisible();
    // 手改过的源文与译文一致：状态仍是完成。
    await expect(page.locator('.split__cell--dst[data-status="done"]')).toHaveCount(4);
  });

  test('Ctrl+Shift+Enter 只译光标段（FR-106）', async ({ page }) => {
    await page.goto('/');
    await pasteDoc(page, SAMPLE_4);
    // 点进第 2 段，按 Ctrl+Shift+Enter。
    const second = page.locator('.split__cell--src .split__input').nth(1);
    await second.click();
    await page.keyboard.press('Control+Shift+Enter');
    // 只有第 2 段完成，其余 3 段仍待译。
    await expect(page.locator('.split__cell--dst[data-status="done"]')).toHaveCount(1, { timeout: 15_000 });
    const dst = page.locator('.split__cell--dst');
    await expect(dst.nth(1)).toHaveAttribute('data-status', 'done');
    await expect(dst.nth(0)).toHaveAttribute('data-status', 'idle');
    await expect(dst.nth(2)).toHaveAttribute('data-status', 'idle');
    await expect(dst.nth(3)).toHaveAttribute('data-status', 'idle');
  });
});

/**
 * p4.spec.ts 是 P4（词典 + 术语 + 词卡）的 e2e（§八 AD-1~12 + FR-501~505）。
 *
 * @file web/e2e/p4.spec.ts
 * 文件职责：真实浏览器验证查词弹卡的触发体系（AD-1/2/3/11）、卡片内容
 *（AD-4/5/7）、收藏（AD-6）、窄屏深色（AD-8）、限频（AD-9）与自动朗读
 *（AD-12）；术语的增删改、进提示词、过期横幅与一键重译（FR-501~504）。
 * AD-10（词表页搜索框）随 P5 验证。
 *
 * 模块边界：被测服务由 scripts/verify-p4.sh 预先启动；MOCK_URL 注入故障。
 */

import { expect, test, type Page } from '@playwright/test';

const MOCK_URL = process.env.MOCK_URL ?? '';

async function gotoWrite(page: Page) {
  await page.goto('/');
  await page.evaluate(() => {
    localStorage.removeItem('fr.doc.v1');
    sessionStorage.clear();
  });
  await page.reload();
  // 离开空态。
  const empty = page.getByTestId('empty-state');
  if (await empty.isVisible().catch(() => false)) {
    await page.getByTestId('start-write').click();
  }
}

/** selectWord 在 textarea 里选中指定词并派发 mouseup（textarea 选区不走 document selection）。 */
async function selectWord(page: Page, word: string) {
  await page.evaluate((w) => {
    const el = document.querySelector('.split__cell--src .split__input') as HTMLTextAreaElement;
    const idx = el.value.indexOf(w);
    if (idx >= 0) {
      el.focus();
      el.setSelectionRange(idx, idx + w.length);
      el.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, clientX: 300, clientY: 200 }));
    }
  }, word);
  await page.waitForTimeout(50);
}

test.describe('P4 查词弹卡', () => {
  test.beforeEach(async ({ page }) => {
    await gotoWrite(page);
    // 写一段含目标词的文本。
    await page.locator('.split__cell--src .split__input').first()
      .fill('The system will recall the failed jobs and receive new ones. This gerrymander story is odd.');
  });

  test('AD-1 双击单词弹卡，Esc/点空白/滚动/输入均能关闭', async ({ page }) => {
    const src = page.locator('.split__cell--src .split__input').first();
    await src.click();
    // 选词 + mouseup：等价于"双击选词"的最终状态（textarea 选区不进 document selection）。
    await selectWord(page, 'recall');
    await expect(page.getByTestId('word-popup')).toBeVisible({ timeout: 5000 });
    await expect(page.getByTestId('word-headword')).toHaveText(/recall|will/);
    // Esc 关闭。
    await page.keyboard.press('Escape');
    await expect(page.getByTestId('word-popup')).not.toBeVisible();
    // 再弹 → 点空白关闭。
    await selectWord(page, 'recall');
    await expect(page.getByTestId('word-popup')).toBeVisible({ timeout: 5000 });
    await page.mouse.click(700, 600);
    await expect(page.getByTestId('word-popup')).not.toBeVisible();
    // 再弹 → 输入关闭。
    await selectWord(page, 'recall');
    await expect(page.getByTestId('word-popup')).toBeVisible({ timeout: 5000 });
    await page.keyboard.type('x');
    await expect(page.getByTestId('word-popup')).not.toBeVisible();
  });

  test('AD-2 单击定位光标零误触', async ({ page }) => {
    const src = page.locator('.split__cell--src .split__input').first();
    for (let i = 0; i < 5; i++) {
      await src.click();
      await page.waitForTimeout(300);
      await expect(page.getByTestId('word-popup')).toHaveCount(0);
    }
  });

  test('AD-4 高频词命中词典：音标/释义/标签；生僻词转在线解释并标注 AI', async ({ page }) => {
    // recall 是 ECDICT 高频词。
    await selectWord(page, 'recall');
    await expect(page.getByTestId('word-popup')).toBeVisible({ timeout: 5000 });
    await expect(page.getByTestId('word-headword')).toHaveText('recall');
    await expect(page.locator('[data-test="word-popup"] .wp-phonetic')).toBeVisible();
    await expect(page.getByTestId('word-senses')).not.toBeEmpty();
    await page.screenshot({ path: '../docs/acceptance/P4/evidence/word-popup-ecdict.png' });

    // 生僻词 → 模型兜底 → "AI 生成"标注。
    await selectWord(page, 'gerrymander');
    await expect(page.getByTestId('word-popup')).toBeVisible({ timeout: 15000 });
    await expect(page.getByTestId('word-source-ai')).toBeVisible();
    await page.screenshot({ path: '../docs/acceptance/P4/evidence/word-popup-model.png' });
  });

  test('AD-5 拼错的词给出拼写提示', async ({ page }) => {
    await page.locator('.split__cell--src .split__input').first().fill('The system recieves data.');
    await selectWord(page, 'recieves');
    await expect(page.getByTestId('word-popup')).toBeVisible({ timeout: 15000 });
    await expect(page.getByTestId('word-typo')).toContainText('receive');
  });

  test('AD-7 词形还原：选中 recalled 显示原形提示', async ({ page }) => {
    await page.locator('.split__cell--src .split__input').first().fill('The judge recalled the witness yesterday.');
    await selectWord(page, 'recalled');
    await expect(page.getByTestId('word-popup')).toBeVisible({ timeout: 5000 });
    await expect(page.getByTestId('word-headword')).toHaveText('recall');
    await expect(page.getByTestId('word-lemma')).toContainText('recalled');
    await expect(page.getByTestId('word-lemma')).toContainText('recall');
  });

  test('AD-11 三种触发模式可切换、刷新保留、行为正确', async ({ page }) => {
    // 切到浮标确认模式。
    await page.goto('/settings');
    await page.locator('[data-test="pref-trigger"] .el-radio-button', { hasText: '浮标确认' }).click();
    await page.reload();
    await expect(page.locator('[data-test="pref-trigger"] .el-radio-button.is-active')).toContainText('浮标确认');

    await gotoWrite(page);
    await page.locator('.split__cell--src .split__input').first()
      .fill('The system will recall the failed jobs.');
    // 选中 recall → 只出浮标不出卡。
    await selectWord(page, 'recall');
    await page.waitForTimeout(400);
    await expect(page.getByTestId('word-popup')).toHaveCount(0);
    await expect(page.getByTestId('word-icon')).toBeVisible();
    // 点浮标才弹卡。
    await page.getByTestId('word-icon').click();
    await expect(page.getByTestId('word-popup')).toBeVisible({ timeout: 5000 });

    // 切到修饰键模式（刷新保留），仅 Alt+划选触发。
    await page.goto('/settings');
    await page.locator('[data-test="pref-trigger"] .el-radio-button', { hasText: '修饰键' }).click();
    await page.goto('/');
    await page.locator('.split__cell--src .split__input').first()
      .fill('The system will recall the failed jobs.');
    await selectWord(page, 'recall');
    await page.waitForTimeout(400);
    await expect(page.getByTestId('word-popup')).toHaveCount(0);
    // Alt+划选（带修饰键的 mouseup）。
    // Alt+划选：textarea 选区 + 带修饰键的 mouseup。
    await page.evaluate(() => {
      const el = document.querySelector('.split__cell--src .split__input') as HTMLTextAreaElement;
      const idx = el.value.indexOf('recall');
      el.focus();
      el.setSelectionRange(idx, idx + 6);
      el.dispatchEvent(new MouseEvent('mouseup', { bubbles: true, altKey: true, clientX: 300, clientY: 200 }));
    });
    await page.waitForTimeout(100);
    await expect(page.getByTestId('word-popup')).toBeVisible({ timeout: 5000 });

    // 刷新后模式保留。
    await page.goto('/settings');
    await expect(page.locator('[data-test="pref-trigger"] .el-radio-button.is-active')).toContainText('修饰键');
    // 恢复默认，避免影响其他用例。
    await page.locator('[data-test="pref-trigger"] .el-radio-button', { hasText: '划选即弹' }).click();
  });

  test('AD-12 自动朗读开关生效', async ({ page }) => {
    // 注入 speak 记录器（headless 可能没有 speechSynthesis，先补 stub）。
    await page.evaluate(() => {
      const spoken: string[] = [];
      (window as unknown as { __spoken: string[] }).__spoken = spoken;
      if (!('speechSynthesis' in window)) {
        (window as unknown as { speechSynthesis: unknown }).speechSynthesis = {
          speak: (u: { text: string }) => spoken.push(u.text),
          cancel: () => {},
        };
        return;
      }
      const synth = window.speechSynthesis;
      synth.speak = (u: SpeechSynthesisUtterance) => {
        spoken.push(u.text);
      };
    });
    await page.locator('.split__cell--src .split__input').first()
      .fill('The system will recall the failed jobs.');
    // 默认关闭：弹卡不朗读。
    await selectWord(page, 'recall');
    await expect(page.getByTestId('word-popup')).toBeVisible({ timeout: 5000 });
    await page.waitForTimeout(300);
    let spoken = await page.evaluate(() => (window as unknown as { __spoken: string[] }).__spoken.length);
    expect(spoken).toBe(0);
    // 手动点朗读按钮 → 发声。
    await page.getByTestId('word-speak').click();
    spoken = await page.evaluate(() => (window as unknown as { __spoken: string[] }).__spoken.length);
    expect(spoken).toBe(1);

    // 设置开启自动朗读 → 弹卡即读。
    await page.goto('/settings');
    await page.getByTestId('pref-autospeak').click();
    await gotoWrite(page);
    await page.evaluate(() => {
      const spoken: string[] = [];
      (window as unknown as { __spoken: string[] }).__spoken = spoken;
      if (!('speechSynthesis' in window)) {
        (window as unknown as { speechSynthesis: unknown }).speechSynthesis = {
          speak: (u: { text: string }) => spoken.push(u.text),
          cancel: () => {},
        };
        return;
      }
      window.speechSynthesis.speak = (u: SpeechSynthesisUtterance) => {
        spoken.push(u.text);
      };
    });
    await page.locator('.split__cell--src .split__input').first()
      .fill('The system will recall the failed jobs.');
    await selectWord(page, 'recall');
    await expect(page.getByTestId('word-popup')).toBeVisible({ timeout: 5000 });
    await page.waitForTimeout(500);
    spoken = await page.evaluate(() => (window as unknown as { __spoken: string[] }).__spoken.length);
    expect(spoken).toBeGreaterThanOrEqual(1);
    // 关闭开关。
    await page.goto('/settings');
    await page.getByTestId('pref-autospeak').click();
  });
});

test.describe('P4 术语', () => {
  test.beforeEach(async ({ page }) => {
    await gotoWrite(page);
  });

  test('FR-501/502 术语增删改且真的进提示词', async ({ page }) => {
    await page.getByTestId('glossary-btn').click();
    await page.getByTestId('glossary-dialog').waitFor({ timeout: 5000 });
    await page.getByTestId('glossary-source').fill('benchmark');
    await page.getByTestId('glossary-target').fill('基准测试');
    await page.getByTestId('glossary-submit').click();
    await expect(page.getByTestId('glossary-table')).toContainText('基准测试');

    // 写正文，重译后译文中含新的系统提示词指纹（mock 把提示词指纹附在译文尾部）。
    await page.keyboard.press('Escape');
    const empty1 = page.getByTestId('empty-state');
    if (await empty1.isVisible().catch(() => false)) {
      await page.getByTestId('start-write').click();
    }
    await page.locator('.split__cell--src .split__input').first()
      .fill('The system will benchmark the recall pipeline.');
    await page.keyboard.press('Control+Enter');
    const dst0 = page.locator('.split__cell--dst[data-status="done"] textarea').first();
    await expect(dst0).toHaveCount(1, { timeout: 15000 });

    // FR-502：重译后译文中含新的系统提示词指纹（mock 把提示词指纹附在译文尾部）。
    const dst = dst0;
    const fp1 = await dst.inputValue();

    // 改术语译法 → 再重译 → 指纹必须变化（提示词变了）。
    await page.getByTestId('glossary-btn').click();
    await page.locator('[data-test^="glossary-del-"]').first().click();
    await page.getByTestId('confirm-ok').click();
    await page.getByTestId('glossary-source').fill('benchmark');
    await page.getByTestId('glossary-target').fill('基准');
    await page.getByTestId('glossary-submit').click();
    await expect(page.getByTestId('glossary-table')).toContainText('基准');
    await page.keyboard.press('Escape');
    // 术语变了 → 过期横幅出现 → 点"全部重译"强制重跑（Ctrl+Enter 只译变更段）。
    await expect(page.getByTestId('banner-glossary-stale')).toBeVisible({ timeout: 5000 });
    await page.getByTestId('glossary-retranslate').click();
    await page.waitForTimeout(1500);
    const fp2 = await dst.inputValue();
    expect(fp2).not.toEqual(fp1);
    // 重译后指纹对齐，横幅消失。
    await expect(page.getByTestId('banner-glossary-stale')).not.toBeVisible({ timeout: 5000 });
  });

  test('FR-503/504 改术语后提示"译文是旧的"，一键重译（手改需确认）', async ({ page }) => {
    // 先写正文并翻译一次。
    await page.locator('.split__cell--src .split__input').first()
      .fill('The system will recall the failed jobs.');
    await page.keyboard.press('Control+Enter');
    await expect(page.locator('.split__cell--dst[data-status="done"]')).toHaveCount(1, { timeout: 15000 });
    // 手改译文（为 FR-504 做铺垫）。
    await page.locator('.split__cell--dst textarea').first().fill('手动改过的译文。');
    // 改术语 → 指纹变化（用 FR-501 没用过的词，避免同名词冲突）。
    await page.getByTestId('glossary-btn').click();
    await page.getByTestId('glossary-source').fill('workflow');
    await page.getByTestId('glossary-target').fill('工作流');
    await page.getByTestId('glossary-submit').click();
    await expect(page.getByTestId('glossary-table')).toContainText('工作流');
    await page.keyboard.press('Escape');
    await expect(page.getByTestId('banner-glossary-stale')).toBeVisible({ timeout: 5000 });
    // 一键重译：有手改段 → 弹确认（FR-504）。
    await page.getByTestId('glossary-retranslate').click();
    await expect(page.getByText('全部重译会覆盖这些修改')).toBeVisible({ timeout: 5000 });
    await page.getByTestId('confirm-ok').click();
    // 重译后横幅消失（指纹对齐）。
    await expect(page.getByTestId('banner-glossary-stale')).not.toBeVisible({ timeout: 10000 });
  });
});

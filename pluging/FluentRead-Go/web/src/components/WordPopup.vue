<!--
  WordPopup.vue 是全局单例查词弹卡（FR-701 + §八《查词弹卡详细设计》全量）。
  @file web/src/components/WordPopup.vue
  文件职责：
  - 触发（8.2）：三种模式可配——划选即弹（direct，默认，200ms 选区落定延迟）/
    浮标确认（icon）/ 修饰键（modifier，Alt+划选）；单击永不弹（AD-2）；
    划选 ≥5 词不弹；中文选区不弹（8.1 边界）。
  - 卡片（8.3）：词典态（音标/词性/多条释义/考纲标签）与在线态（"AI 生成，仅供参考"）；
    词形还原提示（AD-7）；拼写错误提示（AD-5）；已收藏态 + 使用次数（AD-6）。
  - 交互（8.4）：锚定选区、空间翻转、视口夹紧；Esc/滚动/开始输入/点空白关闭（AD-1）；
    自动朗读开关（AD-12，speechSynthesis en-US）；300ms 内同词复用卡片（8.2 预处理）。
  模块边界：查词走 api/client.getDict；收藏事件交给父级（P5 接词表）；
  触发模式与自动朗读读 stores/settings。
-->
<script setup lang="ts">
import { computed, onBeforeUnmount, onMounted, ref } from 'vue';
import { getDict } from '../api/client';
import type { DictEntry } from '../api/types';
import { useSettingsStore } from '../stores/settings';

const settings = useSettingsStore();

interface PopupState {
  visible: boolean;
  x: number;
  y: number;
  flip: boolean;
  query: string;
  loading: boolean;
  error: string;
  entry: DictEntry | null;
}

const state = ref<PopupState>({
  visible: false, x: 0, y: 0, flip: false,
  query: '', loading: false, error: '', entry: null,
});

// 浮标确认模式的小按钮位置。
const iconPos = ref({ x: 0, y: 0, visible: false, query: '' });

let hideTimer: ReturnType<typeof setTimeout> | null = null;
let lastLookup = { word: '', at: 0 };
let lastRect: DOMRect | null = null;
/** lastContext 在弹卡瞬间（textarea 还持有焦点与选区）捕获的所在句（8.5）。 */
let lastContext = '';
let seq = 0;

/** wordCount 选区词数。 */
function wordCount(sel: string): number {
  return sel.trim().split(/\s+/).filter(Boolean).length;
}

/** isEnglishSelection 选区是否英文（含连字符词）；纯数字/符号不弹（8.2 预处理）。 */
function isEnglishSelection(sel: string): boolean {
  const t = sel.trim();
  if (!t || !/^[a-zA-Z][a-zA-Z'\- ]*$/.test(t)) return false;
  return true;
}

/** cleanQuery 清洗选区：去首尾标点、小写（8.2 预处理 1）。 */
function cleanQuery(sel: string): string {
  return sel.trim().toLowerCase().replace(/^[^a-z0-9]+|[^a-z0-9]+$/gi, '');
}

/** lastMouse 记录最近一次鼠标位置：textarea 选区没有 Range 几何，用指针位置锚卡。 */
const lastMouse = { x: 0, y: 0 };

/** selectionRect 取当前选区的屏幕位置。
 * textarea 的选区不在 document selection API 里（Chromium 行为），
 * 单独读 activeElement 的 selectionStart/End，位置用指针坐标近似锚定。 */
function selectionRect(): DOMRect | null {
  const active = document.activeElement;
  if (active instanceof HTMLTextAreaElement && active.selectionStart !== active.selectionEnd) {
    return new DOMRect(lastMouse.x, lastMouse.y - 10, Math.max(40, 40), 20);
  }
  const sel = window.getSelection();
  if (!sel || sel.isCollapsed || sel.rangeCount === 0) return null;
  const rect = sel.getRangeAt(0).getBoundingClientRect();
  if (rect.width === 0 && rect.height === 0) return null;
  return rect;
}

/** selectionText 取当前选中文本（textarea 与普通文本统一）。 */
function selectionText(): string {
  const active = document.activeElement;
  if (active instanceof HTMLTextAreaElement && active.selectionStart !== active.selectionEnd) {
    return active.value.slice(active.selectionStart, active.selectionEnd);
  }
  return window.getSelection()?.toString() ?? '';
}

/** speak 朗读（8.4：speechSynthesis，en-US 优先；不支持时按钮置灰）。 */
function speak(word: string) {
  if (!('speechSynthesis' in window)) return;
  window.speechSynthesis.cancel();
  const u = new SpeechSynthesisUtterance(word);
  u.lang = 'en-US';
  window.speechSynthesis.speak(u);
}

const canSpeak = computed(() => typeof window !== 'undefined' && 'speechSynthesis' in window);

/** lookup 查词并弹卡；300ms 内同词复用（8.2 预处理 4）。 */
async function lookup(query: string, rect: DOMRect) {
  lastRect = rect;
  const now = Date.now();
  if (lastLookup.word === query && now - lastLookup.at < 300 && state.value.visible) {
    return;
  }
  lastLookup = { word: query, at: now };

  // 会话缓存：sessionStorage 命中直接展示（8.2 预处理 5）。
  const cached = sessionStorage.getItem(`fr.dict.${query}`);
  if (cached) {
    try {
      show(JSON.parse(cached) as DictEntry, rect);
      return;
    } catch {
      sessionStorage.removeItem(`fr.dict.${query}`);
    }
  }

  // 弹卡瞬间抓"所在句"：此刻 textarea 还持有焦点与选区；点击收藏时焦点已移走。
  lastContext = contextSentenceOf(query);
  const mySeq = ++seq;
  state.value = {
    visible: true,
    x: rect.left, y: rect.bottom, flip: false,
    query, loading: true, error: '', entry: null,
  };
  positionCard(rect);
  try {
    const entry = await getDict(query);
    if (mySeq !== seq) return; // 已被更新的查询取代
    sessionStorage.setItem(`fr.dict.${query}`, JSON.stringify(entry));
    show(entry, rect);
    // AD-12：自动朗读开启时弹卡即读。
    if (settings.prefs.autoSpeak) speak(entry.word);
  } catch (e) {
    if (mySeq !== seq) return;
    state.value.loading = false;
    state.value.error = e instanceof Error ? e.message : '没查到这个词，请稍后再试';
  }
}

/** show 渲染卡片并定位。 */
function show(entry: DictEntry, rect: DOMRect) {
  state.value.entry = entry;
  state.value.loading = false;
  state.value.error = '';
  state.value.visible = true;
  state.value.query = entry.query;
  positionCard(rect);
}

/** positionCard 锚定选区：优先下方 8px，空间不足翻转到上方；左右夹紧视口（8.4）。 */
function positionCard(rect: DOMRect) {
  requestAnimationFrame(() => {
    const el = cardRef.value;
    if (!el) return;
    const w = el.offsetWidth;
    const h = el.offsetHeight;
    const vw = window.innerWidth;
    const vh = window.innerHeight;
    let x = rect.left + rect.width / 2 - w / 2;
    x = Math.max(8, Math.min(x, vw - w - 8));
    let flip = false;
    let y = rect.bottom + 8;
    if (y + h > vh - 8) {
      y = rect.top - h - 8;
      flip = true;
    }
    if (y < 8) y = 8;
    state.value.x = x;
    state.value.y = y;
    state.value.flip = flip;
  });
}

const cardRef = ref<HTMLElement | null>(null);

/** retryLookup 失败重试（8.4：失败卡内给"没查到 + 重试"）。 */
function retryLookup() {
  const q = state.value.query;
  state.value.visible = false;
  void lookup(q, lastRect ?? new DOMRect(0, 0, 1, 1));
}

/** dismiss 关卡（滚动/开始输入/切换段落都会调用）。 */
function dismiss() {
  state.value.visible = false;
  state.value.entry = null;
  iconPos.value.visible = false;
  window.getSelection()?.removeAllRanges();
}

/** collect 点击收藏 → 按钮进入加载态 → 成功变已收藏态（8.4）。 */
const collectState = ref<'idle' | 'loading' | 'done' | 'error'>('idle');

async function collect() {
  const entry = state.value.entry;
  if (!entry || collectState.value === 'loading') return;
  collectState.value = 'loading';
  try {
    const { collectWord } = await import('../api/client');
    await collectWord({ word: entry.word, lang: 'en', contextSentence: lastContext });
    entry.collected = { yes: true, useCount: entry.collected.useCount };
    collectState.value = 'done';
    // 收藏成功后刷新卡内已收藏态。
    state.value.entry = { ...entry };
  } catch (e) {
    collectState.value = 'error';
    state.value.error = e instanceof Error ? e.message : '收藏失败，请稍后重试';
    setTimeout(() => { collectState.value = 'idle'; }, 1500);
  }
}

/** contextSentenceOf 从当前选区所在文本提取所在句（8.5 收藏上下文，前端尽力提取）。 */
function contextSentenceOf(word: string): string {
  // textarea 场景：直接从输入值里找词。
  const active = document.activeElement;
  if (active instanceof HTMLTextAreaElement) {
    return sentenceIn(active.value, word);
  }
  const sel = window.getSelection();
  if (!sel || sel.rangeCount === 0) return '';
  let node: Node | null = sel.getRangeAt(0).startContainer;
  while (node && node.nodeType !== Node.TEXT_NODE) node = node.firstChild;
  if (!node) return '';
  const text = node.textContent ?? '';
  const pos = text.toLowerCase().indexOf(word.toLowerCase());
  if (pos < 0) return text.slice(0, 300);
  const before = text.slice(0, pos + word.length);
  const after = text.slice(pos + word.length);
  let start = Math.max(
    before.lastIndexOf('.'), before.lastIndexOf('!'), before.lastIndexOf('?'),
  ) + 1;
  let end = after.search(/[.!?]/);
  const sentence = (before.slice(start) + (end >= 0 ? after.slice(0, end + 1) : after)).trim();
  return sentence.slice(0, 300);
}

/** sentenceIn 从文本里提取 word 所在句（8.5 提取规则）。 */
function sentenceIn(text: string, word: string): string {
  const pos = text.toLowerCase().indexOf(word.toLowerCase());
  if (pos < 0) return text.slice(0, 300);
  const before = text.slice(0, pos + word.length);
  const after = text.slice(pos + word.length);
  const start = Math.max(before.lastIndexOf('.'), before.lastIndexOf('!'), before.lastIndexOf('?')) + 1;
  const end = after.search(/[.!?]/);
  return (before.slice(start) + (end >= 0 ? after.slice(0, end + 1) : after)).trim().slice(0, 300);
}

// ---------------------------------------------------------------------------
// 事件绑定（触发体系 8.2）
// ---------------------------------------------------------------------------

/** onMouseUp 处理划选：按模式分流。 */
function onMouseUp(e: MouseEvent) {
  lastMouse.x = e.clientX;
  lastMouse.y = e.clientY;
  // 点击在卡内/浮标内不处理。
  const target = e.target as HTMLElement;
  if (cardRef.value?.contains(target)) return;
  if (iconPos.value.visible) {
    // 浮标点击由浮标自己的 handler 处理；点击空白收浮标。
    if (!iconEl.value?.contains(target)) iconPos.value.visible = false;
    return;
  }

  const sel = selectionText();
  if (!sel || !isEnglishSelection(sel)) return;
  const rect = selectionRect();
  if (!rect) return;
  const query = cleanQuery(sel);
  if (!query || wordCount(query) > 4 || wordCount(query) === 0) return;

  const mode = settings.prefs.wordTrigger;
  const withModifier = e.altKey || e.ctrlKey || e.metaKey;

  if (mode === 'modifier') {
    // 修饰键模式：仅按住 Alt（或 Ctrl/⌘）+ 划选才弹。
    if (withModifier) void lookup(query, rect);
    return;
  }
  if (mode === 'icon') {
    // 浮标确认模式：选中只出浮标，点浮标才查（AD-11）。
    iconPos.value = { x: rect.right + 6, y: rect.top - 4, visible: true, query };
    return;
  }
  // direct（划选即弹，默认）：200ms 选区落定延迟——替换式选中不闪卡（AD-11）。
  if (hideTimer) clearTimeout(hideTimer);
  hideTimer = setTimeout(() => void lookup(query, rect), 200);
}

/** onIconClick 浮标点击 → 查词。 */
function onIconClick(e: MouseEvent) {
  const q = iconPos.value.query;
  iconPos.value.visible = false;
  lastMouse.x = e.clientX;
  lastMouse.y = e.clientY;
  const rect = selectionRect() ?? new DOMRect(e.clientX - 20, e.clientY - 10, 40, 20);
  void lookup(q, rect);
}

/** onMouseDown 单击永不弹（AD-2）：单击只是关掉已有卡片/浮标。 */
function onMouseDown(e: MouseEvent) {
  lastMouse.x = e.clientX;
  lastMouse.y = e.clientY;
  const target = e.target as HTMLElement;
  if (cardRef.value?.contains(target) || iconEl.value?.contains(target)) return;
  if (state.value.visible) dismiss();
  iconPos.value.visible = false;
}

/** onKeydown Esc 关卡（AD-1）。 */
function onKeydown(e: KeyboardEvent) {
  if (e.key === 'Escape' && state.value.visible) dismiss();
}

/** onScroll 滚动关卡（AD-1）。 */
function onScroll() {
  if (state.value.visible) dismiss();
  iconPos.value.visible = false;
}

/** onKeydownTyping 开始输入关卡（AD-1）：由 SplitPane 的输入事件经 window keydown 捕获。 */
function onTyping(e: KeyboardEvent) {
  if (state.value.visible && e.key.length === 1 && !e.metaKey && !e.ctrlKey) {
    dismiss();
  }
}

onMounted(() => {
  window.addEventListener('mouseup', onMouseUp);
  window.addEventListener('mousedown', onMouseDown);
  window.addEventListener('keydown', onKeydown);
  window.addEventListener('keydown', onTyping);
  window.addEventListener('scroll', onScroll, true);
  window.addEventListener('resize', onScroll);
});
onBeforeUnmount(() => {
  window.removeEventListener('mouseup', onMouseUp);
  window.removeEventListener('mousedown', onMouseDown);
  window.removeEventListener('keydown', onKeydown);
  window.removeEventListener('keydown', onTyping);
  window.removeEventListener('scroll', onScroll, true);
  window.removeEventListener('resize', onScroll);
});

const iconEl = ref<HTMLElement | null>(null);

defineExpose({ dismiss });
</script>

<template>
  <!-- 浮标确认模式的小按钮（AD-11：icon 模式只出浮标） -->
  <button
    v-if="iconPos.visible"
    ref="iconEl"
    class="wp-icon"
    :style="{ left: `${iconPos.x}px`, top: `${iconPos.y}px` }"
    title="查词"
    data-test="word-icon"
    @mousedown.stop
    @click.stop="onIconClick"
  >译</button>

  <!-- 弹卡：Teleport 到 body，fixed 定位锚定选区 -->
  <Teleport to="body">
    <div
      v-if="state.visible"
      ref="cardRef"
      class="wp-card"
      :class="{ 'wp-card--flip': state.flip }"
      :style="{ left: `${state.x}px`, top: `${state.y}px` }"
      data-test="word-popup"
      role="dialog"
      aria-label="查词卡"
      @mousedown.stop
    >
      <!-- 加载态（六态之"加载"，8.4） -->
      <div v-if="state.loading" class="wp-loading" data-test="word-loading">
        <el-skeleton :rows="2" animated />
      </div>

      <!-- 失败态（六态之"失败"，8.4） -->
      <div v-else-if="state.error" class="wp-error" data-test="word-error">
        <p>{{ state.error }}</p>
        <el-button size="small" data-test="word-retry" @click="retryLookup">重试</el-button>
      </div>

      <template v-else-if="state.entry">
        <!-- 词头：词 / 音标 / 词性 -->
        <div class="wp-head">
          <span class="wp-word" data-test="word-headword">{{ state.entry.word }}</span>
          <span v-if="state.entry.phonetic" class="wp-phonetic">{{ state.entry.phonetic }}</span>
          <span v-if="state.entry.pos" class="wp-pos">{{ state.entry.pos }}</span>
        </div>

        <!-- 词形还原提示（AD-7） -->
        <p v-if="state.entry.word !== state.entry.query" class="wp-lemma faint" data-test="word-lemma">
          ⓘ 查询的是 {{ state.entry.query }} —— 原形 {{ state.entry.word }}
        </p>
        <!-- 拼写错误提示（AD-5） -->
        <p v-if="state.entry.typoHint" class="wp-lemma" data-test="word-typo">
          ⓘ 可能是 {{ state.entry.typoHint.correctTo }} 的拼写错误
        </p>

        <!-- 来源标注（8.3：在线态必须标注） -->
        <p v-if="state.entry.source === 'model'" class="wp-source" data-test="word-source-ai">
          ⓘ 在线解释（AI 生成，仅供参考）
        </p>

        <!-- 考纲标签（词典态） -->
        <div v-if="state.entry.tags?.length" class="wp-tags" data-test="word-tags">
          <el-tag v-for="t in state.entry.tags" :key="t" size="small" effect="plain" type="info">{{ t }}</el-tag>
        </div>

        <!-- 释义 -->
        <ul class="wp-senses" data-test="word-senses">
          <li v-for="(s, i) in state.entry.senses" :key="i">
            <span v-if="s.pos" class="wp-sense-pos">{{ s.pos }}</span>
            <span data-test="word-sense-def">{{ s.def }}</span>
          </li>
        </ul>
        <p v-if="!state.entry.senses.length" class="wp-senses faint">没有查到这个词的解释。</p>

        <!-- 例句（在线态/短语态） -->
        <p v-if="state.entry.example" class="wp-example" data-test="word-example">
          e.g. {{ state.entry.example }}
        </p>

        <div class="wp-divider" />

        <!-- 操作行：朗读 + 收藏（AD-6：只加不减） -->
        <div class="wp-actions">
          <button
            class="wp-btn"
            :disabled="!canSpeak"
            :title="canSpeak ? '朗读' : '当前环境不支持朗读'"
            data-test="word-speak"
            @click="speak(state.entry.word)"
          >🔊 朗读</button>
          <button
            v-if="!state.entry.collected.yes"
            class="wp-btn wp-btn--primary"
            :disabled="collectState === 'loading'"
            data-test="word-collect"
            @click="collect"
          >{{ collectState === 'loading' ? '收藏中…' : '☆ 收藏' }}</button>
          <span v-else class="wp-collected" data-test="word-collected">
            ✓ 已收藏<template v-if="state.entry.collected.useCount > 0"> · 写作中用过 {{ state.entry.collected.useCount }} 次</template>
          </span>
        </div>
        <p v-if="collectState === 'error' && state.error" class="wp-error-mini">{{ state.error }}</p>
      </template>
    </div>
  </Teleport>
</template>

<style scoped>
.wp-icon {
  position: fixed;
  z-index: 4000;
  width: 24px;
  height: 24px;
  border-radius: 50%;
  border: 1px solid var(--border-strong);
  background: var(--surface);
  color: var(--primary);
  font-size: 12px;
  cursor: pointer;
  box-shadow: var(--shadow);
  padding: 0;
  line-height: 1;
}

.wp-card {
  position: fixed;
  z-index: 4100;
  width: min(320px, calc(100vw - 24px));
  background: var(--surface);
  border: 1px solid var(--border);
  border-radius: var(--radius);
  box-shadow: var(--shadow);
  padding: 12px 14px;
  font-size: 13px;
  color: var(--ink);
}

.wp-word {
  font-size: 16px;
  font-weight: 700;
}

.wp-phonetic {
  color: var(--ink-2);
  margin-left: 8px;
  font-family: var(--mono);
  font-size: 12px;
}

.wp-pos {
  color: var(--ink-3);
  margin-left: 8px;
  font-size: 12px;
}

.wp-lemma {
  margin: 6px 0 0;
  font-size: 12px;
  color: var(--primary);
}

.wp-source {
  margin: 6px 0 0;
  font-size: 12px;
  color: var(--warn);
}

.wp-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 4px;
  margin-top: 8px;
}

.wp-senses {
  margin: 8px 0 0;
  padding-left: 18px;
  line-height: 1.7;
}

.wp-sense-pos {
  color: var(--ink-3);
  margin-right: 4px;
}

.wp-example {
  margin: 8px 0 0;
  color: var(--ink-2);
  font-style: italic;
  font-size: 12.5px;
}

.wp-divider {
  border-top: 1px solid var(--border);
  margin: 10px 0 8px;
}

.wp-actions {
  display: flex;
  align-items: center;
  gap: 10px;
}

.wp-btn {
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--ink);
  border-radius: var(--radius-sm);
  font-size: 12.5px;
  padding: 3px 10px;
  cursor: pointer;
}

.wp-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.wp-btn--primary {
  border-color: var(--primary);
  color: var(--primary);
}

.wp-collected {
  color: var(--ok);
  font-size: 12.5px;
}

.wp-loading,
.wp-error {
  padding: 4px 0;
}

.wp-error p {
  margin: 0 0 8px;
  color: var(--danger);
}

.wp-error-mini {
  margin: 6px 0 0;
  color: var(--danger);
  font-size: 12px;
}
</style>

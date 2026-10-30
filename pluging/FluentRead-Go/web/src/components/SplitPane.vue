<!--
  SplitPane.vue 是左英右中双栏对照区（FR-102/103/107/108）。
  @file web/src/components/SplitPane.vue
  文件职责：单一 CSS Grid——每个段落一行 grid row（display:contents 让行内两格
  落进第 1、2 列），行高由两格中较高者决定，逐行顶端偏差恒为 0（FR-102）；
  hover 某行左右两格同时高亮（FR-103）；左格 textarea 自动增高并检测空行拆段
  （FR-104/105）；右格译文直接可编辑（FR-108），未译/翻译中/失败分别呈现
  （FR-107 + 六态之加载/失败）；窄屏退化为单列堆叠（NFR-10）。
  模块边界：状态在 stores/workbench.ts；本组件只渲染与转发交互；
  不发起网络请求。
-->
<script setup lang="ts">
import { nextTick, ref, watch } from 'vue';
import { useWorkbenchStore, type Segment } from '../stores/workbench';

const doc = useWorkbenchStore();
const activeId = ref<number | null>(null);
/** dismissedEmpty 用户点过"开始写"后不再显示空态卡片（本次会话内）。 */
const dismissedEmpty = ref(false);

const emit = defineEmits<{ loadSample: []; setActive: [id: number]; proofread: [id: number] }>();

const sourceEls = new Map<number, HTMLTextAreaElement>();

/** virgin 文档还是"初始空白"状态：只有一段且从未输入/翻译——此时显示空态卡片（FR-111）。 */
const virgin = () =>
  doc.segments.length === 1 &&
  doc.segments[0].source === '' &&
  doc.segments[0].target === '' &&
  doc.segments[0].status === 'idle';

/** onFocusSource 记录光标所在段（Ctrl/⌘+Shift+Enter 只译该段，FR-106）。 */
function onFocusSource(seg: Segment) {
  activeId.value = seg.id;
  emit('setActive', seg.id);
}

/** startWrite 从空态进入写作：隐藏卡片并聚焦第一段。 */
async function startWrite() {
  dismissedEmpty.value = true;
  await nextTick();
  const first = sourceEls.get(doc.segments[0]?.id ?? -1);
  first?.focus();
}

/** autoGrow 让 textarea 高度跟随内容。 */
function autoGrow(el: HTMLTextAreaElement) {
  el.style.height = 'auto';
  el.style.height = `${el.scrollHeight}px`;
}

/** onSourceInput 处理源文输入：更新状态 → 检测空行边界递归拆段（FR-104/105）。 */
function onSourceInput(seg: Segment, e: Event) {
  const el = e.target as HTMLTextAreaElement;
  autoGrow(el);
  doc.updateSource(seg.id, el.value);
  if (/\n[ \t]*\n/.test(el.value)) {
    doc.normalizeBlanks(seg.id, el.selectionStart);
  }
}

/** onPaste 粘贴整篇：去掉尾部空行后并入源文再拆段（FR-104：空行不变成空输入框）。
 * 手动 Enter×2 仍走 input 事件路径开新段（FR-105），两者语义分开。 */
function onPaste(seg: Segment, e: ClipboardEvent) {
  const text = e.clipboardData?.getData('text/plain');
  if (!text || !/\n[ \t]*\n/.test(text)) return; // 无段落边界：走默认粘贴。
  e.preventDefault();
  const el = e.target as HTMLTextAreaElement;
  const start = el.selectionStart;
  const end = el.selectionEnd;
  const cleaned = text.replace(/\r\n/g, '\n').replace(/\n[ \t]*\n+$/, '\n');
  const next = el.value.slice(0, start) + cleaned + el.value.slice(end);
  doc.updateSource(seg.id, next);
  doc.normalizeBlanks(seg.id, start + cleaned.length);
  autoGrow(el);
}

/** onTargetInput 译文直接编辑（FR-108）。 */
function onTargetInput(seg: Segment, e: Event) {
  const el = e.target as HTMLTextAreaElement;
  autoGrow(el);
  doc.updateTarget(seg.id, el.value);
}

/** retry 重试失败段（TEXT_TOO_LONG/QUOTA 等不可重试的段落不给按钮，FR-305/308）。 */
function retry(seg: Segment) {
  const retryable = seg.error && !['TEXT_TOO_LONG', 'TOO_MANY_SEGMENTS', 'QUOTA_EXCEEDED', 'MODEL_NOT_CONFIGURED', 'UNSUPPORTED_LANGUAGE'].includes(seg.error.code);
  if (retryable) {
    void doc.translateOne(seg.id);
  }
}

/** noRetry 判断该失败是否值得给重试按钮。 */
function noRetry(seg: Segment): boolean {
  return !!seg.error && ['TEXT_TOO_LONG', 'TOO_MANY_SEGMENTS', 'QUOTA_EXCEEDED', 'MODEL_NOT_CONFIGURED', 'UNSUPPORTED_LANGUAGE'].includes(seg.error.code);
}

// 拆段后把焦点与光标落到目标段（Enter×2 流畅开新段）。
watch(
  () => doc.pendingFocus,
  async (f) => {
    if (!f) return;
    doc.pendingFocus = null;
    await nextTick();
    const el = sourceEls.get(f.id);
    if (el) {
      el.focus();
      el.setSelectionRange(f.offset, f.offset);
      autoGrow(el);
    }
  },
);

// 初始渲染与窗口变化时统一量高。
function growAll() {
  for (const el of sourceEls.values()) autoGrow(el);
}
watch(() => doc.segments.length, () => nextTick(growAll));
</script>

<template>
  <div class="split" data-test="split-pane">
    <div class="split__head">
      <div class="split__head-cell">English</div>
      <div class="split__head-cell">中文</div>
    </div>

    <div class="split__scroll">
      <!-- 空文档态（FR-111）：给说明 + 两个可点的动作 -->
      <div v-if="virgin() && !dismissedEmpty" class="split__empty" data-test="empty-state">
        <el-empty description="左边写下第一段英文，或粘贴整篇文章（空行会自动分段）">
          <div class="split__empty-actions">
            <el-button data-test="start-write" @click="startWrite">开始写</el-button>
            <el-button type="primary" data-test="load-sample" @click="$emit('loadSample')">载入示例</el-button>
          </div>
        </el-empty>
      </div>
      <div
        v-for="(seg, i) in doc.segments"
        v-show="!(virgin() && !dismissedEmpty)"
        :key="seg.id"
        class="split-row"
        :class="{ 'split-row--active': activeId === seg.id }"
        @mouseenter="activeId = seg.id"
        @mouseleave="activeId = null"
      >
        <!-- 左：源文（FR-104/105） -->
        <div class="split__cell split__cell--src">
          <span class="split__num" aria-hidden="true">{{ i + 1 }}</span>
          <textarea
            :ref="(el) => { if (el) { sourceEls.set(seg.id, el as HTMLTextAreaElement); autoGrow(el as HTMLTextAreaElement); } else { sourceEls.delete(seg.id); } }"
            class="split__input"
            :value="seg.source"
            placeholder="在这里写英文，或粘贴整篇（空行自动分段）"
            spellcheck="false"
            @input="onSourceInput(seg, $event)"
            @paste="onPaste(seg, $event)"
            @focus="onFocusSource(seg)"
          />
          <div class="split__src-meta">
            <span class="faint">{{ [...seg.source].length }} 字</span>
            <button
              v-if="doc.isEnglishish(seg.id) && seg.source.trim()"
              class="split__mini"
              :data-test="`proofread-btn-${i}`"
              title="检查这一段的拼写与语法"
              @click="emit('proofread', seg.id)"
            >纠错</button>
            <button class="split__mini" title="在这段后面插入新段" @click="doc.insertAfter(seg.id)">＋段</button>
            <button class="split__mini split__mini--danger" title="删除这一段" @click="doc.remove(seg.id)">删</button>
          </div>
        </div>

        <!-- 右：译文状态机（FR-107/108 + 六态） -->
        <div class="split__cell split__cell--dst" :data-status="seg.status">
          <template v-if="seg.status === 'pending'">
            <div class="split__skeleton" data-test="seg-loading">
              <el-skeleton :rows="2" animated />
            </div>
          </template>
          <template v-else-if="seg.status === 'error'">
            <div class="split__error" data-test="seg-error" role="alert">
              <p class="split__error-msg">{{ seg.error?.message }}</p>
              <div class="split__error-actions">
                <el-button
                  v-if="!noRetry(seg)"
                  size="small"
                  data-test="seg-retry"
                  @click="retry(seg)"
                >重试本段</el-button>
                <span v-else class="faint">{{ seg.error?.code === 'TEXT_TOO_LONG' ? '请把这一段拆短再翻译' : '按提示处理后重试' }}</span>
              </div>
            </div>
          </template>
          <template v-else-if="seg.status === 'done'">
            <textarea
              class="split__input split__input--dst"
              :class="{ 'split__input--edited': seg.edited }"
              :value="seg.target"
              placeholder="（译文为空）"
              spellcheck="false"
              @input="onTargetInput(seg, $event)"
            />
            <div class="split__dst-meta">
              <el-tag v-if="seg.edited" size="small" type="warning" effect="plain">已手改</el-tag>
              <span v-if="seg.ms !== undefined" class="faint">{{ seg.ms }}ms</span>
            </div>
          </template>
          <template v-else>
            <div class="split__idle faint" data-test="seg-idle">按 Ctrl/⌘ + Enter 翻译全文</div>
          </template>
        </div>
      </div>

    </div>
  </div>
</template>

<style scoped>
.split {
  display: flex;
  flex-direction: column;
  min-height: 0;
  flex: 1;
}

/* 表头与列宽：同一 grid 模板，保证左右列宽一致。 */
.split__head,
.split__scroll {
  display: grid;
  grid-template-columns: minmax(0, 1fr) minmax(0, 1fr);
  gap: 0 var(--sp-3);
}

.split__head {
  position: sticky;
  top: 0;
  z-index: 2;
  padding: var(--sp-2) 0;
  background: var(--bg);
  border-bottom: 1px solid var(--border);
}

.split__head-cell {
  font-size: 12px;
  font-weight: 600;
  color: var(--ink-3);
  letter-spacing: 0.05em;
}

.split__scroll {
  flex: 1;
  overflow-y: auto;
  align-content: start;
  padding: var(--sp-2) 0 var(--sp-6);
}

/* 每段一行：display:contents 让两格直接成为外层 grid 的行内两列，
   行高由较高一格决定 → 逐行顶端对齐（FR-102）。 */
.split-row {
  display: contents;
}

.split__cell {
  position: relative;
  padding: var(--sp-2) var(--sp-2) var(--sp-2) var(--sp-4);
  border-radius: var(--radius-sm);
  transition: background 0.1s ease;
}

/* hover 行联动高亮（FR-103）。 */
.split-row:hover > .split__cell {
  background: var(--surface-2);
}

.split__num {
  position: absolute;
  left: 2px;
  top: 10px;
  width: 18px;
  height: 18px;
  border-radius: 50%;
  background: var(--surface-3);
  color: var(--ink-2);
  font-size: 11px;
  line-height: 18px;
  text-align: center;
  user-select: none;
}

.split__input {
  width: 100%;
  min-height: 44px;
  border: 0;
  background: transparent;
  color: var(--ink);
  font-family: var(--font);
  font-size: 14px;
  line-height: 1.7;
  resize: none;
  overflow: hidden;
  display: block;
}

.split__input:focus {
  outline: none;
}

.split__input--dst {
  color: var(--ink);
}

.split__input--edited {
  box-shadow: -2px 0 0 var(--warn);
  padding-left: 6px;
}

.split__src-meta,
.split__dst-meta {
  display: flex;
  align-items: center;
  gap: var(--sp-2);
  margin-top: 2px;
  font-size: 11px;
  min-height: 18px;
}

.split__mini {
  border: 0;
  background: none;
  color: var(--ink-3);
  cursor: pointer;
  font-size: 11px;
  padding: 0;
  opacity: 0;
  transition: opacity 0.12s ease;
}

.split-row:hover .split__mini {
  opacity: 1;
}

.split__mini--danger:hover {
  color: var(--danger);
}

/* 右格各状态。 */
.split__cell--dst[data-status='error'] {
  background: var(--danger-soft);
}

.split__error-msg {
  margin: 0 0 var(--sp-1);
  color: var(--danger);
  font-size: 13px;
}

.split__error-actions {
  display: flex;
  align-items: center;
  gap: var(--sp-2);
}

.split__idle {
  font-size: 13px;
  padding-top: 4px;
}

.split__empty {
  grid-column: 1 / -1;
  padding: var(--sp-6) 0;
}

/* 窄屏：单列堆叠，保持"段 N 的英文在上、中文在下"（NFR-10）。 */
@media (max-width: 900px) {
  .split__head {
    display: none;
  }
  .split__head,
  .split__scroll {
    grid-template-columns: minmax(0, 1fr);
  }
  .split-row {
    display: block;
    border-bottom: 1px dashed var(--border);
    padding-bottom: var(--sp-2);
    margin-bottom: var(--sp-2);
  }
  .split__cell--dst {
    border-left: 2px solid var(--border-strong);
  }
}
</style>

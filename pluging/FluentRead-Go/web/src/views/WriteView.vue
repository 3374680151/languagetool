<!--
  WriteView.vue 是写作台主页（FR-1xx 全量，P1 链路 + P2 完整体）。
  @file web/src/views/WriteView.vue
  文件职责：工具栏（语言对/风格/翻译按钮/导出/术语/纠错入口）+ SplitPane 两栏；
  全局快捷键 Ctrl/⌘+Enter 全译、Ctrl/⌘+Shift+Enter 译光标段（FR-106）；
  横幅体系：未配置/降级/额度/术语过期/取消翻译（六态，P1.5/P2 逐个接通）；
  手改后重译确认（FR-109/504）；空文档示例（FR-111）；草稿自动恢复（FR-110）。
  模块边界：状态在 stores；网络只经 api/client；组件不直接 fetch。
-->
<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue';
import { ArrowRight, Switch } from '@element-plus/icons-vue';
import SplitPane from '../components/SplitPane.vue';
import { useWorkbenchStore } from '../stores/workbench';
import { useSettingsStore } from '../stores/settings';
import { useQuotaStore } from '../stores/quota';
import { useUiStore } from '../stores/ui';
import { Languages } from '../lib/lang';
import { buildMarkdown, downloadText, exportFileName, type ExportMode } from '../lib/markdown';

const doc = useWorkbenchStore();
const settings = useSettingsStore();
const quota = useQuotaStore();
const ui = useUiStore();

const activeSegId = ref<number | null>(null);

// 语言对下拉：目标语言不含 auto（FR-101/112：默认英→中，可改但新访客默认不变）。
const fromOptions = Languages;
const toOptions = Languages.filter((l) => l.id !== 'auto');
const styleOptions = [
  { value: 'academic', label: '学术' },
  { value: 'formal', label: '正式' },
  { value: 'concise', label: '简洁' },
  { value: 'casual', label: '口语' },
];

const SAMPLE = [
  'Retrieval-augmented generation improves answer quality by grounding the model in retrieved documents.',
  'Before answering, the system first recalls the most relevant passages from the knowledge base.',
  'Terminology is locked at the site level, which keeps the wording consistent across a long article.',
  'Errors must tell the reader what to do next, not merely that something went wrong.',
].join('\n\n');

/** loadSample 载入示例文档（FR-111：空文档给一个可点的动作）。 */
function loadSample() {
  doc.setDocument(SAMPLE);
  ui.toast('info', '示例已载入，按 Ctrl/⌘ + Enter 翻译');
}

/** translateAll 入口：手改段将被覆盖时确认一次（FR-109/504）。 */
async function onTranslateAll(force = false) {
  if (doc.translating) {
    doc.cancel();
    return;
  }
  if (force && doc.stats.edited > 0) {
    const okToOverwrite = await ui.askConfirm(
      '全部重译',
      `有 ${doc.stats.edited} 段译文被你手动改过，全部重译会覆盖这些修改。继续吗？`,
      '全部重译',
    );
    if (!okToOverwrite) return;
  }
  const before = doc.stats.done;
  await doc.translateAll({ force });
  syncQuota(before);
}

/** syncQuota 翻译后同步额度（FR-606：快用完才提示）。 */
async function syncQuota(beforeDone: number) {
  const delta = doc.stats.done - beforeDone;
  if (delta > 0) {
    quota.consume(delta);
  }
  if (!quota.loaded) {
    await quota.refresh();
  } else if (quota.lowQuota) {
    // 低额度时以服务端为准，防止本地累计漂移。
    await quota.refresh();
  }
}

/** onSingleTranslate 译光标所在段（Ctrl/⌘+Shift+Enter，FR-106）。 */
async function onSingleTranslate() {
  if (activeSegId.value == null) {
    ui.toast('warn', '先把光标点进要翻译的段落');
    return;
  }
  const seg = doc.segments.find((s) => s.id === activeSegId.value);
  if (seg && seg.edited && seg.status === 'done') {
    const ok = await ui.askConfirm('重译这一段', '这段译文被你手动改过，重译会覆盖修改。继续吗？', '重译');
    if (!ok) return;
  }
  const before = doc.stats.done;
  await doc.translateOne(activeSegId.value!);
  syncQuota(before);
}

/** exportAs 导出 Markdown 三种模式（FR-801/802）。 */
function exportAs(mode: ExportMode) {
  const segments = doc.segments
    .filter((s) => s.source.trim())
    .map((s) => ({ source: s.source, target: s.target, hasTranslation: s.status === 'done' }));
  if (segments.length === 0) {
    ui.toast('warn', '还没有内容可导出');
    return;
  }
  downloadText(exportFileName(mode), buildMarkdown(mode, segments));
  ui.toast('success', '已导出 Markdown');
}

/** 全局快捷键（FR-106：阻止浏览器默认行为）。 */
function onKeydown(e: KeyboardEvent) {
  const mod = e.metaKey || e.ctrlKey;
  if (!mod || e.key !== 'Enter') return;
  e.preventDefault();
  if (e.shiftKey) {
    void onSingleTranslate();
  } else {
    void onTranslateAll(false);
  }
}

onMounted(() => {
  document.documentElement.classList.toggle('dark', settings.theme === 'dark');
  if (!doc.restoreDraft()) {
    doc.clearAll();
  }
  void quota.refresh();
  window.addEventListener('keydown', onKeydown);
  window.addEventListener('pagehide', saveNow);
});
onBeforeUnmount(() => {
  window.removeEventListener('keydown', onKeydown);
  window.removeEventListener('pagehide', saveNow);
  doc.saveNow();
});

function saveNow() {
  doc.saveNow();
}

function setActive(id: number) {
  activeSegId.value = id;
}
</script>

<template>
  <main class="page__body write">
    <!-- 工具栏 -->
    <div class="toolbar">
      <div class="toolbar__group">
        <el-select
          :model-value="settings.prefs.langPair.from"
          size="small"
          class="toolbar__lang"
          data-test="lang-from"
          aria-label="源语言"
          @update:model-value="(v: string) => settings.patch({ langPair: { from: v, to: settings.prefs.langPair.to } })"
        >
          <el-option v-for="l in fromOptions" :key="l.id" :value="l.id" :label="l.label" />
        </el-select>
        <el-button
          size="small"
          :icon="Switch"
          circle
          title="互换语言"
          aria-label="互换语言"
          @click="settings.swap()"
        />
        <ArrowRight class="toolbar__arrow" />
        <el-select
          :model-value="settings.prefs.langPair.to"
          size="small"
          class="toolbar__lang"
          data-test="lang-to"
          aria-label="目标语言"
          @update:model-value="(v: string) => settings.patch({ langPair: { from: settings.prefs.langPair.from, to: v } })"
        >
          <el-option v-for="l in toOptions" :key="l.id" :value="l.id" :label="l.label" />
        </el-select>
      </div>

      <el-select
        :model-value="settings.prefs.style"
        size="small"
        class="toolbar__style"
        data-test="style-select"
        aria-label="风格"
        @update:model-value="(v: string) => settings.patch({ style: v })"
      >
        <el-option v-for="s in styleOptions" :key="s.value" :value="s.value" :label="s.label" />
      </el-select>

      <el-button
        type="primary"
        size="small"
        data-test="translate-all"
        :loading="doc.translating"
        @click="onTranslateAll(false)"
      >{{ doc.translating ? `取消（${doc.waitedSeconds}s）` : '翻译全部' }}</el-button>

      <el-dropdown trigger="click" @command="exportAs">
        <el-button size="small" data-test="export-btn">导出</el-button>
        <template #dropdown>
          <el-dropdown-menu>
            <el-dropdown-item command="bilingual" data-test="export-bilingual">中英对照</el-dropdown-item>
            <el-dropdown-item command="en">仅英文</el-dropdown-item>
            <el-dropdown-item command="zh">仅中文</el-dropdown-item>
          </el-dropdown-menu>
        </template>
      </el-dropdown>

      <span class="toolbar__stats faint" data-test="stats">
        {{ doc.stats.done }}/{{ doc.stats.total }} 段已译<template v-if="doc.stats.failed"> · {{ doc.stats.failed }} 段失败</template><template v-if="doc.stats.edited"> · {{ doc.stats.edited }} 段手改</template>
      </span>
    </div>

    <!-- 横幅：未配置 / 降级 / 超时取消（六态） -->
    <el-alert
      v-if="doc.unconfigured"
      type="warning"
      :closable="false"
      show-icon
      data-test="banner-unconfigured"
      title="站点还没有配置翻译模型"
      description="写作与草稿保存照常可用；配置好模型后翻译立即生效。"
    />
    <el-alert
      v-if="doc.degradedReason"
      type="warning"
      :closable="false"
      show-icon
      data-test="banner-degraded"
      title="本站今日付费翻译额度已用完，已自动切到免费方式"
      description="译文仍然可用，但质量可能下降，明天自动恢复。"
    />

    <el-alert
      v-if="quota.lowQuota && !quota.exhausted"
      type="warning"
      :closable="false"
      show-icon
      data-test="banner-quota-low"
      :title="`今日还剩 ${quota.remaining} 段可翻译，${quota.resetAtText()}恢复`"
    />
    <el-alert
      v-if="quota.exhausted"
      type="error"
      :closable="false"
      show-icon
      data-test="banner-quota-out"
      :title="`今日额度已用完，${quota.resetAtText()}恢复`"
    />

    <!-- 双栏对照区 -->
    <SplitPane @load-sample="loadSample" @set-active="setActive" />
  </main>
</template>

<style scoped>
.write {
  display: flex;
  flex-direction: column;
  min-height: calc(100vh - 48px);
}

.toolbar {
  display: flex;
  align-items: center;
  gap: var(--sp-2);
  flex-wrap: wrap;
  padding: var(--sp-2) 0;
}

.toolbar__group {
  display: flex;
  align-items: center;
  gap: var(--sp-1);
}

.toolbar__lang {
  width: 120px;
  flex: none;
}

.toolbar__style {
  width: 96px;
  flex: none;
}

.toolbar__arrow {
  color: var(--ink-3);
}

.toolbar__stats {
  margin-left: auto;
  font-size: 12px;
}

@media (max-width: 480px) {
  .toolbar__stats {
    width: 100%;
    margin-left: 0;
  }
  .toolbar__lang {
    width: 104px;
  }
}
</style>

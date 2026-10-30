<!--
  WordsView.vue 是词表页（FR-702/703/704/1202 + AD-10，P5）。
  @file web/src/views/WordsView.vue
  文件职责：收藏词列表（原句上下文/使用次数/复习到期时间）、删除（带确认）、
  空态（说明 + 去写作台/查词收藏引导）、按最近收藏/最少使用排序（FR-1202）、
  搜索框查词（AD-10：输入词 + 回车 → 固定位置弹卡，不受触发模式影响）。
  模块边界：数据在 api/client 直连（词表是本页的核心数据源，无独立 store）；
  弹卡复用全局 WordPopup 的 lookupAt。
-->
<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { deleteWord, getDict, getWords } from '../api/client';
import type { DictEntry, Word } from '../api/types';
import { useUiStore } from '../stores/ui';

const ui = useUiStore();

const words = ref<Word[]>([]);
const sort = ref<'recent' | 'least_used'>('recent');
const loading = ref(true);
const loadError = ref('');
const searchQuery = ref('');

// 搜索框弹卡（AD-10）：固定位置渲染词典结果。
const cardState = ref<{
  visible: boolean;
  loading: boolean;
  error: string;
  entry: DictEntry | null;
}>({ visible: false, loading: false, error: '', entry: null });

async function refresh() {
  loading.value = true;
  loadError.value = '';
  try {
    words.value = await getWords(sort.value);
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '词表加载失败，请稍后重试';
  } finally {
    loading.value = false;
  }
}

async function removeWord(w: Word) {
  const ok = await ui.askConfirm('删除收藏词', `确定删除"${w.word}"吗？删除后该词的复习也会停止。`, '删除');
  if (!ok) return;
  await deleteWord(w.id);
  await refresh();
  ui.toast('success', '已删除');
}

async function switchSort(v: 'recent' | 'least_used') {
  sort.value = v;
  await refresh();
}

/** searchLookup AD-10：词表页搜索框 → 回车/按钮 → 固定位置弹卡。 */
async function searchLookup() {
  const q = searchQuery.value.trim().toLowerCase();
  if (!q) return;
  cardState.value = { visible: true, loading: true, error: '', entry: null };
  try {
    const entry = await getDict(q);
    cardState.value.entry = entry;
  } catch (e) {
    cardState.value.error = e instanceof Error ? e.message : '没查到这个词，请稍后再试';
  } finally {
    cardState.value.loading = false;
  }
}

/** collectFromSearch 把搜索到的词收藏进词表（无原句，FR-1204 退化为词→释义卡）。 */
async function collectFromSearch() {
  const entry = cardState.value.entry;
  if (!entry || entry.collected.yes) return;
  try {
    const { collectWord } = await import('../api/client');
    await collectWord({ word: entry.word, lang: 'en' });
    entry.collected = { yes: true, useCount: entry.collected.useCount };
    cardState.value.entry = { ...entry };
    await refresh();
    ui.toast('success', '已收藏（无原句，复习时退化为词→释义卡）');
  } catch (e) {
    ui.toast('error', e instanceof Error ? e.message : '收藏失败，请稍后重试');
  }
}

/** dueText 复习到期时间的人话。 */
function dueText(w: Word): string {
  if (!w.nextDueAt) return '未排期';
  const d = new Date(w.nextDueAt);
  const diff = d.getTime() - Date.now();
  if (diff <= 0) return '现在可复习';
  const minutes = Math.round(diff / 60000);
  if (minutes < 60) return `${minutes} 分钟后`;
  const hours = Math.round(minutes / 60);
  if (hours < 24) return `${hours} 小时后`;
  return `${Math.round(hours / 24)} 天后`;
}

onMounted(refresh);
</script>

<template>
  <main class="page__body words">
    <section class="card block">
      <div class="head">
        <h1>词表</h1>
        <div class="head-actions">
          <el-radio-group :model-value="sort" size="small" data-test="words-sort" @update:model-value="(v: 'recent' | 'least_used') => switchSort(v)">
            <el-radio-button value="recent">最近收藏</el-radio-button>
            <el-radio-button value="least_used">最少使用</el-radio-button>
          </el-radio-group>
          <el-input
            v-model="searchQuery"
            size="small"
            class="search"
            data-test="words-search"
            placeholder="查一个词（回车）"
            @keyup.enter="searchLookup"
          >
            <template #append>
              <el-button data-test="words-search-btn" @click="searchLookup">查词</el-button>
            </template>
          </el-input>
        </div>
      </div>

      <!-- 加载态（六态） -->
      <div v-if="loading" data-test="words-loading" class="pad">
        <el-skeleton :rows="4" animated />
      </div>

      <!-- 失败态（六态） -->
      <el-result v-else-if="loadError" icon="warning" title="词表加载失败" :sub-title="loadError">
        <template #extra>
          <el-button type="primary" data-test="words-retry" @click="refresh">重试</el-button>
        </template>
      </el-result>

      <!-- 空态（FR-704：说明 + 下一步动作） -->
      <el-empty
        v-else-if="words.length === 0"
        data-test="words-empty"
        description="词表还是空的。写作台里划选生词 → 点卡片上的「收藏」，它会带着你写的原句进来；复习时挖空的就是这句。"
      >
        <el-button type="primary" @click="searchQuery = 'recall'; searchLookup()">试试查一个词</el-button>
      </el-empty>

      <!-- 列表 -->
      <ul v-else class="word-list" data-test="words-list">
        <li v-for="w in words" :key="w.id" class="word-row" :data-test="`word-row-${w.word}`">
          <div class="word-main">
            <span class="word">{{ w.word }}</span>
            <span class="faint use-count" data-test="word-use-count">用过 {{ w.useCount }} 次</span>
            <span class="faint due" data-test="word-due">复习：{{ dueText(w) }}</span>
          </div>
          <p v-if="w.contextSentence" class="context muted" data-test="word-context">{{ w.contextSentence }}</p>
          <p v-else class="context faint" data-test="word-context">（收藏时没有原句）</p>
          <el-button size="small" text type="danger" :data-test="`word-del-${w.word}`" @click="removeWord(w)">删除</el-button>
        </li>
      </ul>
    </section>

    <!-- 搜索弹卡（AD-10：固定位置，不受触发模式影响） -->
    <div v-if="cardState.visible" class="card search-card" data-test="words-search-card">
      <div v-if="cardState.loading" class="pad"><el-skeleton :rows="2" animated /></div>
      <div v-else-if="cardState.error" class="pad">
        <p class="muted">{{ cardState.error }}</p>
        <el-button size="small" @click="searchLookup">重试</el-button>
      </div>
      <template v-else-if="cardState.entry">
        <div class="wp-head">
          <span class="word">{{ cardState.entry.word }}</span>
          <span v-if="cardState.entry.phonetic" class="faint">{{ cardState.entry.phonetic }}</span>
          <el-tag v-if="cardState.entry.source === 'model'" size="small" type="warning" effect="plain">AI 生成，仅供参考</el-tag>
        </div>
        <p v-if="cardState.entry.typoHint" class="hint">ⓘ 可能是 {{ cardState.entry.typoHint.correctTo }} 的拼写错误</p>
        <ul class="senses">
          <li v-for="(s, i) in cardState.entry.senses" :key="i">
            <span v-if="s.pos" class="faint">{{ s.pos }}</span>{{ s.def }}
          </li>
        </ul>
        <div class="card-actions">
          <button
            v-if="!cardState.entry.collected.yes"
            class="collect-btn" data-test="words-collect-btn"
            @click="collectFromSearch"
          >☆ 收藏</button>
          <span v-else class="collected" data-test="words-collected">✓ 已收藏</span>
          <el-button size="small" text @click="cardState.visible = false">关闭</el-button>
        </div>
      </template>
    </div>
  </main>
</template>

<style scoped>
.words {
  max-width: 860px;
}

.block {
  padding: var(--sp-4) var(--sp-5);
}

.head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: var(--sp-3);
  flex-wrap: wrap;
  margin-bottom: var(--sp-3);
}

.head h1 {
  font-size: 16px;
  margin: 0;
}

.head-actions {
  display: flex;
  align-items: center;
  gap: var(--sp-2);
  flex-wrap: wrap;
}

.search {
  width: 220px;
}

.pad {
  padding: var(--sp-3) 0;
}

.word-list {
  list-style: none;
  margin: 0;
  padding: 0;
  display: flex;
  flex-direction: column;
  gap: var(--sp-2);
}

.word-row {
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: var(--sp-2) var(--sp-3);
  display: flex;
  flex-direction: column;
  gap: 2px;
}

.word-main {
  display: flex;
  align-items: baseline;
  gap: var(--sp-3);
  flex-wrap: wrap;
}

.word {
  font-weight: 700;
  font-size: 14px;
}

.use-count,
.due {
  font-size: 12px;
}

.context {
  font-size: 12.5px;
  margin: 0;
}

.word-row .el-button {
  align-self: flex-end;
}

.search-card {
  position: fixed;
  right: 24px;
  bottom: 24px;
  z-index: 500;
  width: min(340px, calc(100vw - 32px));
  padding: var(--sp-3) var(--sp-4);
  box-shadow: var(--shadow);
}

.wp-head {
  display: flex;
  align-items: baseline;
  gap: 8px;
  flex-wrap: wrap;
}

.senses {
  margin: 8px 0;
  padding-left: 18px;
  line-height: 1.7;
  font-size: 13px;
}

.card-actions {
  display: flex;
  align-items: center;
  gap: 10px;
  border-top: 1px solid var(--border);
  padding-top: 8px;
}

.collect-btn {
  border: 1px solid var(--primary);
  color: var(--primary);
  background: var(--surface);
  border-radius: var(--radius-sm);
  font-size: 12.5px;
  padding: 3px 10px;
  cursor: pointer;
}

.collected {
  color: var(--ok);
  font-size: 12.5px;
}
</style>

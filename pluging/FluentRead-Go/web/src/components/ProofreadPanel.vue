<!--
  ProofreadPanel.vue 是纠错建议面板（FR-2xx，P3）。
  @file web/src/components/ProofreadPanel.vue
  文件职责：以抽屉呈现当前段的建议列表（original→suggestion→category→reason 四字段，
  FR-203）；逐条 接受/忽略（FR-205：接受后由父级重新校验剩余条目位置）；
  没有错误时明确说"这段没有发现问题"（FR-209）；加载/失败态齐备（六态）。
  不做整段重写（FR-208）；面板打开不阻塞继续写作（非模态由 el-drawer 承担）。
  模块边界：数据由 WriteView 传入（已是 service 校验后的结果）；本组件只渲染与发事件。
-->
<script setup lang="ts">
import type { Suggestion } from '../api/types';

const props = defineProps<{
  open: boolean;
  loading: boolean;
  error: string;
  segmentNo: number;
  suggestions: Suggestion[];
  appliedIds: number[];
}>();

const emit = defineEmits<{
  close: [];
  accept: [index: number];
  ignore: [index: number];
  retry: [];
}>();

const categoryLabels: Record<string, string> = {
  spelling: '拼写',
  grammar: '语法',
  punctuation: '标点',
  style: '风格',
  word_choice: '用词',
};

function categoryTagType(category: string): 'danger' | 'warning' | 'info' | 'success' {
  switch (category) {
    case 'spelling':
      return 'danger';
    case 'grammar':
      return 'warning';
    default:
      return 'info';
  }
}

const isEmpty = () => !props.loading && !props.error && props.suggestions.length === 0;
</script>

<template>
  <el-drawer
    :model-value="open"
    :title="`第 ${segmentNo} 段 · 纠错建议`"
    size="420px"
    append-to-body
    data-test="proofread-panel"
    @close="emit('close')"
  >
    <!-- 加载态（六态） -->
    <div v-if="loading" class="proof-loading" data-test="proofread-loading">
      <el-skeleton :rows="4" animated />
    </div>

    <!-- 失败态（六态）：原因 + 重试 -->
    <el-result
      v-else-if="error"
      icon="warning"
      title="纠错暂时不可用"
      :sub-title="error"
    >
      <template #extra>
        <el-button type="primary" data-test="proofread-retry" @click="emit('retry')">重试</el-button>
      </template>
    </el-result>

    <!-- 空态（FR-209）：明确说没有发现问题 -->
    <el-result
      v-else-if="isEmpty()"
      icon="success"
      title="这段没有发现问题"
      sub-title="模型逐条检查了拼写、语法、标点、风格与用词，没有要改的地方。"
      data-test="proofread-empty"
    />

    <!-- 建议列表 -->
    <div v-else class="proof-list">
      <div
        v-for="(s, i) in suggestions"
        :key="`${s.original}-${i}`"
        class="proof-item"
        :class="{ 'proof-item--applied': appliedIds.includes(i) }"
        :data-test="`suggestion-${i}`"
      >
        <div class="proof-item__head">
          <el-tag size="small" :type="categoryTagType(s.category)">{{ categoryLabels[s.category] ?? s.category }}</el-tag>
          <span v-if="appliedIds.includes(i)" class="applied-mark">已接受</span>
        </div>
        <div class="proof-item__diff">
          <span class="diff-original">{{ s.original }}</span>
          <span class="diff-arrow">→</span>
          <span class="diff-suggestion">{{ s.suggestion }}</span>
        </div>
        <p class="proof-item__reason">{{ s.reason }}</p>
        <div class="proof-item__actions">
          <el-button size="small" type="primary" :disabled="appliedIds.includes(i)" @click="emit('accept', i)">
            接受
          </el-button>
          <el-button size="small" :disabled="appliedIds.includes(i)" @click="emit('ignore', i)">忽略</el-button>
        </div>
      </div>
      <p class="proof-foot faint">接受会直接改左边的原文；忽略只关掉这一条。改完原文后记得重新翻译这一段。</p>
    </div>
  </el-drawer>
</template>

<style scoped>
.proof-loading {
  padding: var(--sp-3);
}

.proof-list {
  display: flex;
  flex-direction: column;
  gap: var(--sp-3);
}

.proof-item {
  border: 1px solid var(--border);
  border-radius: var(--radius);
  padding: var(--sp-3);
  background: var(--surface);
}

.proof-item--applied {
  opacity: 0.55;
}

.proof-item__head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: var(--sp-2);
}

.applied-mark {
  color: var(--ok);
  font-size: 12px;
}

.proof-item__diff {
  font-size: 13px;
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  align-items: baseline;
}

.diff-original {
  text-decoration: line-through;
  color: var(--danger);
}

.diff-arrow {
  color: var(--ink-3);
}

.diff-suggestion {
  color: var(--ok);
  font-weight: 600;
}

.proof-item__reason {
  margin: var(--sp-2) 0;
  font-size: 12.5px;
  color: var(--ink-2);
  line-height: 1.7;
}

.proof-item__actions {
  display: flex;
  gap: var(--sp-2);
}

.proof-foot {
  font-size: 12px;
  margin: var(--sp-2) 0 0;
}
</style>

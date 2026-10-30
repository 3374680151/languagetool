<!--
  ReviewView.vue 是复习页（FR-1204，P5.5）。
  @file web/src/views/ReviewView.vue
  文件职责：到期队列逐张翻卡——收藏词挖空你自己的句子、错误本挖空正确改法；
  提示按需展开；两键评分"记住 / 没记住"（间隔调度 10min/1/3/7/14/30 天，
  没记住回退一档，由服务端结算）；完成态给总结。不弹窗、不常驻顶栏。
  模块边界：数据经 api/client；卡片渲染在本组件。
-->
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { getReviewQueue, postReviewScore } from '../api/client';
import type { ReviewItem } from '../api/types';
import { useUiStore } from '../stores/ui';

const ui = useUiStore();

const queue = ref<ReviewItem[]>([]);
const idx = ref(0);
const loading = ref(true);
const loadError = ref('');
const revealed = ref(false);
const scoring = ref(false);
const doneCount = ref(0);
const rememberedCount = ref(0);

const current = computed(() => queue.value[idx.value] ?? null);
const total = computed(() => queue.value.length);

async function refresh() {
  loading.value = true;
  loadError.value = '';
  try {
    const resp = await getReviewQueue();
    queue.value = resp.items;
    idx.value = 0;
    doneCount.value = 0;
    rememberedCount.value = 0;
  } catch (e) {
    loadError.value = e instanceof Error ? e.message : '复习队列加载失败，请稍后重试';
  } finally {
    loading.value = false;
  }
}

/** score 两键评分：记住/没记住都进下一张（FR-1204）。 */
async function score(remembered: boolean) {
  const item = current.value;
  if (!item || scoring.value) return;
  scoring.value = true;
  try {
    await postReviewScore({ kind: item.kind, id: item.id, remembered });
    doneCount.value++;
    if (remembered) rememberedCount.value++;
    revealed.value = false;
    idx.value++;
  } catch (e) {
    ui.toast('error', e instanceof Error ? e.message : '评分失败，请稍后重试');
  } finally {
    scoring.value = false;
  }
}

/** clozeRender 把 ____ 高亮。 */
function clozeHtml(cloze: string): string {
  return cloze.replace(/____/g, '<span class="blank">____</span>');
}

onMounted(refresh);
</script>

<template>
  <main class="page__body review">
    <section class="card block">
      <h1>复习</h1>
      <p class="muted hint-line">
        材料全部来自你自己的写作：收藏词挖空你的原句，错误本挖空正确改法。
        评分两键——记住 / 没记住，间隔 10 分钟 / 1 / 3 / 7 / 14 / 30 天，没记住回退重排。
      </p>

      <!-- 加载态（六态） -->
      <div v-if="loading" class="pad" data-test="review-loading">
        <el-skeleton :rows="3" animated />
      </div>

      <!-- 失败态（六态） -->
      <el-result v-else-if="loadError" icon="warning" title="复习队列加载失败" :sub-title="loadError">
        <template #extra>
          <el-button type="primary" data-test="review-retry" @click="refresh">重试</el-button>
        </template>
      </el-result>

      <!-- 空态（六态）：没有到期卡 -->
      <el-empty
        v-else-if="total === 0"
        data-test="review-empty"
        description="现在没有到期的复习。收藏词与接受的纠错会在 10 分钟后开始进入复习队列。"
      />

      <!-- 完成态 -->
      <el-result
        v-else-if="!current"
        icon="success"
        data-test="review-done"
        :title="`本轮完成：${doneCount} 张`"
        :sub-title="`记住 ${rememberedCount} · 没记住 ${doneCount - rememberedCount}；没记住的已回退重排，稍后还会出现。`"
      >
        <template #extra>
          <el-button type="primary" @click="refresh">再查一次</el-button>
        </template>
      </el-result>

      <!-- 翻卡 -->
      <div v-else class="flash" data-test="review-card">
        <div class="flash-meta">
          <el-tag size="small" :type="current.kind === 'word' ? 'success' : 'warning'" effect="plain">
            {{ current.kind === 'word' ? '收藏词' : '错误本' }}
          </el-tag>
          <span class="faint">{{ idx + 1 }} / {{ total }}</span>
          <span v-if="current.category" class="faint">类别：{{ current.category }}</span>
        </div>

        <p v-if="current.kind === 'word'" class="flash-prompt" data-test="review-prompt">
          这个词你写过：<strong>{{ current.prompt }}</strong>
        </p>
        <p v-else class="flash-prompt" data-test="review-prompt">{{ current.prompt }}</p>

        <!-- 挖空句（你自己的句子） -->
        <blockquote class="flash-cloze" data-test="review-cloze">
          <!-- eslint-disable-next-line vue/no-v-html —— 内容来自用户自己的句子，仅做空白高亮替换 -->
          <span v-html="clozeHtml(current.cloze)" />
        </blockquote>

        <!-- 提示按需展开 -->
        <p v-if="current.hint && revealed" class="flash-hint" data-test="review-hint">提示：{{ current.hint }}</p>
        <p v-if="revealed && current.kind === 'mistake'" class="flash-answer" data-test="review-answer">
          正确改法：<strong>{{ current.answer }}</strong>
        </p>
        <p v-if="revealed && current.kind === 'word'" class="flash-answer" data-test="review-answer">
          答案：<strong>{{ current.answer }}</strong>
        </p>
        <p v-if="revealed && current.reason" class="flash-reason muted" data-test="review-reason">{{ current.reason }}</p>

        <div class="flash-actions">
          <el-button v-if="!revealed" size="large" data-test="review-reveal" @click="revealed = true">显示答案</el-button>
          <template v-else>
            <el-button type="success" size="large" :loading="scoring" data-test="review-remember" @click="score(true)">记住</el-button>
            <el-button type="danger" size="large" :loading="scoring" data-test="review-forget" @click="score(false)">没记住</el-button>
          </template>
        </div>
      </div>
    </section>
  </main>
</template>

<style scoped>
.review {
  max-width: 760px;
}

.block {
  padding: var(--sp-4) var(--sp-5);
}

.block h1 {
  font-size: 16px;
  margin: 0 0 var(--sp-2);
}

.hint-line {
  font-size: 12.5px;
  margin: 0 0 var(--sp-4);
}

.pad {
  padding: var(--sp-3) 0;
}

.flash {
  border-top: 1px solid var(--border);
  padding-top: var(--sp-4);
}

.flash-meta {
  display: flex;
  align-items: center;
  gap: var(--sp-3);
  margin-bottom: var(--sp-3);
  font-size: 12px;
}

.flash-prompt {
  font-size: 15px;
  margin: 0 0 var(--sp-2);
}

.flash-cloze {
  margin: 0 0 var(--sp-3);
  padding: var(--sp-3) var(--sp-4);
  background: var(--surface-2);
  border-left: 3px solid var(--primary);
  border-radius: var(--radius-sm);
  font-size: 14px;
  line-height: 1.8;
}

.blank {
  color: var(--primary);
  font-weight: 700;
  letter-spacing: 2px;
}

.flash-hint,
.flash-answer,
.flash-reason {
  margin: var(--sp-2) 0;
  font-size: 13px;
}

.flash-answer strong {
  color: var(--ok);
}

.flash-actions {
  display: flex;
  gap: var(--sp-3);
  margin-top: var(--sp-4);
}
</style>

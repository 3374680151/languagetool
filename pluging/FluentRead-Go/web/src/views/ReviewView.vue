<!--
  ReviewView.vue 是复习页（P0 占位，P5.5 填充挖空复习）。
  @file web/src/views/ReviewView.vue
  文件职责：挖空自己的句子 + 两键评分（FR-1204）；P0 先给空态。
  模块边界：数据在 stores（P5.5 引入）。
-->
<script setup lang="ts">
</script>

<template>
  <main class="page__body">
    <section class="card review-placeholder">
      <h1>复习</h1>
      <p class="muted">到期词卡用你自己的句子挖空；两键评分"记住 / 没记住"。</p>
      <el-empty description="复习还没有开放（P5.5 上线）" />
    </section>
  </main>
</template>

<style scoped>
.review-placeholder {
  padding: var(--sp-5);
}
.review-placeholder h1 {
  font-size: 16px;
  margin: 0 0 var(--sp-2);
}
</style>

<!--
  WordsView.vue 是词表页（P0 占位，P5 填充词表闭环）。
  @file web/src/views/WordsView.vue
  文件职责：收藏词列表/删除/排序/复习入口（FR-702~704/1202）；P0 先给空态。
  模块边界：数据在 stores（P5 引入）；本组件只做交互编排。
-->
<script setup lang="ts">
</script>

<template>
  <main class="page__body">
    <section class="card words-placeholder">
      <h1>词表</h1>
      <p class="muted">查词后一键收藏，收藏的词会带着你写的原句进来；复习时挖空的就是这句。</p>
      <el-empty description="词表还没有开放（P5 上线），先去写作台写点什么吧" />
    </section>
  </main>
</template>

<style scoped>
.words-placeholder {
  padding: var(--sp-5);
}
.words-placeholder h1 {
  font-size: 16px;
  margin: 0 0 var(--sp-2);
}
</style>

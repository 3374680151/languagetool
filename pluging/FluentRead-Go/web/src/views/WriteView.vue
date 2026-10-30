<!--
  WriteView.vue 是写作台主页（P0 占位，P1 填充两栏工作台）。
  @file web/src/views/WriteView.vue
  文件职责：左英右中逐段写作与回译（FR-1xx）；P0 先提供空文档态与示例载入
  （FR-111：首次进入不白屏），翻译链路 P1 接通。
  模块边界：业务状态在 stores/workbench.ts；本组件只做交互编排。
-->
<script setup lang="ts">
import { useUiStore } from '../stores/ui';

const ui = useUiStore();

function loadSample() {
  ui.toast('info', '写作台正在建设中（P1 接通翻译链路），示例暂不写入编辑器');
}
</script>

<template>
  <main class="page__body">
    <section class="card write-placeholder">
      <h1>写作台</h1>
      <p class="muted">
        左边写英文，右边即时看到中文回译；写错的单词与语法会被逐条指出并讲清为什么。
      </p>
      <el-empty description="还没有内容">
        <el-button type="primary" data-test="load-sample" @click="loadSample">载入示例</el-button>
      </el-empty>
    </section>
  </main>
</template>

<style scoped>
.write-placeholder {
  padding: var(--sp-5);
}
.write-placeholder h1 {
  font-size: 16px;
  margin: 0 0 var(--sp-2);
}
</style>

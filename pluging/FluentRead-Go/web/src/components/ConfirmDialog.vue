<!--
  ConfirmDialog.vue 是全局确认框（Promise 式，ui store 驱动）。
  @file web/src/components/ConfirmDialog.vue
  文件职责：渲染 ui store 的 confirm 状态；确定/取消 resolve 对应布尔；
  Esc=取消、Enter=确定。用于"手改后重译确认"（FR-109/504）与学习资产清空（FR-1205）。
  模块边界：只读写 ui store；不发起请求。
-->
<script setup lang="ts">
import { onBeforeUnmount, onMounted } from 'vue';
import { useUiStore } from '../stores/ui';

const ui = useUiStore();

function onKeydown(e: KeyboardEvent) {
  if (!ui.confirm.visible) return;
  if (e.key === 'Escape') {
    e.preventDefault();
    ui.resolveConfirm(false);
  } else if (e.key === 'Enter' && !e.shiftKey) {
    e.preventDefault();
    ui.resolveConfirm(true);
  }
}

onMounted(() => window.addEventListener('keydown', onKeydown));
onBeforeUnmount(() => window.removeEventListener('keydown', onKeydown));
</script>

<template>
  <el-dialog
    :model-value="ui.confirm.visible"
    :title="ui.confirm.title"
    width="420px"
    align-center
    append-to-body
    data-test="confirm-dialog"
    @update:model-value="(v: boolean) => { if (!v) ui.resolveConfirm(false); }"
  >
    <p class="confirm-body">{{ ui.confirm.body }}</p>
    <template #footer>
      <el-button data-test="confirm-cancel" @click="ui.resolveConfirm(false)">{{ ui.confirm.cancelText }}</el-button>
      <el-button type="primary" data-test="confirm-ok" @click="ui.resolveConfirm(true)">{{ ui.confirm.confirmText }}</el-button>
    </template>
  </el-dialog>
</template>

<style scoped>
.confirm-body {
  margin: 0;
  color: var(--ink-2);
  line-height: 1.7;
}
</style>

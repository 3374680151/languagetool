<!--
  ToastHost.vue 渲染全局轻提示队列。
  @file web/src/components/ToastHost.vue
  文件职责：把 ui store 的 toasts 渲染成右上角浮动列表；深浅色用 token。
  模块边界：只读 ui store；不发起请求。
-->
<script setup lang="ts">
import { useUiStore } from '../stores/ui';

const ui = useUiStore();
</script>

<template>
  <div class="toasts" aria-live="polite">
    <TransitionGroup name="toast">
      <div v-for="t in ui.toasts" :key="t.id" class="toast" :class="`toast--${t.kind}`" role="status">
        <span>{{ t.text }}</span>
        <button class="toast__close" aria-label="关闭提示" @click="ui.dismiss(t.id)">×</button>
      </div>
    </TransitionGroup>
  </div>
</template>

<style scoped>
.toasts {
  position: fixed;
  top: 60px;
  right: 16px;
  z-index: 3000;
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-width: min(360px, calc(100vw - 32px));
}

.toast {
  display: flex;
  align-items: flex-start;
  gap: 8px;
  padding: 10px 12px;
  border-radius: var(--radius);
  border: 1px solid var(--border);
  background: var(--surface);
  box-shadow: var(--shadow);
  font-size: 13px;
}

.toast--success { border-left: 3px solid var(--ok); }
.toast--warn { border-left: 3px solid var(--warn); }
.toast--error { border-left: 3px solid var(--danger); }
.toast--info { border-left: 3px solid var(--primary); }

.toast__close {
  margin-left: auto;
  border: 0;
  background: none;
  color: var(--ink-3);
  cursor: pointer;
  font-size: 14px;
  line-height: 1;
  padding: 0 2px;
}

.toast-enter-active,
.toast-leave-active {
  transition: all 0.18s ease;
}
.toast-enter-from,
.toast-leave-to {
  opacity: 0;
  transform: translateY(-6px);
}
</style>

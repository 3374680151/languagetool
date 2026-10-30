<!--
  SettingsView.vue 是设置页（P0 占位，P2 填充偏好与状态区块）。
  @file web/src/views/SettingsView.vue
  文件职责：偏好 6 项（FR-1001）与服务状态只读区块（FR-1003）；
  P0 先给结构骨架。
  模块边界：数据在 stores/settings.ts。
-->
<script setup lang="ts">
import { useSettingsStore } from '../stores/settings';

const settings = useSettingsStore();
</script>

<template>
  <main class="page__body">
    <section class="card settings-placeholder">
      <h1>设置</h1>
      <el-form label-width="120px" class="settings-form">
        <el-form-item label="主题">
          <el-radio-group
            :model-value="settings.theme"
            @update:model-value="(v: 'light' | 'dark') => settings.applyTheme(v)"
          >
            <el-radio-button value="light">浅色</el-radio-button>
            <el-radio-button value="dark">深色</el-radio-button>
          </el-radio-group>
        </el-form-item>
      </el-form>
      <p class="muted faint">语言对、风格、模型、查词触发方式与词卡自动朗读在 P2 开放。</p>
    </section>
  </main>
</template>

<style scoped>
.settings-placeholder {
  padding: var(--sp-5);
}
.settings-placeholder h1 {
  font-size: 16px;
  margin: 0 0 var(--sp-3);
}
.settings-form {
  max-width: 480px;
}
</style>

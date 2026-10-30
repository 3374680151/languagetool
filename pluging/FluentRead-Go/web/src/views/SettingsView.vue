<!--
  SettingsView.vue 是设置页（FR-1001 六项偏好 + FR-1003 服务状态 + FR-1004 如实开关）。
  @file web/src/views/SettingsView.vue
  文件职责：语言对/风格/主题/查词触发方式/词卡自动朗读的表单（模型选择随 P3.5
  接入站点池下拉）；只读"服务状态"区块（网关/数据库/版本）；改主题立即生效。
  模块边界：数据在 stores/settings 与 stores/config；无任何密钥输入框（FR-1002）。
-->
<script setup lang="ts">
import { onMounted } from 'vue';
import { useSettingsStore } from '../stores/settings';
import { useConfigStore } from '../stores/config';
import { Languages } from '../lib/lang';

const settings = useSettingsStore();
const config = useConfigStore();

const toOptions = Languages.filter((l) => l.id !== 'auto');
const fromOptions = Languages;
const styleOptions = [
  { value: 'academic', label: '学术' },
  { value: 'formal', label: '正式' },
  { value: 'concise', label: '简洁' },
  { value: 'casual', label: '口语' },
];
const triggerOptions = [
  { value: 'direct', label: '划选即弹' },
  { value: 'icon', label: '浮标确认' },
  { value: 'modifier', label: '修饰键' },
];

onMounted(() => {
  if (!config.loaded) void config.refresh();
});
</script>

<template>
  <main class="page__body settings">
    <section class="card block">
      <h1>偏好</h1>
      <el-form label-width="140px" label-position="left">
        <el-form-item label="语言对">
          <div class="pair">
            <el-select
              :model-value="settings.prefs.langPair.from"
              size="small"
              data-test="pref-from"
              aria-label="源语言"
              @update:model-value="(v: string) => settings.patch({ langPair: { from: v, to: settings.prefs.langPair.to } })"
            >
              <el-option v-for="l in fromOptions" :key="l.id" :value="l.id" :label="l.label" />
            </el-select>
            <span class="faint">→</span>
            <el-select
              :model-value="settings.prefs.langPair.to"
              size="small"
              data-test="pref-to"
              aria-label="目标语言"
              @update:model-value="(v: string) => settings.patch({ langPair: { from: settings.prefs.langPair.from, to: v } })"
            >
              <el-option v-for="l in toOptions" :key="l.id" :value="l.id" :label="l.label" />
            </el-select>
          </div>
        </el-form-item>
        <el-form-item label="译文风格">
          <el-select
            :model-value="settings.prefs.style"
            size="small"
            data-test="pref-style"
            @update:model-value="(v: string) => settings.patch({ style: v })"
          >
            <el-option v-for="s in styleOptions" :key="s.value" :value="s.value" :label="s.label" />
          </el-select>
        </el-form-item>
        <el-form-item label="主题">
          <el-radio-group
            :model-value="settings.prefs.theme"
            data-test="pref-theme"
            @update:model-value="(v: 'light' | 'dark') => settings.applyTheme(v)"
          >
            <el-radio-button value="light">浅色</el-radio-button>
            <el-radio-button value="dark">深色</el-radio-button>
          </el-radio-group>
        </el-form-item>
        <el-form-item label="查词触发方式">
          <el-radio-group
            :model-value="settings.prefs.wordTrigger"
            data-test="pref-trigger"
            @update:model-value="(v: 'direct' | 'icon' | 'modifier') => settings.patch({ wordTrigger: v })"
          >
            <el-radio-button v-for="t in triggerOptions" :key="t.value" :value="t.value">{{ t.label }}</el-radio-button>
          </el-radio-group>
          <div class="hint faint">查词卡随 P4 上线；先把触发习惯存好。</div>
        </el-form-item>
        <el-form-item label="词卡自动朗读">
          <el-switch
            :model-value="settings.prefs.autoSpeak"
            data-test="pref-autospeak"
            @update:model-value="(v: boolean) => settings.patch({ autoSpeak: v })"
          />
        </el-form-item>
      </el-form>
    </section>

    <section class="card block">
      <h1>服务状态</h1>
      <el-descriptions :column="1" border size="small" data-test="service-status">
        <el-descriptions-item label="翻译模型">
          <el-tag v-if="config.gatewayState === 'ok'" type="success" size="small">正常</el-tag>
          <el-tag v-else-if="config.gatewayState === 'degraded'" type="warning" size="small">部分模型不可用</el-tag>
          <el-tag v-else type="info" size="small" data-test="gateway-unconfigured">未配置</el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="数据库">
          <el-tag :type="config.dbState === 'ok' ? 'success' : 'danger'" size="small">
            {{ config.dbState === 'ok' ? '正常' : '不可用' }}
          </el-tag>
        </el-descriptions-item>
        <el-descriptions-item label="模型清单">
          <span v-if="config.models.length === 0" class="faint">站点还没有配置模型</span>
          <span v-else>{{ config.models.map((m) => m.label).join('、') }}</span>
        </el-descriptions-item>
      </el-descriptions>
      <p class="hint faint">密钥只保存在站长侧的环境变量里，本页面不会出现任何密钥输入框。</p>
    </section>
  </main>
</template>

<style scoped>
.settings {
  display: flex;
  flex-direction: column;
  gap: var(--sp-4);
  max-width: 760px;
}

.block {
  padding: var(--sp-4) var(--sp-5);
}

.block h1 {
  font-size: 15px;
  margin: 0 0 var(--sp-3);
}

.pair {
  display: flex;
  align-items: center;
  gap: var(--sp-2);
}

.pair .el-select {
  width: 130px;
}

.hint {
  font-size: 12px;
  margin-top: 4px;
}
</style>

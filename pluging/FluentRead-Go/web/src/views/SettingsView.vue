<!--
  SettingsView.vue 是设置页（FR-1001 六项偏好 + FR-403 自定义模型 + FR-1003 服务状态）。
  @file web/src/views/SettingsView.vue
  文件职责：语言对/风格/主题/查词触发方式/词卡自动朗读的表单；自定义模型表单
  （baseURL/apiKey/model + 测试连接 + "密钥会经本站转发"告知，FR-403/404）；
  只读"服务状态"区块（FR-1003）。模型下拉在写作台顶栏（FR-402）。
  模块边界：数据在 stores/settings 与 stores/config；测试连接走 api/client。
-->
<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useSettingsStore } from '../stores/settings';
import { useConfigStore } from '../stores/config';
import { postModelTest } from '../api/client';
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

// 自定义模型表单（FR-403）：apiKey 只进内存（FR-404）。
const customForm = ref({
  baseURL: settings.customModel.baseURL,
  model: settings.customModel.model,
  apiKey: settings.customModel.apiKey,
});
const testState = ref({ testing: false, result: '' });

/** testCustom 测试连接：服务端转发一次最小调用（FR-403）。 */
async function testCustom() {
  testState.value = { testing: true, result: '' };
  try {
    const r = await postModelTest({
      baseURL: customForm.value.baseURL.trim(),
      apiKey: customForm.value.apiKey.trim(),
      model: customForm.value.model.trim(),
    });
    testState.value.result = r.ok ? `连接成功（${r.latencyMs ?? 0}ms）` : `连接失败：${r.message}`;
  } catch (e) {
    testState.value.result = e instanceof Error ? e.message : '测试失败，请稍后重试';
  } finally {
    testState.value.testing = false;
  }
}

/** saveCustom 保存自定义端点。 */
function saveCustom() {
  settings.saveCustom({
    baseURL: customForm.value.baseURL.trim(),
    model: customForm.value.model.trim(),
    apiKey: customForm.value.apiKey.trim(),
  });
  if (!customForm.value.baseURL.trim() || !customForm.value.model.trim()) {
    testState.value.result = '已清空自定义模型（将使用站点默认模型）';
    return;
  }
  testState.value.result = '已保存。到写作台顶栏选"自定义模型…"即可使用。';
}

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
          <div class="hint faint">划选即弹：选中文字直接查；浮标确认：选中后点小浮标才查；修饰键：按住 Alt 划选才查。</div>
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
      <h1>自定义模型（可选）</h1>
      <p class="hint" style="margin-top:0">
        用自己的模型账号：填写 OpenAI 兼容端点、密钥与模型名。
        <strong>密钥会经本站转发</strong>（浏览器直连会被跨域拦截），用完即弃、不落盘、不写日志。
      </p>
      <el-form label-width="140px" label-position="left">
        <el-form-item label="API 地址">
          <el-input v-model="customForm.baseURL" data-test="custom-base" placeholder="例如 https://api.openai.com（不带 /v1，填了也会自动去掉）" />
        </el-form-item>
        <el-form-item label="API 密钥">
          <el-input
            v-model="customForm.apiKey"
            data-test="custom-key"
            type="password"
            show-password
            placeholder="只在本次会话内存中保存，刷新后需重填"
          />
        </el-form-item>
        <el-form-item label="模型名">
          <el-input v-model="customForm.model" data-test="custom-model" placeholder="例如 deepseek-chat" />
        </el-form-item>
        <el-form-item>
          <el-button :loading="testState.testing" data-test="custom-test" @click="testCustom">测试连接</el-button>
          <el-button type="primary" data-test="custom-save" @click="saveCustom">保存</el-button>
          <span v-if="testState.result" class="hint" data-test="custom-result" style="margin-left:8px">{{ testState.result }}</span>
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

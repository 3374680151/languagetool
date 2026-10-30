<!--
  App.vue 是根组件：顶栏 + 路由视图 + 全局弹层。
  @file web/src/App.vue
  文件职责：页面骨架——TopBar（导航与全局状态）固定在顶部，路由视图填充主体；
  应用启动时初始化主题与服务端偏好同步（FR-1001/1105）；ToastHost 全局单例。
  模块边界：不写业务；各页面自己管数据。
-->
<script setup lang="ts">
import { onMounted } from 'vue';
import { RouterView } from 'vue-router';
import TopBar from './components/TopBar.vue';
import ToastHost from './components/ToastHost.vue';
import ConfirmDialog from './components/ConfirmDialog.vue';
import { useSettingsStore } from './stores/settings';
import { useConfigStore } from './stores/config';

const settings = useSettingsStore();
const config = useConfigStore();

onMounted(() => {
  // 主题先应用避免闪白，服务端偏好异步覆盖；公共配置拉一次。
  settings.init();
  void config.refresh();
});
</script>

<template>
  <div class="page">
    <TopBar />
    <RouterView />
    <ToastHost />
    <ConfirmDialog />
  </div>
</template>

<!--
  App.vue 是根组件：顶栏 + 路由视图 + 全局弹层。
  @file web/src/App.vue
  文件职责：页面骨架——TopBar（导航与全局状态）固定在顶部，路由视图填充主体；
  ToastHost 与 WordPopup 是全局单例（词卡在写作台与词表页共用，§7.8）。
  模块边界：不写业务；各页面自己管数据。
-->
<script setup lang="ts">
import { RouterView } from 'vue-router';
import TopBar from './components/TopBar.vue';
import ToastHost from './components/ToastHost.vue';
</script>

<template>
  <div class="page">
    <TopBar />
    <RouterView />
    <ToastHost />
  </div>
</template>

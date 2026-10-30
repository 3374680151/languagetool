/**
 * main.ts 是前端入口：装配 Element Plus、Pinia、Router 与主题。
 *
 * @file web/src/main.ts
 * 文件职责：创建 Vue 应用，注册 Element Plus（DR-2 唯一组件库）与深色主题
 * 样式，挂载根组件。装配顺序：样式 → 插件 → 挂载。
 *
 * 主要内容：createApp 装配。
 *
 * 模块边界：只做装配；路由表在 router.ts；主题工具在 stores/settings.ts。
 */

import { createApp } from 'vue';
import { createPinia } from 'pinia';
import ElementPlus from 'element-plus';
import 'element-plus/dist/index.css';
import 'element-plus/theme-chalk/dark/css-vars.css';

import App from './App.vue';
import { router } from './router';
import './styles/tokens.css';
import './styles/base.css';

const app = createApp(App);
app.use(createPinia());
app.use(router);
app.use(ElementPlus);
app.mount('#app');

/**
 * router.ts 定义前端路由（§7.8：/ 写作台、/words 词表、/review 复习、/settings 设置）。
 *
 * @file web/src/router.ts
 * 文件职责：路由表与回落；写作台是主页（FR-101：打开就能写，无落地页/登录）。
 * 无导航守卫——没有账号体系（FR-1104 匿名 cookie）。
 *
 * 主要内容：router 实例。
 *
 * 模块边界：只做路径→组件映射；页面组件在 views/。
 */

import { createRouter, createWebHistory } from 'vue-router';
import WriteView from './views/WriteView.vue';

export const router = createRouter({
  history: createWebHistory(),
  routes: [
    { path: '/', name: 'write', component: WriteView },
    { path: '/words', name: 'words', component: () => import('./views/WordsView.vue') },
    { path: '/review', name: 'review', component: () => import('./views/ReviewView.vue') },
    { path: '/settings', name: 'settings', component: () => import('./views/SettingsView.vue') },
    { path: '/:pathMatch(.*)*', redirect: '/' },
  ],
  scrollBehavior() {
    return { top: 0 };
  },
});

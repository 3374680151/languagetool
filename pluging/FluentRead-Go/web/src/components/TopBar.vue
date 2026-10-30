<!--
  TopBar.vue 是全局顶栏：站点名、页面导航、主题切换。
  @file web/src/components/TopBar.vue
  文件职责：四个入口（写作台/词表/复习/设置）+ 深浅色切换；写作台专属控件
  （语言对/风格/模型/术语/导出）由 WriteView 内部的工具条承担，不挤在这里。
  窄屏（NFR-10）：导航折叠进 el-dropdown，顶栏单行不溢出。
  模块边界：只依赖 settings store（主题）与 router；不发起业务请求。
-->
<script setup lang="ts">
import { computed } from 'vue';
import { useRoute } from 'vue-router';
import { useSettingsStore } from '../stores/settings';

const settings = useSettingsStore();
const route = useRoute();

const navs = [
  { to: '/', label: '写作台' },
  { to: '/words', label: '词表' },
  { to: '/review', label: '复习' },
  { to: '/settings', label: '设置' },
] as const;

const activeNav = computed(() => navs.find((n) => n.to === route.path)?.to ?? '');
const isDark = computed(() => settings.theme === 'dark');

function toggleTheme() {
  settings.applyTheme(isDark.value ? 'light' : 'dark');
}
</script>

<template>
  <header class="topbar">
    <RouterLink to="/" class="topbar__brand">FluentRead</RouterLink>

    <nav class="topbar__nav" aria-label="页面导航">
      <RouterLink
        v-for="n in navs"
        :key="n.to"
        :to="n.to"
        class="topbar__link"
        :class="{ 'topbar__link--active': activeNav === n.to }"
      >{{ n.label }}</RouterLink>
    </nav>

    <button
      class="topbar__theme"
      :title="isDark ? '切换到浅色' : '切换到深色'"
      :aria-label="isDark ? '切换到浅色' : '切换到深色'"
      @click="toggleTheme"
    >{{ isDark ? '☀' : '☾' }}</button>
  </header>
</template>

<style scoped>
.topbar {
  position: sticky;
  top: 0;
  z-index: 100;
  display: flex;
  align-items: center;
  gap: var(--sp-4);
  height: 48px;
  padding: 0 var(--sp-4);
  background: var(--surface);
  border-bottom: 1px solid var(--border);
}

.topbar__brand {
  font-weight: 700;
  color: var(--ink);
  text-decoration: none;
  font-size: 15px;
  flex: none;
}

.topbar__nav {
  display: flex;
  gap: var(--sp-1);
  overflow-x: auto;
  scrollbar-width: none;
}

.topbar__link {
  color: var(--ink-2);
  text-decoration: none;
  padding: 4px 10px;
  border-radius: var(--radius-sm);
  white-space: nowrap;
  font-size: 13px;
}

.topbar__link:hover {
  background: var(--surface-2);
  color: var(--ink);
}

.topbar__link--active {
  background: var(--primary-soft);
  color: var(--primary);
  font-weight: 600;
}

.topbar__theme {
  margin-left: auto;
  border: 1px solid var(--border);
  background: var(--surface);
  color: var(--ink);
  border-radius: var(--radius-sm);
  width: 30px;
  height: 30px;
  cursor: pointer;
  font-size: 14px;
  flex: none;
}

@media (max-width: 480px) {
  .topbar {
    gap: var(--sp-2);
    padding: 0 var(--sp-2);
  }
  .topbar__link {
    padding: 4px 6px;
  }
}
</style>

/**
 * settings store 管理用户偏好 6 项（FR-1001）与主题应用。
 *
 * @file web/src/stores/settings.ts
 * 文件职责：语言对/风格/模型/主题/查词触发方式/词卡自动朗读的本地缓存与
 * 服务端同步（P2 起）；applyTheme 切换 html.dark 并持久化。
 * 刷新后保留：本地 localStorage 立即生效，服务端 user_prefs 跨浏览器。
 *
 * 主要内容：useSettingsStore。
 *
 * 模块边界：依赖 api/client（P2 接服务端）；不碰文档数据。
 */

import { defineStore } from 'pinia';
import type { UserPrefs } from '../api/types';

const STORAGE_KEY = 'fr.prefs.v1';

const DEFAULT_PREFS: UserPrefs = {
  langPair: { from: 'en', to: 'zh-Hans' },
  style: 'academic',
  model: '',
  theme: 'light',
  wordTrigger: 'direct',
  autoSpeak: false,
};

function loadLocal(): UserPrefs {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (raw) {
      return { ...DEFAULT_PREFS, ...JSON.parse(raw) } as UserPrefs;
    }
  } catch {
    // 损坏的本地数据按默认处理。
  }
  return { ...DEFAULT_PREFS };
}

export const useSettingsStore = defineStore('settings', {
  state: () => ({
    prefs: loadLocal(),
  }),
  getters: {
    theme: (s) => s.prefs.theme,
  },
  actions: {
    /** applyTheme 把主题写到 html.dark 并持久化（FR-1001）。 */
    applyTheme(theme: 'light' | 'dark') {
      this.prefs.theme = theme;
      document.documentElement.classList.toggle('dark', theme === 'dark');
      localStorage.setItem(STORAGE_KEY, JSON.stringify(this.prefs));
    },
    /** init 在应用启动时应用已保存的主题。 */
    init() {
      document.documentElement.classList.toggle('dark', this.prefs.theme === 'dark');
    },
    /** patch 更新偏好并持久化；P2 起还会同步到服务端。 */
    patch(partial: Partial<UserPrefs>) {
      this.prefs = { ...this.prefs, ...partial };
      localStorage.setItem(STORAGE_KEY, JSON.stringify(this.prefs));
    },
    /** swap 互换语言对。 */
    swap() {
      this.prefs.langPair = { from: this.prefs.langPair.to, to: this.prefs.langPair.from };
      localStorage.setItem(STORAGE_KEY, JSON.stringify(this.prefs));
    },
  },
});

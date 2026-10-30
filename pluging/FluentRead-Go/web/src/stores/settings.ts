/**
 * settings store 管理用户偏好 6 项（FR-1001）与主题应用。
 *
 * @file web/src/stores/settings.ts
 * 文件职责：语言对/风格/模型/主题/查词触发方式/词卡自动朗读——本地 localStorage
 * 立即生效，服务端 user_prefs 同步（FR-1105，刷新后保留、跨浏览器一致）；
 * applyTheme 切换 html.dark；本地为乐观写，服务端失败不阻断使用。
 *
 * 主要内容：useSettingsStore（init/patch/swap/applyTheme、服务端同步）。
 *
 * 模块边界：依赖 api/client；不碰文档数据；模型清单来自 config store。
 */

import { defineStore } from 'pinia';
import { getPrefs, putPrefs } from '../api/client';
import type { UserPrefs } from '../api/types';

const STORAGE_KEY = 'fr.prefs.v1';
/** 自定义端点单独存：baseURL/model 可以落 localStorage，apiKey 绝不落盘（FR-404）。 */
const CUSTOM_KEY = 'fr.custom.v1';

const DEFAULT_PREFS: UserPrefs = {
  langPair: { from: 'en', to: 'zh-Hans' },
  style: 'academic',
  model: '',
  theme: 'light',
  wordTrigger: 'direct',
  autoSpeak: false,
};

export interface CustomModel {
  baseURL: string;
  model: string;
  /** apiKey 只存内存（Pinia），刷新后需重新填写——密钥不落任何盘（FR-404）。 */
  apiKey: string;
}

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

function loadCustom(): CustomModel {
  try {
    const raw = localStorage.getItem(CUSTOM_KEY);
    if (raw) {
      const parsed = JSON.parse(raw) as Partial<CustomModel>;
      return { baseURL: parsed.baseURL ?? '', model: parsed.model ?? '', apiKey: '' };
    }
  } catch {
    // 同上。
  }
  return { baseURL: '', model: '', apiKey: '' };
}

export const useSettingsStore = defineStore('settings', {
  state: () => ({
    prefs: loadLocal(),
    /** customModel 是 L3 访客自定义端点（FR-403）。apiKey 只在内存。 */
    customModel: loadCustom(),
    /** serverSynced 表示服务端偏好已拉取过（拉取后以服务端为准）。 */
    serverSynced: false,
  }),
  getters: {
    theme: (s) => s.prefs.theme,
    /** customReady：自定义模型三要素齐全。 */
    customReady: (s) => !!(s.customModel.baseURL && s.customModel.model && s.customModel.apiKey),
  },
  actions: {
    /** saveCustom 保存自定义端点；apiKey 只进内存，baseURL/model 落 localStorage。 */
    saveCustom(c: Partial<CustomModel>) {
      this.customModel = { ...this.customModel, ...c };
      try {
        localStorage.setItem(CUSTOM_KEY, JSON.stringify({
          baseURL: this.customModel.baseURL,
          model: this.customModel.model,
        }));
      } catch {
        // 忽略存储失败。
      }
    },
    /** applyTheme 把主题写到 html.dark 并持久化（FR-1001）。 */
    applyTheme(theme: 'light' | 'dark') {
      this.patch({ theme });
      document.documentElement.classList.toggle('dark', theme === 'dark');
    },
    /** init 应用启动：先应用本地主题，再拉服务端偏好覆盖（FR-1105）。 */
    async init() {
      document.documentElement.classList.toggle('dark', this.prefs.theme === 'dark');
      try {
        const server = await getPrefs();
        // 服务端已保存过的键才覆盖本地（首次访客保留本地默认）。
        const merged: UserPrefs = { ...this.prefs };
        if (server.langPair?.from) merged.langPair = server.langPair;
        if (server.style) merged.style = server.style;
        if (server.model) merged.model = server.model;
        if (server.theme === 'light' || server.theme === 'dark') merged.theme = server.theme;
        if (server.wordTrigger) merged.wordTrigger = server.wordTrigger;
        merged.autoSpeak = !!server.autoSpeak;
        this.prefs = merged;
        localStorage.setItem(STORAGE_KEY, JSON.stringify(this.prefs));
        document.documentElement.classList.toggle('dark', this.prefs.theme === 'dark');
      } catch {
        // 服务端不可用：本地偏好照用。
      } finally {
        this.serverSynced = true;
      }
    },
    /** patch 更新偏好：本地立即生效 + 防抖同步服务端。 */
    patch(partial: Partial<UserPrefs>) {
      this.prefs = { ...this.prefs, ...partial };
      localStorage.setItem(STORAGE_KEY, JSON.stringify(this.prefs));
      void this.pushToServer();
    },
    /** swap 互换语言对。 */
    swap() {
      this.patch({ langPair: { from: this.prefs.langPair.to, to: this.prefs.langPair.from } });
    },
    /** pushToServer 把当前偏好推到服务端（PUT 支持部分更新，这里全量推）。 */
    async pushToServer() {
      try {
        await putPrefs(this.prefs);
      } catch {
        // 推送失败不阻断：下次改动会再推。
      }
    },
  },
});

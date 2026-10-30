/**
 * config store 缓存 /api/config 的公共配置。
 *
 * @file web/src/stores/config.ts
 * 文件职责：语言表/风格表/模型池/能力开关/健康摘要（FR-1003/1004）；
 * 应用启动时拉一次；设置页"服务状态"区块与写作台"未配置"态都读这里。
 *
 * 主要内容：useConfigStore。
 *
 * 模块边界：依赖 api/client；不发起翻译。
 */

import { defineStore } from 'pinia';
import { getConfig } from '../api/client';
import type { PublicConfig } from '../api/types';

export const useConfigStore = defineStore('config', {
  state: () => ({
    data: null as PublicConfig | null,
    loaded: false,
    failed: false,
  }),
  getters: {
    /** unconfigured：站点没配模型（六态之"未配置"，页面照常可用）。 */
    unconfigured: (s) => !!s.loaded && !s.failed && !s.data?.features.translate,
    /** gatewayState 供设置页展示。 */
    gatewayState: (s) => s.data?.health.gateway ?? 'fail',
    dbState: (s) => s.data?.health.db ?? 'fail',
    models: (s) => s.data?.models ?? [],
  },
  actions: {
    async refresh() {
      this.failed = false;
      try {
        this.data = await getConfig();
        this.loaded = true;
      } catch {
        this.failed = true;
      }
    },
  },
});

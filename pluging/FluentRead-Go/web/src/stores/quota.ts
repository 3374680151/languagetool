/**
 * quota store 管理访客额度展示（FR-606）。
 *
 * @file web/src/stores/quota.ts
 * 文件职责：拉取 /api/quota 并在本地同步扣减；只在"快用完"时给出提示——
 * 阈值 = max(10, 日上限 10%)，额度充足时界面不出现任何额度文字。
 *
 * 主要内容：useQuotaStore。
 *
 * 模块边界：依赖 api/client；不拦截请求（429 由 workbench 落到段状态）。
 */

import { defineStore } from 'pinia';
import { getQuota } from '../api/client';

export const useQuotaStore = defineStore('quota', {
  state: () => ({
    used: 0,
    limit: 0,
    remaining: 0,
    resetsAt: '',
    unlimited: false,
    loaded: false,
  }),
  getters: {
    /** lowQuota 决定是否展示提示（FR-606：快用完才提示）。 */
    lowQuota: (s) => {
      if (s.unlimited || !s.loaded) return false;
      const threshold = Math.max(10, Math.round(s.limit * 0.1));
      return s.remaining <= threshold;
    },
    /** exhausted 用于限额态横幅。 */
    exhausted: (s) => !s.unlimited && s.loaded && s.remaining <= 0,
  },
  actions: {
    /** refresh 从服务端拉一次额度。 */
    async refresh() {
      try {
        const q = await getQuota();
        this.used = q.used;
        this.limit = q.limit;
        this.remaining = q.remaining;
        this.resetsAt = q.resetsAt;
        this.unlimited = q.unlimited;
        this.loaded = true;
      } catch {
        // 拉不到就不展示（不阻断写作）。
      }
    },
    /** consume 翻译成功后本地扣减（避免每次都打接口）。 */
    consume(segments: number) {
      if (this.unlimited) return;
      this.used += segments;
      this.remaining = Math.max(0, this.remaining - segments);
    },
    /** resetAtText 把恢复时间说成人话（FR-605）。 */
    resetAtText(): string {
      if (!this.resetsAt) return '明日 0 点';
      const t = new Date(this.resetsAt);
      const hours = Math.max(1, Math.round((t.getTime() - Date.now()) / 3600_000));
      return `约 ${hours} 小时后（明日 0 点）`;
    },
  },
});

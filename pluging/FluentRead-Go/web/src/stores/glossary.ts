/**
 * glossary store 管理术语（FR-501~505，P4）。
 *
 * @file web/src/stores/glossary.ts
 * 文件职责：拉取合并视图（站点 > 用户）与指纹；增删改后刷新；
 * 指纹对比是"译文已过期"横幅的依据（FR-503，写作台持有上次翻译的指纹）。
 *
 * 主要内容：useGlossaryStore。
 *
 * 模块边界：依赖 api/client；不碰翻译状态。
 */

import { defineStore } from 'pinia';
import { createGlossary, deleteGlossary, getGlossary, updateGlossary } from '../api/client';
import type { GlossaryItem } from '../api/types';

export const useGlossaryStore = defineStore('glossary', {
  state: () => ({
    items: [] as GlossaryItem[],
    fingerprint: '',
    loaded: false,
  }),
  actions: {
    async refresh() {
      try {
        const v = await getGlossary();
        this.items = v.items;
        this.fingerprint = v.fingerprint;
        this.loaded = true;
      } catch {
        // 拉不到时保持现状（术语是增强功能，不阻断写作）。
      }
    },
    async add(scope: 'site' | 'user', sourceTerm: string, targetTerm: string, note: string) {
      await createGlossary({ scope, sourceTerm, targetTerm, note, enabled: true } as never);
      await this.refresh();
    },
    async update(item: GlossaryItem) {
      await updateGlossary(item as never);
      await this.refresh();
    },
    async remove(id: number) {
      await deleteGlossary(id);
      await this.refresh();
    },
  },
});

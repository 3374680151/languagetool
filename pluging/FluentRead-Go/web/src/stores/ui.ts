/**
 * ui store 管理全局 UI 状态：轻提示队列与 Promise 式确认框。
 *
 * @file web/src/stores/ui.ts
 * 文件职责：跨页面共享的 toast 与 confirm；确认框返回 Promise<boolean>，
 * 供"手改后重译需确认"（FR-109）这类流程 await。
 *
 * 主要内容：useUiStore（toasts、confirmState、toast()、confirm()、resolveConfirm()）。
 *
 * 模块边界：纯 UI 状态；不持有业务数据；文案由调用方传入（NFR-11 说人话）。
 */

import { defineStore } from 'pinia';

export interface Toast {
  id: number;
  kind: 'info' | 'success' | 'warn' | 'error';
  text: string;
}

interface ConfirmState {
  visible: boolean;
  title: string;
  body: string;
  confirmText: string;
  cancelText: string;
  resolve: ((v: boolean) => void) | null;
}

let toastSeq = 0;

export const useUiStore = defineStore('ui', {
  state: () => ({
    toasts: [] as Toast[],
    confirm: {
      visible: false,
      title: '',
      body: '',
      confirmText: '确定',
      cancelText: '取消',
      resolve: null,
    } as ConfirmState,
  }),
  actions: {
    /** toast 弹一条轻提示，默认 3.2 秒自动消失。 */
    toast(kind: Toast['kind'], text: string, duration = 3200) {
      const id = ++toastSeq;
      this.toasts.push({ id, kind, text });
      if (this.toasts.length > 4) {
        this.toasts.shift();
      }
      window.setTimeout(() => this.dismiss(id), duration);
    },
    dismiss(id: number) {
      this.toasts = this.toasts.filter((t) => t.id !== id);
    },
    /** confirm 弹确认框，resolve 为 true/false。 */
    askConfirm(title: string, body: string, confirmText = '确定'): Promise<boolean> {
      return new Promise((resolve) => {
        this.confirm = {
          visible: true,
          title,
          body,
          confirmText,
          cancelText: '取消',
          resolve,
        };
      });
    },
    resolveConfirm(v: boolean) {
      this.confirm.resolve?.(v);
      this.confirm.resolve = null;
      this.confirm.visible = false;
    },
  },
});

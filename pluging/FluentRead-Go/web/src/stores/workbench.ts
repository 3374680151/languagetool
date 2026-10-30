/**
 * workbench store 是写作台的核心状态机（FR-1xx）。
 *
 * @file web/src/stores/workbench.ts
 * 文件职责：段落列表（源文/译文/状态/手改标记）、分段与粘贴拆段（FR-104/105）、
 * 全译与单段译（Ctrl/⌘+Enter，FR-106）、单段失败不影响其他段（FR-303/107）、
 * 手改后重译需确认（FR-109）、草稿持久化（FR-110，localStorage 防抖 500ms）、
 * 术语过期指纹（FR-503，P4 接入）。
 *
 * 主要内容：Segment 类型、useWorkbenchStore。
 *
 * 模块边界：依赖 api/client（唯一网络出口）与 lib/text；不碰 DOM；
 * 确认框通过 ui store（组件层 await）。
 */

import { defineStore } from 'pinia';
import { postTranslate } from '../api/client';
import { ApiError, NetworkError } from '../api/client';
import type { SegmentResult } from '../api/types';
import { isBlank, splitParagraphs } from '../lib/text';

const DRAFT_KEY = 'fr.doc.v1';
const DRAFT_DEBOUNCE_MS = 500;
/** 单次请求最多 20 段（与后端 MaxSegments 一致，FR-305）。 */
const BATCH_SIZE = 20;

/** 段落状态机（FR-107）：idle 待译 / pending 翻译中 / done 完成 / error 失败。 */
export type SegmentStatus = 'idle' | 'pending' | 'done' | 'error';

export interface Segment {
  id: number;
  source: string;
  /** target 是右侧中文回译；可直接编辑（FR-108）。 */
  target: string;
  status: SegmentStatus;
  /** edited 为 true 表示译文被手动改过，重译前需确认（FR-109）。 */
  edited: boolean;
  /** lastSource 是上次成功翻译时的源文，用于判断"源文改过需要重译"。 */
  lastSource: string;
  error?: { code: string; message: string };
  /** ms 是该段真实耗时，展示用。 */
  ms?: number;
  model?: string;
  /** degraded 该段走了免费方式（FR-306）。 */
  degraded?: boolean;
}

interface DraftPayload {
  v: 1;
  segments: Pick<Segment, 'source' | 'target' | 'edited' | 'lastSource'>[];
  savedAt: number;
}

let nextId = 1;
const newId = () => nextId++;

export const useWorkbenchStore = defineStore('workbench', {
  state: () => ({
    segments: [] as Segment[],
    translating: false,
    /** waitedSeconds 翻译中已等待秒数（六态之"超时"展示）。 */
    waitedSeconds: 0,
    /** degradedReason 全局降级原因（FR-306），空串 = 未降级。 */
    degradedReason: '',
    /** unconfigured 站点未配置模型（六态之"未配置"）。 */
    unconfigured: false,
    /** glossaryFingerprint 是上次全译成功时的术语指纹（FR-503，P4 接入）。 */
    glossaryFingerprint: '',
    /** pendingFocus 拆段后组件应聚焦的段与光标位（Enter×2 流畅开新段）。 */
    pendingFocus: null as { id: number; offset: number } | null,
    _saveTimer: 0 as unknown as ReturnType<typeof setTimeout> | 0,
    _abort: null as AbortController | null,
    _waitTimer: 0 as unknown as ReturnType<typeof setInterval> | 0,
  }),

  getters: {
    stats: (s) => {
      const done = s.segments.filter((x) => x.status === 'done').length;
      const failed = s.segments.filter((x) => x.status === 'error').length;
      const edited = s.segments.filter((x) => x.edited).length;
      const chars = s.segments.reduce((n, x) => n + [...x.source].length, 0);
      return { total: s.segments.length, done, failed, edited, chars };
    },
    hasTranslated: (s) => s.segments.some((x) => x.status === 'done' || x.status === 'error'),
  },

  actions: {
    // ------------------------------------------------------------------
    // 文档结构（FR-104/105）
    // ------------------------------------------------------------------

    /** setDocument 用整篇文本重建段落（粘贴自动切开，空行不开空框）。 */
    setDocument(text: string) {
      const parts = splitParagraphs(text);
      this.segments = parts.map((source) => this.makeSegment(source));
      this.scheduleSave();
    },

    /** replaceAll 直接替换段落内容（草稿恢复用，保留 target/edited）。 */
    replaceAll(list: Segment[]) {
      this.segments = list;
      this.scheduleSave();
    },

    /** updateSource 更新单段源文（FR-104：手动 Enter×2 的拆段由组件调 splitSegment）。 */
    updateSource(id: number, source: string) {
      const seg = this.segments.find((s) => s.id === id);
      if (!seg) return;
      seg.source = source;
      this.scheduleSave();
    },

    /** updateTarget 更新译文并标记手改（FR-108/109：点击即可改，无弹窗）。 */
    updateTarget(id: number, target: string) {
      const seg = this.segments.find((s) => s.id === id);
      if (!seg) return;
      seg.target = target;
      seg.edited = true;
      this.scheduleSave();
    },

    /** splitSegment 在段内指定偏移处拆段：空行边界即段落边界（FR-104/105）。
     * 手动 Enter×2（末尾新段）与"粘贴整篇"都走这里；空白侧生成空段占位，
     * 便于用户继续输入；粘贴产生的空段在切分时被吸收（不开空框）。 */
    splitSegment(id: number, offset: number) {
      const idx = this.segments.findIndex((s) => s.id === id);
      if (idx < 0) return null;
      const seg = this.segments[idx];
      const before = seg.source.slice(0, offset).replace(/\s+$/, '');
      const after = seg.source.slice(offset).replace(/^\s+/, '');

      if (isBlank(before) && isBlank(after)) {
        seg.source = '';
        return null;
      }
      if (isBlank(before)) {
        // 边界在段首：本段变为 after，前面插一个空段占位。
        seg.source = after;
        seg.lastSource = '';
        seg.status = 'idle';
        const created = this.makeSegment('');
        this.segments.splice(idx, 0, created);
        this.scheduleSave();
        return { id: seg.id, offset: 0 };
      }
      // 常规拆分：before 留在本段，after 建新段（after 可为空 = Enter×2 开新段）。
      seg.source = before;
      seg.lastSource = '';
      seg.status = 'idle';
      const created = this.makeSegment(after);
      this.segments.splice(idx + 1, 0, created);
      this.scheduleSave();
      return { id: created.id, offset: 0 };
    },

    /** normalizeBlanks 把段内所有空行边界递归拆开（粘贴整篇 / Enter×2 共用，FR-104/105）。
     * caret 是拆分时的光标位置：焦点跟随光标最终落入的段（Enter×2 后直接打字）。 */
    normalizeBlanks(id: number, caret?: number): void {
      const originalCaret = caret ?? Number.MAX_SAFE_INTEGER;
      let currentId = id;
      let consumed = 0;
      let focusId = id;
      let focusOffset = originalCaret;
      for (let guard = 0; guard < 200; guard++) {
        const seg = this.segments.find((s) => s.id === currentId);
        if (!seg) break;
        const m = seg.source.match(/\n[ \t]*\n/);
        if (!m || m.index === undefined) break;
        const boundary = m.index + m[0].length;
        const res = this.splitSegment(seg.id, boundary);
        if (!res) break;
        consumed += boundary;
        currentId = res.id;
        if (originalCaret >= consumed) {
          focusId = res.id;
          focusOffset = originalCaret - consumed;
        } else {
          break; // 光标在前面已拆出的段里：焦点留原地。
        }
      }
      const target = this.segments.find((s) => s.id === focusId);
      this.pendingFocus = {
        id: focusId,
        offset: Math.max(0, Math.min(focusOffset, target ? target.source.length : 0)),
      };
    },

    /** insertAfter 在指定段后插入一个空段。 */
    insertAfter(id: number) {
      const idx = this.segments.findIndex((s) => s.id === id);
      const created = this.makeSegment('');
      if (idx < 0) {
        this.segments.push(created);
      } else {
        this.segments.splice(idx + 1, 0, created);
      }
      this.scheduleSave();
      return created.id;
    },

    /** remove 删除一段；最后一段删空时给一个空段占位，保证页面不空白。 */
    remove(id: number) {
      const idx = this.segments.findIndex((s) => s.id === id);
      if (idx < 0) return;
      this.segments.splice(idx, 1);
      if (this.segments.length === 0) {
        this.segments = [this.makeSegment('')];
      }
      this.scheduleSave();
    },

    /** clearAll 清空整篇（导出后的新开始；带确认由组件处理）。 */
    clearAll() {
      this.segments = [this.makeSegment('')];
      this.scheduleSave();
    },

    makeSegment(source: string, extra?: Partial<Segment>): Segment {
      return {
        id: newId(),
        source,
        target: '',
        status: 'idle',
        edited: false,
        lastSource: '',
        ...extra,
      };
    },

    // ------------------------------------------------------------------
    // 翻译（FR-106/107/303）
    // ------------------------------------------------------------------

    /** pendingTargets 收集当前需要翻译的段：非空且（未译过或源文已改或已失败）。 */
    pendingTargets(): Segment[] {
      return this.segments.filter(
        (s) => !isBlank(s.source) && (s.status === 'idle' || s.status === 'error' || s.source !== s.lastSource),
      );
    },

    /** translateAll 全译（Ctrl/⌘+Enter）：分批 ≤20 段，逐段独立（FR-106/107）。 */
    async translateAll(opts?: { force?: boolean }): Promise<void> {
      if (this.translating) return;
      let targets = this.pendingTargets();
      if (opts?.force) {
        targets = this.segments.filter((s) => !isBlank(s.source));
      }
      if (targets.length === 0) return;

      this.translating = true;
      this.degradedReason = '';
      this._startWaitTimer();
      this._abort = new AbortController();
      try {
        for (let i = 0; i < targets.length; i += BATCH_SIZE) {
          if (this._abort.signal.aborted) break;
          const batch = targets.slice(i, i + BATCH_SIZE);
          await this.translateBatch(batch);
        }
      } finally {
        this.translating = false;
        this._stopWaitTimer();
        this._abort = null;
      }
    },

    /** translateOne 单段翻译（Ctrl/⌘+Shift+Enter 只译光标段，FR-106）。 */
    async translateOne(id: number): Promise<void> {
      const seg = this.segments.find((s) => s.id === id);
      if (!seg || isBlank(seg.source) || this.translating) return;
      this.translating = true;
      this._startWaitTimer();
      this._abort = new AbortController();
      try {
        await this.translateBatch([seg]);
      } finally {
        this.translating = false;
        this._stopWaitTimer();
        this._abort = null;
      }
    },

    /** cancel 取消进行中的翻译（六态之"超时"的"取消"按钮）。 */
    cancel() {
      this._abort?.abort();
    },

    /** translateBatch 翻译一批段落：请求前先置 pending，逐段按结果落状态。 */
    async translateBatch(batch: Segment[]): Promise<void> {
      for (const seg of batch) {
        seg.status = 'pending';
        seg.error = undefined;
      }
      try {
        const { useSettingsStore } = await import('./settings');
        const settings = useSettingsStore();
        const resp = await postTranslate(
          {
            segments: batch.map((s) => ({ text: s.source })),
            from: settings.prefs.langPair.from,
            to: settings.prefs.langPair.to,
            style: settings.prefs.style,
            model: settings.prefs.model || undefined,
          },
          { signal: this._abort?.signal },
        );
        this.applyResults(batch, resp.segments);
        if (resp.degradedReason) {
          this.degradedReason = resp.degradedReason;
        }
      } catch (e) {
        this.applyBatchError(batch, e);
      }
    },

    /** applyResults 把响应结果逐段写回状态机（FR-107）。 */
    applyResults(batch: Segment[], results: SegmentResult[]) {
      batch.forEach((seg, i) => {
        const r = results[i];
        if (!r) return;
        if (r.status === 'ok') {
          seg.status = 'done';
          seg.target = r.text ?? '';
          seg.lastSource = seg.source;
          seg.edited = false;
          seg.ms = r.ms;
          seg.model = r.model;
          seg.degraded = r.degraded;
          seg.error = undefined;
        } else if (r.status === 'skipped') {
          // 空白段：不算错误，回到待译（正常流程不该出现——前端只发非空段）。
          seg.status = 'idle';
        } else {
          seg.status = 'error';
          seg.error = { code: r.code ?? 'INTERNAL', message: r.message ?? '翻译失败' };
          if (r.code === 'MODEL_NOT_CONFIGURED') {
            this.unconfigured = true;
          }
        }
      });
    },

    /** applyBatchError 整批请求级失败（限流/断网/取消）：逐段落状态。 */
    applyBatchError(batch: Segment[], e: unknown) {
      if (e instanceof NetworkError && e.aborted) {
        // 用户主动取消：回到待译，不算失败。
        for (const seg of batch) {
          if (seg.status === 'pending') seg.status = seg.lastSource ? 'done' : 'idle';
        }
        return;
      }
      let code = 'INTERNAL';
      let message = '服务出错了，请稍后重试';
      let retryAfter = 0;
      if (e instanceof ApiError) {
        code = e.code;
        message = e.message;
        retryAfter = e.retryAfter;
      } else if (e instanceof NetworkError) {
        code = 'NETWORK';
        message = e.message;
      }
      for (const seg of batch) {
        if (seg.status === 'pending') {
          seg.status = 'error';
          seg.error = { code, message: retryAfter > 0 && code === 'RATE_LIMITED' ? `${message}（已按等待时间自动避开）` : message };
        }
      }
    },

    _startWaitTimer() {
      this.waitedSeconds = 0;
      this._stopWaitTimer();
      this._waitTimer = setInterval(() => {
        this.waitedSeconds += 1;
      }, 1000);
    },
    _stopWaitTimer() {
      if (this._waitTimer) {
        clearInterval(this._waitTimer);
        this._waitTimer = 0;
      }
    },

    // ------------------------------------------------------------------
    // 草稿持久化（FR-110：刷新不丢稿）
    // ------------------------------------------------------------------

    /** restoreDraft 启动时恢复草稿。恢复的 pending 一律回退为待译或完成。 */
    restoreDraft(): boolean {
      try {
        const raw = localStorage.getItem(DRAFT_KEY);
        if (!raw) return false;
        const draft = JSON.parse(raw) as DraftPayload;
        if (!draft?.segments?.length) return false;
        this.segments = draft.segments.map((s) => ({
          ...this.makeSegment(s.source ?? ''),
          target: s.target ?? '',
          edited: !!s.edited,
          lastSource: s.lastSource ?? '',
          status: (s.target && s.lastSource === (s.source ?? '') ? 'done' : 'idle') as SegmentStatus,
        }));
        return true;
      } catch {
        return false;
      }
    },

    /** scheduleSave 防抖 500ms 持久化（§7.8 草稿）。 */
    scheduleSave() {
      if (this._saveTimer) {
        clearTimeout(this._saveTimer);
      }
      this._saveTimer = setTimeout(() => this.saveNow(), DRAFT_DEBOUNCE_MS);
    },

    /** saveNow 立即落盘（pagehide 时调用，FR-110）。 */
    saveNow() {
      try {
        const payload: DraftPayload = {
          v: 1,
          segments: this.segments.map((s) => ({
            source: s.source,
            target: s.target,
            edited: s.edited,
            lastSource: s.lastSource,
          })),
          savedAt: Date.now(),
        };
        localStorage.setItem(DRAFT_KEY, JSON.stringify(payload));
      } catch {
        // 存储满（约 1MB 上限）时静默放弃：草稿是尽力而为的兜底。
      }
    },
  },
});

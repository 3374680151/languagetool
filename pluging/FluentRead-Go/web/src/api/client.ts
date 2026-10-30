/**
 * client.ts 是前端唯一的 HTTP 出口（§7.8）。
 *
 * @file web/src/api/client.ts
 * 文件职责：封装 fetch——统一响应包解包、错误码归一为 ApiError、
 * 429 读取 Retry-After 头、超时与取消（AbortController，六态之"超时"）。
 * 所有 API 调用函数集中在这里，组件不直接 fetch。
 *
 * 主要内容：ApiError、get/post/put/del 基础方法、各端点调用函数。
 *
 * 模块边界：只依赖 types.ts；不持有业务状态；不打本站以外的地址（SDK 除外）。
 */

import type {
  ApiErrorPayload,
  DictEntry,
  ErrEnvelope,
  GlossaryItem,
  GlossaryView,
  Mistake,
  OkEnvelope,
  ProofreadResponse,
  PublicConfig,
  Quota,
  ReviewItem,
  TranslateRequest,
  TranslateResponse,
  UserPrefs,
  Word,
} from './types';

/** ApiError 是归一后的接口错误：message 已经是人话（附录 B）。 */
export class ApiError extends Error {
  readonly code: string;
  readonly status: number;
  readonly retryable: boolean;
  readonly retryAfter: number;

  constructor(payload: ApiErrorPayload, status: number) {
    super(payload.message);
    this.name = 'ApiError';
    this.code = payload.code;
    this.status = status;
    this.retryable = payload.retryable;
    this.retryAfter = payload.retryAfter ?? 0;
  }
}

/** 断网/请求中途被取消等非接口错误。 */
export class NetworkError extends Error {
  readonly aborted: boolean;

  constructor(aborted: boolean) {
    super(aborted ? '请求已取消' : '网络连接失败，请检查网络后重试');
    this.name = 'NetworkError';
    this.aborted = aborted;
  }
}

interface RequestOptions {
  signal?: AbortSignal;
}

/** 基础请求：解统一响应包，失败抛 ApiError。 */
async function request<T>(method: string, path: string, body?: unknown, opts?: RequestOptions): Promise<T> {
  let res: Response;
  try {
    res = await fetch(path, {
      method,
      headers: body !== undefined ? { 'content-type': 'application/json' } : undefined,
      body: body !== undefined ? JSON.stringify(body) : undefined,
      credentials: 'same-origin',
      signal: opts?.signal,
    });
  } catch (e) {
    throw new NetworkError(e instanceof DOMException && e.name === 'AbortError');
  }

  let payload: OkEnvelope<T> | ErrEnvelope;
  try {
    payload = await res.json();
  } catch {
    // 非 JSON（例如网关层错误页）：按不可用处理。
    throw new ApiError(
      { code: 'UPSTREAM_UNAVAILABLE', message: '服务暂时不可用，请稍后重试', retryable: true },
      res.status,
    );
  }

  if (!payload.ok) {
    throw new ApiError(payload.error, res.status);
  }
  return payload.data;
}

// ---------------------------------------------------------------------------
// 各端点调用函数（§7.4 端点表）。分期落地时同步补齐。
// ---------------------------------------------------------------------------

/** GET /api/health（FR-1003 三态）。 */
export function getHealth(opts?: RequestOptions) {
  return request<{ ok: boolean; db: string; gateway: string; version: string }>('GET', '/api/health', undefined, opts);
}

/** POST /api/translate（P1）。 */
export function postTranslate(req: TranslateRequest, opts?: RequestOptions) {
  return request<TranslateResponse>('POST', '/api/translate', req, opts);
}

/** GET /api/quota（P1.5）。 */
export function getQuota(opts?: RequestOptions) {
  return request<Quota>('GET', '/api/quota', undefined, opts);
}

/** GET /api/config（P2）。 */
export function getConfig(opts?: RequestOptions) {
  return request<PublicConfig>('GET', '/api/config', undefined, opts);
}

/** GET /api/user/prefs（P2）。 */
export function getPrefs(opts?: RequestOptions) {
  return request<UserPrefs>('GET', '/api/user/prefs', undefined, opts);
}

/** PUT /api/user/prefs（P2）。 */
export function putPrefs(prefs: UserPrefs, opts?: RequestOptions) {
  return request<UserPrefs>('PUT', '/api/user/prefs', prefs, opts);
}

/** POST /api/proofread（P3）。 */
export function postProofread(text: string, lang: string, opts?: RequestOptions) {
  return request<ProofreadResponse>('POST', '/api/proofread', { text, lang }, opts);
}

/** POST /api/models/test（P3.5）。 */
export function postModelTest(p: { baseURL: string; apiKey: string; model: string }) {
  return request<{ ok: boolean; message: string; latencyMs?: number }>('POST', '/api/models/test', p);
}

/** 术语 CRUD（P4）。 */
export function getGlossary() {
  return request<GlossaryView>('GET', '/api/glossary');
}
export function createGlossary(item: Pick<GlossaryItem, 'sourceTerm' | 'targetTerm' | 'note' | 'scope'>) {
  return request<GlossaryItem>('POST', '/api/glossary', item);
}
export function updateGlossary(item: Pick<GlossaryItem, 'id' | 'sourceTerm' | 'targetTerm' | 'note' | 'scope' | 'enabled'>) {
  return request<GlossaryItem>('PUT', '/api/glossary', item);
}
export function deleteGlossary(id: number) {
  return request<{ deleted: boolean }>('DELETE', `/api/glossary?id=${id}`);
}

/** 查词（P4，§八）。 */
export function getDict(word: string, opts?: RequestOptions) {
  return request<DictEntry>('GET', `/api/dict/${encodeURIComponent(word)}`, undefined, opts);
}

/** 词表（P5）。 */
export function getWords(sort: 'recent' | 'least_used' = 'recent') {
  return request<Word[]>('GET', `/api/words?sort=${sort}`);
}
export function collectWord(p: { word: string; lang?: string; contextSentence?: string; sourceUrl?: string }) {
  return request<Word>('POST', '/api/words', p);
}
export function deleteWord(id: number) {
  return request<{ deleted: boolean }>('DELETE', `/api/words?id=${id}`);
}

/** 错误本与复习（P5.5）。 */
export function postMistake(p: { sourceSnippet: string; correctedSnippet: string; category: string; reason: string }) {
  return request<Mistake>('POST', '/api/mistakes', p);
}
export function getMistakes(category?: string) {
  const q = category ? `?category=${encodeURIComponent(category)}` : '';
  return request<Mistake[]>('GET', `/api/mistakes${q}`);
}
export function deleteMistake(id: number) {
  return request<{ deleted: boolean }>('DELETE', `/api/mistakes?id=${id}`);
}
export function clearMistakes() {
  return request<{ cleared: boolean }>('DELETE', '/api/mistakes?all=1');
}
export function getReviewQueue() {
  return request<{ items: ReviewItem[]; count: number }>('GET', '/api/review');
}
export function postReviewScore(p: { kind: 'word' | 'mistake'; id: number; remembered: boolean }) {
  return request<{ nextDueAt: string }>('POST', '/api/review/score', p);
}

/** 学习资产导出/清空（FR-1205）。 */
export function exportAssets() {
  return request<{ words: Word[]; mistakes: Mistake[]; exportedAt: string }>('GET', '/api/assets/export');
}
export function clearAssets() {
  return request<{ cleared: boolean }>('POST', '/api/assets/clear');
}

/**
 * types.ts 定义前后端共享的 DTO 类型（§7.4 API 契约）。
 *
 * @file web/src/api/types.ts
 * 文件职责：与 Go handler 的 JSON 结构一一对应；统一响应包、错误码、
 * 各端点的请求/响应形状。前端不做后端没有的能力承诺（FR-1004）。
 *
 * 主要内容：ApiEnvelope、ApiErrorPayload、ErrorCode、Config/Model/Quota DTO、
 * Translate 系列、Proofread 系列、Glossary/Words/Mistakes/Review/Prefs/Dict 系列。
 *
 * 模块边界：纯类型；被 api/client.ts 与 stores 使用；不包含任何运行时逻辑。
 */

/** 统一响应包：成功 {@link OkEnvelope} / 失败 {@link ErrEnvelope}（NFR-12）。 */
export interface OkEnvelope<T> {
  ok: true;
  data: T;
}

export interface ErrEnvelope {
  ok: false;
  error: ApiErrorPayload;
}

/** 错误信封的 error 字段（§7.4）。 */
export interface ApiErrorPayload {
  code: ErrorCode;
  message: string;
  retryable: boolean;
  retryAfter?: number;
}

/** 错误码全集（附录 B）。 */
export type ErrorCode =
  | 'UNSUPPORTED_LANGUAGE'
  | 'TEXT_TOO_LONG'
  | 'TOO_MANY_SEGMENTS'
  | 'INVALID_REQUEST'
  | 'QUOTA_EXCEEDED'
  | 'RATE_LIMITED'
  | 'UPSTREAM_TIMEOUT'
  | 'UPSTREAM_UNAVAILABLE'
  | 'MODEL_NOT_CONFIGURED'
  | 'CUSTOM_MODEL_FAILED'
  | 'ORIGIN_FORBIDDEN'
  | 'INTERNAL';

/** 语言项。 */
export interface Language {
  id: string;
  label: string;
}

/** 模型项（FR-402：带质量/速度/成本标签与健康态）。 */
export interface Model {
  id: string;
  label: string;
  role: string;
  quality: string;
  speed: string;
  cost: string;
  enabled: boolean;
  health: 'ok' | 'degraded' | 'removed';
}

/** /api/config 的公共配置（无任何密钥，FR-1002）。 */
export interface PublicConfig {
  languages: Language[];
  styles: string[];
  defaultLangPair: { from: string; to: string };
  defaultStyle: string;
  models: Model[];
  defaultModel: string;
  features: {
    translate: boolean;
    proofread: boolean;
    dictionary: boolean;
    glossary: boolean;
    sdk: boolean;
  };
  health: { gateway: 'ok' | 'degraded' | 'fail'; db: 'ok' | 'fail' };
}

/** /api/quota 的访客额度（FR-606：只在快用完时展示）。 */
export interface Quota {
  used: number;
  limit: number;
  remaining: number;
  resetsAt: string;
  unlimited: boolean;
}

/** 翻译请求：逐段（§7.4）。 */
export interface TranslateRequest {
  segments: { text: string }[];
  from: string;
  to: string;
  style?: string;
  model?: string;
}

/** 单段结果。status: ok|skipped|failed。 */
export interface SegmentResult {
  status: 'ok' | 'skipped' | 'failed';
  text?: string;
  code?: ErrorCode;
  message?: string;
  degraded?: boolean;
  model?: string;
  ms?: number;
  customFailed?: boolean;
}

export interface TranslateResponse {
  segments: SegmentResult[];
  degraded?: boolean;
  degradedReason?: string;
  model?: string;
}

/** 纠错建议（FR-203：四字段）。 */
export interface Suggestion {
  original: string;
  suggestion: string;
  category: 'spelling' | 'grammar' | 'punctuation' | 'style' | 'word_choice';
  reason: string;
}

export interface ProofreadResponse {
  suggestions: Suggestion[];
}

/** 术语（FR-501）。 */
export interface GlossaryItem {
  id: number;
  scope: 'site' | 'user';
  sourceTerm: string;
  targetTerm: string;
  note?: string;
  enabled: boolean;
}

export interface GlossaryView {
  items: GlossaryItem[];
  fingerprint: string;
}

/** 词表（FR-702/1202/1204）。 */
export interface Word {
  id: number;
  word: string;
  lang: string;
  contextSentence?: string;
  sourceUrl?: string;
  useCount: number;
  lastUsedAt?: string;
  nextDueAt?: string;
  createdAt: string;
}

/** 错误本（FR-1203）。 */
export interface Mistake {
  id: number;
  sourceSnippet: string;
  correctedSnippet: string;
  category: string;
  reason: string;
  nextDueAt?: string;
  createdAt: string;
}

/** 复习队列项（FR-1204：挖空自己的句子）。 */
export interface ReviewItem {
  kind: 'word' | 'mistake';
  id: number;
  prompt: string;
  cloze: string;
  answer: string;
  hint: string;
  category?: string;
  reason?: string;
}

/** 查词卡响应（§八 8.5）。 */
export interface DictEntry {
  word: string;
  query: string;
  phonetic?: string;
  pos?: string;
  senses: { pos: string; def: string }[];
  tags: string[];
  source: 'ecdict' | 'model';
  example?: string;
  typoHint?: { correctTo: string };
  collected: { yes: boolean; useCount: number };
}

/** 用户偏好 6 项（FR-1001）。 */
export interface UserPrefs {
  langPair: { from: string; to: string };
  style: string;
  model: string;
  theme: 'light' | 'dark';
  wordTrigger: 'direct' | 'icon' | 'modifier';
  autoSpeak: boolean;
}

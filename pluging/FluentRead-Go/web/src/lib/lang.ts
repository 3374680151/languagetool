/**
 * lang.ts 是前端的语言/风格清单（与后端 internal/service/lang.go 一致）。
 *
 * @file web/src/lib/lang.ts
 * 文件职责：语言下拉与风格下拉的选项；顺序与后端一致（FR-301/302）。
 * P2 起 /api/config 也会下发同一份，本地清单是"接口不可用时的兜底"。
 *
 * 主要内容：Languages、DefaultLangPair。
 *
 * 模块边界：纯数据；不发起请求。
 */

export interface Language {
  id: string;
  label: string;
}

/** Languages 顺序即前端下拉顺序（FR-301）。 */
export const Languages: Language[] = [
  { id: 'auto', label: '自动检测' },
  { id: 'zh-Hans', label: '简体中文' },
  { id: 'zh-Hant', label: '繁體中文' },
  { id: 'en', label: 'English' },
  { id: 'ja', label: '日本語' },
  { id: 'ko', label: '한국어' },
  { id: 'fr', label: 'Français' },
  { id: 'de', label: 'Deutsch' },
  { id: 'es', label: 'Español' },
  { id: 'ru', label: 'Русский' },
];

/** 默认语言对永远是英→中（FR-101/112）。 */
export const DefaultLangPair = { from: 'en', to: 'zh-Hans' } as const;

export const styleLabels: Record<string, string> = {
  academic: '学术',
  formal: '正式',
  concise: '简洁',
  casual: '口语',
};

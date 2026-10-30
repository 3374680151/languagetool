/**
 * markdown.ts 生成导出的 Markdown（FR-801/802，前端生成 Blob 下载）。
 *
 * @file web/src/lib/markdown.ts
 * 文件职责：三种导出模式——仅英文 / 仅中文 / 中英对照（英文段、中文段交替）；
 * 保留段落顺序与空行分隔（FR-802）；未翻译的段落标"（未翻译）"占位。
 *
 * 主要内容：buildMarkdown、exportFileName、downloadText。
 *
 * 模块边界：纯函数 + 一次 Blob 下载；不认识 store（段落由调用方整理传入）。
 */

export interface ExportSegment {
  source: string;
  target: string;
  /** hasTranslation 为 false 的段落用占位标注。 */
  hasTranslation: boolean;
}

export type ExportMode = 'en' | 'zh' | 'bilingual';

/**
 * buildMarkdown 按模式拼装导出文本。
 * 对照结构 = 每段"英文段、中文段"交替（FR-801 验收标准）。
 */
export function buildMarkdown(mode: ExportMode, segments: ExportSegment[]): string {
  const blocks: string[] = [];
  for (const seg of segments) {
    const src = seg.source.trim();
    const tgt = seg.hasTranslation ? seg.target.trim() : '（未翻译）';
    if (mode === 'en') {
      blocks.push(src);
    } else if (mode === 'zh') {
      blocks.push(tgt);
    } else {
      blocks.push(`${src}\n\n${tgt}`);
    }
  }
  // 段落之间空一行：导出的英文段保留原有段落顺序与空行（FR-802）。
  return blocks.join('\n\n') + '\n';
}

/** exportFileName 生成 fluentread-YYYYMMDD.md 文件名。 */
export function exportFileName(mode: ExportMode): string {
  const d = new Date();
  const ymd = `${d.getFullYear()}${String(d.getMonth() + 1).padStart(2, '0')}${String(d.getDate()).padStart(2, '0')}`;
  const suffix = mode === 'en' ? 'en' : mode === 'zh' ? 'zh' : 'bilingual';
  return `fluentread-${ymd}-${suffix}.md`;
}

/** downloadText 用 Blob 触发浏览器下载。 */
export function downloadText(fileName: string, content: string): void {
  const blob = new Blob([content], { type: 'text/markdown;charset=utf-8' });
  const url = URL.createObjectURL(blob);
  const a = document.createElement('a');
  a.href = url;
  a.download = fileName;
  document.body.appendChild(a);
  a.click();
  a.remove();
  URL.revokeObjectURL(url);
}

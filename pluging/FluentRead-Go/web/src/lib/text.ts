/**
 * text.ts 是文本处理纯函数集。
 *
 * @file web/src/lib/text.ts
 * 文件职责：段落切分（FR-104/105：空行分段、粘贴时空行不开空框、单换行同段）、
 * 字数统计（按 Unicode 码点，与后端口径一致）、空白判断。
 *
 * 主要内容：splitParagraphs、charCount、isBlank。
 *
 * 模块边界：纯函数，无状态；被 workbench store 与组件使用。
 */

/**
 * splitParagraphs 把整篇文本切成段落数组。
 *
 * 规则（FR-104/105）：
 * - 空行（含只含空白的行）是段落分隔符；
 * - 单个换行不算分段（同一段内的折行）；
 * - 连续多个空行只产生一次分隔（不产生空输入框）；
 * - 首尾空白段丢弃。
 */
export function splitParagraphs(text: string): string[] {
  if (isBlank(text)) return [];
  const raw = text.replace(/\r\n/g, '\n').split(/\n[ \t]*\n+/);
  const out: string[] = [];
  for (const part of raw) {
    const trimmed = part.replace(/^\n+|\n+$/g, '');
    if (!isBlank(trimmed)) {
      out.push(trimmed);
    }
  }
  return out;
}

/** charCount 按 Unicode 码点计数（与后端 utf8.RuneCountInString 口径一致）。 */
export function charCount(text: string): number {
  return [...text].length;
}

/** isBlank 判断"全是空白"。 */
export function isBlank(text: string): boolean {
  return text.trim().length === 0;
}

/** splitSentences 把一段文本按句末标点粗切，供"提取词所在句"使用（§八 8.5）。 */
export function sentenceAround(text: string, pos: number, maxLen = 300): string {
  if (text.length === 0) return '';
  const clamped = Math.min(Math.max(pos, 0), text.length - 1);
  let start = 0;
  for (let i = clamped; i >= 0; i--) {
    if ('.?!。？！'.includes(text[i])) {
      start = i + 1;
      break;
    }
  }
  let end = text.length;
  for (let i = clamped; i < text.length; i++) {
    if ('.?!。？！'.includes(text[i])) {
      end = i + 1;
      break;
    }
  }
  let sentence = text.slice(start, end).trim();
  if (sentence.length > maxLen) {
    sentence = sentence.slice(0, maxLen);
  }
  return sentence;
}

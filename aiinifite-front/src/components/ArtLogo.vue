<script setup lang="ts">
defineProps<{
  /** "aiinifite" 默认；可传 "aiinfinite" / "infiniteform" 等 */
  text?: string
  /** "lg" 巨大 hero 标题；"md" 顶部 nav 品牌位 */
  size?: 'lg' | 'md'
}>()
</script>

<template>
  <span class="art-logo" :class="size ?? 'md'">
    <span class="art-text" :data-text="text ?? 'aiinifite'">
      {{ text ?? 'aiinifite' }}
    </span>
  </span>
</template>

<style scoped>
/* ============================================================
   art-logo: 奶咖色系双色渐变 + 1px 深棕描边的艺术字
   - 衬出原图的奶咖卡片风格
   - 字母间距拉大、字重 900
   - 大小写由 .lg / .md 控制
   - data-text 用于 ::after 双层（实现"双层渐变"立体感）
   ============================================================ */
.art-logo {
  --grad-1: #34d399;
  --grad-2: #10b981;
  --grad-3: #065f46;
  --stroke: #047857;
  display: inline-block;
  font-family: var(--font-display);
  font-weight: 900;
  letter-spacing: 0.18em;
  text-transform: uppercase;
  line-height: 0.95;
  position: relative;
  white-space: nowrap;
}

/* md = nav 品牌位 (16-18px) */
.art-logo.md {
  font-size: clamp(14px, 1.4vw, 18px);
}
/* lg = hero 主标题 (80-180px) */
.art-logo.lg {
  font-size: clamp(80px, 12vw, 180px);
  letter-spacing: 0.08em;
}

/* 主文字层：双色斜向渐变 + 描边 */
.art-text {
  position: relative;
  display: inline-block;
  background: linear-gradient(135deg, var(--grad-1) 0%, var(--grad-2) 55%, var(--grad-3) 100%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  -webkit-text-stroke: 1px var(--stroke);
  filter: drop-shadow(0 2px 0 rgba(255, 255, 255, 0.4))
          drop-shadow(0 6px 12px rgba(74, 51, 24, 0.18));
  z-index: 1;
}

/* 第二层：偏移 1px 的"亮边"——制造立体感（只在 .lg 启用） */
.art-logo.lg .art-text::after {
  content: attr(data-text);
  position: absolute;
  inset: 0;
  background: linear-gradient(135deg, rgba(255, 255, 255, 0.6), rgba(255, 245, 212, 0.1) 60%, transparent 90%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  transform: translate(2px, -2px);
  z-index: -1;
  pointer-events: none;
}

/* hover 轻动效 */
.art-logo:hover .art-text {
  filter: drop-shadow(0 2px 0 rgba(255, 255, 255, 0.5))
          drop-shadow(0 10px 18px rgba(74, 51, 24, 0.25));
  transition: filter 0.3s ease;
}
</style>

<script setup lang="ts">
import { models } from '../data/models'

interface Item {
  kind: 'art' | 'chip' | 'tag'
  text: string
  sub?: string
}

const artWords = ['一把密钥。', '无限模型。', '零限制。', 'api.aiinfinite.online']

const phrases = ['按量计费', 'OpenAI 兼容', '随时切换', '无需订阅', '99.9% 可用', '人民币充值']

const priceChips: Item[] = models.map((m) => ({
  kind: 'chip',
  text: m.name,
  sub: `${m.priceIn} / ${m.priceOut}`,
}))

const tagItems: Item[] = phrases.map((t) => ({ kind: 'tag', text: t }))

interface Row {
  speed: number
  reverse?: boolean
  items: Item[]
}

const rows: Row[] = [
  { speed: 110, items: artWords.map((t) => ({ kind: 'art', text: t } as Item)) },
  { speed: 70, reverse: true, items: priceChips },
  { speed: 85, items: tagItems },
]

function repeated(row: Row, copies = 4): Item[] {
  return Array.from({ length: copies }, () => row.items).flat()
}
</script>

<template>
  <div class="bg-marquee" aria-hidden="true">
    <div class="grid-overlay"></div>
    <div class="glow glow-a"></div>
    <div class="glow glow-b"></div>

    <div class="stack">
      <div
        v-for="(row, ri) in rows"
        :key="ri"
        class="line"
        :class="{ reverse: row.reverse }"
        :style="{ '--speed': row.speed + 's' }"
      >
        <template v-for="(item, ii) in repeated(row)" :key="ii">
          <span v-if="item.kind === 'art'" class="seg art">{{ item.text }}</span>
          <span v-else-if="item.kind === 'chip'" class="chip">
            <code>{{ item.text }}</code>
            <i>·</i>
            <em>{{ item.sub }}</em>
          </span>
          <span v-else class="tag">{{ item.text }}</span>
        </template>
      </div>
    </div>

    <div class="fade fade-top"></div>
    <div class="fade fade-bottom"></div>

    <footer class="hud">
      <span>SYS.STATUS <b>NOMINAL</b></span>
      <span>GATEWAY <b>API.AIINFINITE.ONLINE</b></span>
      <span>MODELS <b>200+</b></span>
      <span class="live"><i></i>LIVE</span>
    </footer>
  </div>
</template>

<style scoped>
.bg-marquee {
  position: fixed;
  inset: 0;
  z-index: 0;
  overflow: hidden;
  pointer-events: none;
}

.grid-overlay {
  position: absolute;
  inset: 0;
  background-image:
    linear-gradient(rgba(4, 120, 87, 0.04) 1px, transparent 1px),
    linear-gradient(90deg, rgba(4, 120, 87, 0.04) 1px, transparent 1px);
  background-size: 72px 72px;
}

.glow {
  position: absolute;
  width: 52vw;
  height: 52vw;
  border-radius: 50%;
  opacity: 0.55;
}
.glow-a {
  top: -20%;
  left: -12%;
  background: radial-gradient(circle, rgba(52, 211, 153, 0.2) 0%, rgba(52, 211, 153, 0.07) 35%, transparent 68%);
}
.glow-b {
  bottom: -24%;
  right: -14%;
  background: radial-gradient(circle, rgba(16, 185, 129, 0.14) 0%, rgba(16, 185, 129, 0.05) 35%, transparent 68%);
}

.stack {
  position: absolute;
  inset: -4% 0;
  display: flex;
  flex-direction: column;
  justify-content: space-evenly;
}

.line {
  display: flex;
  align-items: center;
  gap: 3vw;
  width: max-content;
  white-space: nowrap;
  will-change: transform;
  backface-visibility: hidden;
  animation: slide var(--speed) linear infinite;
}
.line.reverse {
  animation-direction: reverse;
}
@keyframes slide {
  from {
    transform: translateX(0);
  }
  to {
    transform: translateX(-50%);
  }
}

/* 大字水印：淡填充 + 清晰描边，隐约但可读 */
.seg.art {
  font: 900 clamp(40px, 5.5vw, 80px) var(--font-serif-cn);
  letter-spacing: 0.08em;
  color: rgba(16, 185, 129, 0.1);
  -webkit-text-stroke: 1.5px rgba(4, 120, 87, 0.3);
}

/* 价格胶囊：小而精致的白色药丸 */
.chip {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  padding: 10px 20px;
  border-radius: 100px;
  background: rgba(255, 255, 255, 0.72);
  border: 1px solid rgba(16, 185, 129, 0.38);
  box-shadow: 0 2px 12px rgba(4, 120, 87, 0.08);
}
.chip code {
  font: 600 14px var(--font-mono);
  color: #047857;
}
.chip i {
  font-style: normal;
  color: rgba(16, 185, 129, 0.5);
}
.chip em {
  font: 400 13px var(--font-mono);
  font-style: normal;
  color: rgba(6, 95, 70, 0.7);
}

/* 短语：极简细边框 */
.tag {
  display: inline-flex;
  align-items: center;
  padding: 9px 18px;
  border-radius: 100px;
  border: 1px solid rgba(4, 120, 87, 0.3);
  font: 500 13px var(--font-mono);
  letter-spacing: 0.12em;
  color: rgba(6, 95, 70, 0.68);
}

/* 顶底渐隐，让跑马灯融入页面而不是被截断 */
.fade {
  position: absolute;
  left: 0;
  right: 0;
  height: 18vh;
  pointer-events: none;
}
.fade-top {
  top: 0;
  background: linear-gradient(to bottom, #eafaf3, transparent);
}
.fade-bottom {
  bottom: 0;
  background: linear-gradient(to top, #eafaf3, transparent);
}

.hud {
  position: absolute;
  left: clamp(20px, 5vw, 76px);
  right: clamp(20px, 5vw, 76px);
  bottom: 14px;
  display: flex;
  gap: clamp(14px, 3vw, 42px);
  flex-wrap: wrap;
  font: 500 10px var(--font-mono);
  letter-spacing: 0.14em;
  text-transform: uppercase;
  color: rgba(6, 95, 70, 0.45);
}
.hud b {
  color: var(--brand-deep);
  font-weight: 700;
}
.hud .live {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  color: #047857;
}
.hud .live i {
  width: 7px;
  height: 7px;
  border-radius: 50%;
  background: #10b981;
  box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.25);
  animation: blink 1.6s ease-in-out infinite;
}
@keyframes blink {
  50% {
    box-shadow: 0 0 0 7px rgba(16, 185, 129, 0);
  }
}

@media (prefers-reduced-motion: reduce) {
  .line,
  .hud .live i {
    animation: none;
  }
}
</style>

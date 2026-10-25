<script setup lang="ts">
import InteractiveCat from './InteractiveCat.vue'

const features = [
  {
    title: '极简接入，零成本迁移',
    desc: '兼容 OpenAI / Anthropic / Gemini 协议，只需替换 Base URL，现有代码即可无缝调用所有模型。',
    icon: 'plug',
  },
  {
    title: '超值性价比，成本直降',
    desc: '批量采购与企业折扣，把利润让利给开发者，部分模型价格低至官方的一折。',
    icon: 'bolt',
  },
  {
    title: '企业级稳定与安全',
    desc: '99.9% 可用性保障、多通道容灾自动切换、端到端加密。不虚标倍率、不偷偷缩水、不超售。',
    icon: 'shield',
  },
  {
    title: '统一管控，透明计费',
    desc: '一个后台管理所有模型的调用记录与费用，支持人民币充值与增值税发票，用量明细实时可查。',
    icon: 'chart',
  },
]
</script>

<template>
  <section class="card">
    <div class="copy">
      <span class="eyebrow"><i></i>api.aiinfinite.online</span>

      <h1>
        <span>一个 Base URL，</span>
        <span>连接所有 AI。</span>
      </h1>

      <p class="sub">告别多模型适配地狱，一行代码切换全球大模型。</p>

      <span class="stats-pill">
        <i></i>
        实时已为 <b>2,847</b> 位开发者服务超 <b>96,341</b> 次
      </span>

      <div class="features">
        <div v-for="(f, i) in features" :key="f.title" class="feature" :style="{ '--d': i * 0.12 + 's' }">
          <span class="icon">
            <svg v-if="f.icon === 'plug'" viewBox="0 0 24 24"><path d="M7 7V4a1 1 0 0 1 2 0v3h6V4a1 1 0 0 1 2 0v3h1a1 1 0 0 1 1 1v3a6 6 0 0 1-5 5.91V21a1 1 0 0 1-2 0v-4.09A6 6 0 0 1 7 11V8H6a1 1 0 0 1 0-2h1z"/></svg>
            <svg v-else-if="f.icon === 'bolt'" viewBox="0 0 24 24"><path d="M13 2 4 14h6l-1 8 9-12h-6l1-8z"/></svg>
            <svg v-else-if="f.icon === 'shield'" viewBox="0 0 24 24"><path d="M12 2 4 5v6c0 5.25 3.4 9.74 8 11 4.6-1.26 8-5.75 8-11V5l-8-3zm-1.2 13.5-3.3-3.3 1.4-1.4 1.9 1.9 4.3-4.3 1.4 1.4-5.7 5.7z"/></svg>
            <svg v-else viewBox="0 0 24 24"><path d="M5 21V9h4v12H5zm7 0V3h4v18h-4zm7 0v-8h4v8h-4z" transform="translate(-1 0)"/></svg>
          </span>
          <div class="feature-copy">
            <h3>{{ f.title }}</h3>
            <p>{{ f.desc }}</p>
          </div>
        </div>
      </div>

      <span class="endpoint">
        <span class="tag">POST</span>
        <code>/v1/chat/completions</code>
      </span>
    </div>

    <div class="stage">
      <InteractiveCat />
    </div>
  </section>
</template>

<style scoped>
.card {
  position: relative;
  max-width: 1200px;
  min-height: 0;
  margin: auto;
  width: 100%;
  border-radius: 30px;
  background: linear-gradient(115deg, rgba(255, 255, 255, 0.72), rgba(255, 255, 255, 0.42));
  backdrop-filter: blur(8px);
  -webkit-backdrop-filter: blur(8px);
  border: 1px solid rgba(16, 185, 129, 0.22);
  overflow: hidden;
  display: grid;
  grid-template-columns: 1.1fr 0.9fr;
  align-items: center;
  padding: clamp(26px, 4vw, 52px);
  box-shadow: var(--card-shadow);
}
.copy {
  position: relative;
  z-index: 2;
  max-width: 640px;
  color: var(--text-accent);
}
.eyebrow {
  display: inline-flex;
  align-items: center;
  gap: 14px;
  font: italic 400 clamp(18px, 1.8vw, 25px) var(--font-serif);
  letter-spacing: 0.01em;
  color: var(--brand-deep);
  margin-bottom: 18px;
}
.eyebrow i {
  width: clamp(11px, 1vw, 14px);
  height: clamp(11px, 1vw, 14px);
  background: var(--brand-1);
  transform: rotate(45deg);
}
h1 {
  font: 900 clamp(32px, 4.6vw, 62px) var(--font-serif-cn);
  line-height: 1.22;
  margin: 0 0 14px;
  letter-spacing: 0.02em;
  background: linear-gradient(92deg, #065f46 10%, #10b981 55%, #34d399 90%);
  -webkit-background-clip: text;
  background-clip: text;
  -webkit-text-fill-color: transparent;
  filter: drop-shadow(0 4px 24px rgba(16, 185, 129, 0.25));
}
h1 span {
  display: block;
  white-space: nowrap;
}
.sub {
  font-size: clamp(14px, 1.3vw, 17px);
  line-height: 1.7;
  margin: 0 0 16px;
  color: rgba(6, 95, 70, 0.75);
}
.stats-pill {
  display: inline-flex;
  align-items: center;
  gap: 10px;
  font: 600 12.5px var(--font-mono);
  color: #047857;
  background: rgba(209, 250, 229, 0.7);
  border: 1px solid rgba(16, 185, 129, 0.45);
  padding: 9px 15px;
  border-radius: 100px;
  margin-bottom: 20px;
  box-shadow: inset 0 0 24px rgba(16, 185, 129, 0.08);
}
.stats-pill i {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: #10b981;
  box-shadow: 0 0 0 3px rgba(16, 185, 129, 0.3);
  animation: live-blink 1.8s ease-in-out infinite;
}
@keyframes live-blink {
  50% {
    box-shadow: 0 0 0 7px rgba(16, 185, 129, 0);
  }
}
.stats-pill b {
  font-weight: 800;
  color: #065f46;
}

.features {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  margin-bottom: 20px;
}
.feature {
  display: flex;
  gap: 12px;
  align-items: flex-start;
  padding: 13px 14px;
  border-radius: 14px;
  background: rgba(255, 255, 255, 0.55);
  border: 1px solid rgba(16, 185, 129, 0.2);
  opacity: 0;
  transform: translateY(14px);
  animation: rise 0.7s cubic-bezier(0.22, 1, 0.36, 1) forwards;
  animation-delay: var(--d);
  transition:
    background 0.25s ease,
    transform 0.25s ease,
    border-color 0.25s ease;
}
.feature:hover {
  background: rgba(255, 255, 255, 0.85);
  border-color: rgba(16, 185, 129, 0.45);
  transform: translateY(-3px);
}
@keyframes rise {
  to {
    opacity: 1;
    transform: translateY(0);
  }
}
.icon {
  flex: none;
  width: 34px;
  height: 34px;
  border-radius: 10px;
  display: grid;
  place-items: center;
  background: linear-gradient(135deg, rgba(16, 185, 129, 0.18), rgba(52, 211, 153, 0.28));
  border: 1px solid rgba(16, 185, 129, 0.3);
}
.icon svg {
  width: 17px;
  height: 17px;
  fill: #047857;
}
.feature-copy h3 {
  margin: 0 0 4px;
  font: 700 13.5px var(--font-display);
  letter-spacing: 0.01em;
  color: #065f46;
}
.feature-copy p {
  margin: 0;
  font-size: 11.5px;
  line-height: 1.6;
  color: rgba(6, 95, 70, 0.65);
}
.endpoint {
  display: inline-flex;
  gap: 15px;
  align-items: center;
  background: #fff;
  color: var(--text-accent);
  border-radius: 10px;
  padding: 12px 16px;
  font: 500 13px var(--font-mono);
  border: 1px solid rgba(16, 185, 129, 0.25);
  box-shadow: 0 6px 20px rgba(4, 120, 87, 0.12);
}
.endpoint .tag {
  font: bold 10px var(--font-mono);
  background: var(--brand-1);
  color: #fff;
  padding: 7px 9px;
  border-radius: 5px;
}
.stage {
  position: relative;
  display: flex;
  justify-content: center;
  align-items: center;
  height: 100%;
  min-height: 400px;
  z-index: 1;
}
@media (max-width: 960px) {
  .card {
    grid-template-columns: 1fr;
    gap: 16px;
    padding: clamp(24px, 6vw, 44px);
  }
  .stage {
    min-height: 300px;
  }
  .copy {
    max-width: none;
  }
}
@media (max-width: 720px) {
  .features {
    grid-template-columns: 1fr;
  }
}
@media (max-width: 480px) {
  h1 {
    font-size: clamp(26px, 7.6vw, 40px);
  }
  h1 span {
    white-space: normal;
  }
  .eyebrow {
    font-size: 16px;
  }
  .stats-pill {
    font-size: 11px;
    padding: 9px 13px;
  }
  .endpoint {
    flex-wrap: wrap;
    gap: 10px;
  }
}
@media (prefers-reduced-motion: reduce) {
  .feature {
    animation: none;
    opacity: 1;
    transform: none;
  }
}
</style>
